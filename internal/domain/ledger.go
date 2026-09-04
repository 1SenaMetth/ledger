package domain

import (
	"time"

	"github.com/google/uuid"
)

// AccountType determines which invariants apply to an account.
type AccountType string

const (
	// AccountUserWallet belongs to a user and may never hold a negative balance.
	AccountUserWallet AccountType = "user_wallet"

	// AccountSystem represents the boundary with the outside world (external
	// funding, fees, pending withdrawals). System accounts are allowed to go
	// negative: a deposit of R$100 credits a wallet and debits the external
	// funding account, which is how the books stay balanced.
	AccountSystem AccountType = "system"
)

// Account is a single balance in the ledger.
type Account struct {
	ID        uuid.UUID
	OwnerID   *uuid.UUID // nil for system accounts
	Type      AccountType
	Balance   Money
	Version   int64 // optimistic locking counter
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AllowsNegativeBalance reports whether this account may hold a debt.
func (a Account) AllowsNegativeBalance() bool {
	return a.Type == AccountSystem
}

// Entry is one leg of a transaction: a signed amount applied to one account.
// Entries are append-only. A mistake is corrected with a new reversing
// transaction, never by updating or deleting an entry.
type Entry struct {
	ID            uuid.UUID
	TransactionID uuid.UUID
	AccountID     uuid.UUID
	Amount        Money // negative = debit, positive = credit
	CreatedAt     time.Time
}

// Transaction is a set of entries that must sum to exactly zero.
type Transaction struct {
	ID             uuid.UUID
	IdempotencyKey string
	Description    string
	Entries        []Entry
	CreatedAt      time.Time
}

// Validate enforces the two invariants every transaction must satisfy:
// at least two entries, and a net movement of exactly zero per currency.
//
// TODO(phase1): implement this. The tests in ledger_test.go describe the
// expected behaviour; remove the t.Skip calls and make them pass.
//
// Hints:
//   - Group entries by currency and sum each group independently. A transaction
//     mixing BRL and USD is not automatically invalid, but each currency must
//     net to zero on its own.
//   - Use Money.Add so overflow is detected rather than silently wrapping.
//   - Return ErrUnbalancedTransaction when a currency group does not sum to zero.
func (t Transaction) Validate() error {
	if len(t.Entries) < 2 { //invariant1: at least two Entries
		return ErrInsufficientEntries
	}

	//track balance by currency code -> BRL, USD, EUR ...
	currencySums := make(map[Currency]Money)

	for _, entry := range t.Entries {
		currencyCode := entry.Amount.Currency()

		currentSum, exists := currencySums[currencyCode]
		if !exists {
			var err error
			currentSum, err = Zero(currencyCode)
			if err != nil {
				return err
			}
		}

		//invariant2
		newSum, err := currentSum.Add(entry.Amount)
		if err != nil {
			return err
		}
		currencySums[currencyCode] = newSum
	}

	for _, sum := range currencySums {
		if !sum.IsZero() {
			return ErrUnbalancedTransaction
		}
	}
	return nil
}

// NewTransfer builds the entries for moving amount from one account to another.
//
// TODO(phase1): implement this.
//
// Rules:
//   - amount must be strictly positive (ErrNonPositiveAmount)
//   - from and to must differ (ErrSameAccount)
//   - currencies of both accounts and the amount must match (ErrCurrencyMismatch)
//   - if the source cannot go negative and lacks funds, return ErrInsufficientFunds
//   - produce exactly two entries: -amount on from, +amount on to
//
// Note that this function checks the balance it is *given*. Preventing a race
// between reading the balance and writing the entries is the storage layer's
// job, and it is milestone 1.8 in the roadmap. Do not try to solve it here.
func NewTransfer(from, to Account, amount Money, description, idempotencyKey string) (Transaction, error) {
	if !amount.IsPositive() {
		return Transaction{}, ErrNonPositiveAmount
	}

	if from.ID == to.ID {
		return Transaction{}, ErrSameAccount
	}

	curr := amount.Currency()
	if from.Balance.Currency() != curr || to.Balance.Currency() != curr {
		return Transaction{}, ErrCurrencyMismatch
	}
	if !from.AllowsNegativeBalance() {
		cmp, err := from.Balance.Compare(amount)
		if err != nil {
			return Transaction{}, err
		}
		if cmp < 0 {
			return Transaction{}, ErrInsufficientFunds
		}
	}

	debitAmount, err := amount.Neg()
	if err != nil {
		return Transaction{}, err
	}

	txID := uuid.New()

	tx := Transaction{
		ID:             txID,
		IdempotencyKey: idempotencyKey,
		Description:    description,
		Entries: []Entry{
			{
				ID:            uuid.New(),
				TransactionID: txID,
				AccountID:     from.ID,
				Amount:        debitAmount,
			},
			{
				ID:            uuid.New(),
				TransactionID: txID,
				AccountID:     to.ID,
				Amount:        amount,
			},
		},
	}
	if err := tx.Validate(); err != nil {
		return Transaction{}, err
	}

	return tx, nil

}
