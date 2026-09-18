package domain

import (
	"errors"
	"github.com/google/uuid"
	"testing"
	"time"
)

func wallet(balance int64, c Currency) Account {
	owner := uuid.New()
	return Account{
		ID:      uuid.New(),
		OwnerID: &owner,
		Type:    AccountUserWallet,
		Balance: MustMoney(balance, c),
	}
}

func systemAccount(balance int64, c Currency) Account {
	return Account{
		ID:      uuid.New(),
		Type:    AccountSystem,
		Balance: MustMoney(balance, c),
	}
}

func TestNewTransfer(t *testing.T) {

	from := wallet(10_000, BRL) // R$100.00
	to := wallet(0, BRL)

	tx, err := NewTransfer(from, to, MustMoney(2_500, BRL), "rent split", "key-1")
	if err != nil {
		t.Fatalf("NewTransfer() unexpected error: %v", err)
	}
	if len(tx.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(tx.Entries))
	}
	if err := tx.Validate(); err != nil {
		t.Errorf("a fresh transfer must be valid, got: %v", err)
	}

	if tx.ID == uuid.Nil {
		t.Errorf("expected transaction ID to be generated, got nil")
	}

	var debit, credit int64
	seenIDs := make(map[uuid.UUID]bool)

	for _, e := range tx.Entries {
		if e.ID == uuid.Nil {
			t.Errorf("expected entry ID to be generated for accounts %s, got Nil", e.AccountID)
		}

		if seenIDs[e.ID] {
			t.Errorf("entries must have distinct IDs, found duplicate: %s", e.ID)
		}
		seenIDs[e.ID] = true

		if e.TransactionID != tx.ID {
			t.Errorf("entry TransactionID = %s, want parent tx.ID %s", e.TransactionID, tx.ID)

		}

		switch e.AccountID {
		case from.ID:
			debit = e.Amount.Minor()
		case to.ID:
			credit = e.Amount.Minor()
		default:
			t.Errorf("entry references unknown account %s", e.AccountID)
		}
	}
	if debit != -2_500 {
		t.Errorf("debit = %d, want -2500", debit)
	}
	if credit != 2_500 {
		t.Errorf("credit = %d, want 2500", credit)
	}
}

func TestNewTransferRejections(t *testing.T) {

	from := wallet(10_000, BRL)
	to := wallet(0, BRL)
	usdWallet := wallet(0, USD)

	tests := []struct {
		name     string
		from, to Account
		amount   Money
		wantErr  error
	}{
		{"zero amount", from, to, MustMoney(0, BRL), ErrNonPositiveAmount},
		{"negative amount", from, to, MustMoney(-1, BRL), ErrNonPositiveAmount},
		{"same account", from, from, MustMoney(100, BRL), ErrSameAccount},
		{"currency mismatch", from, usdWallet, MustMoney(100, BRL), ErrCurrencyMismatch},
		{"insufficient funds", wallet(50, BRL), to, MustMoney(100, BRL), ErrInsufficientFunds},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewTransfer(tt.from, tt.to, tt.amount, "d", "k"); !errors.Is(err, tt.wantErr) {
				t.Errorf("NewTransfer() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// A system account funding a deposit is allowed to go negative.
func TestSystemAccountMayGoNegative(t *testing.T) {

	external := systemAccount(0, BRL)
	userWallet := wallet(0, BRL)

	if _, err := NewTransfer(external, userWallet, MustMoney(50_000, BRL), "deposit", "k"); err != nil {
		t.Errorf("system account should be allowed to go negative, got: %v", err)
	}
}

func TestTransactionValidate(t *testing.T) {
	a, b := uuid.New(), uuid.New()

	balanced := Transaction{Entries: []Entry{
		{AccountID: a, Amount: MustMoney(-100, BRL)},
		{AccountID: b, Amount: MustMoney(100, BRL)},
	}}
	if err := balanced.Validate(); err != nil {
		t.Errorf("balanced transaction rejected: %v", err)
	}

	unbalanced := Transaction{Entries: []Entry{
		{AccountID: a, Amount: MustMoney(-100, BRL)},
		{AccountID: b, Amount: MustMoney(99, BRL)},
	}}
	if err := unbalanced.Validate(); !errors.Is(err, ErrUnbalancedTransaction) {
		t.Errorf("unbalanced transaction error = %v, want ErrUnbalancedTransaction", err)
	}

	// Each currency must net to zero independently.
	mixed := Transaction{Entries: []Entry{
		{AccountID: a, Amount: MustMoney(-100, BRL)},
		{AccountID: b, Amount: MustMoney(100, USD)},
	}}
	if err := mixed.Validate(); !errors.Is(err, ErrUnbalancedTransaction) {
		t.Errorf("mixed-currency transaction error = %v, want ErrUnbalancedTransaction", err)
	}
}

func TestParseAccount(t *testing.T) {
	id := uuid.New()
	ownerID := uuid.New()
	now := time.Now().UTC()

	tests := []struct {
		name         string
		rawType      string
		balanceMinor int64
		currency     string
		wantErr      error
	}{

		{"Valid user wallet", string(AccountUserWallet), 1500, "BRL", nil},
		{"valid system account", string(AccountSystem), -500, "USD", nil},
		{"invalid type rejects", "checking", 1500, "BRL", ErrInvalidAccountType},
		{"invalid currency rejects", string(AccountUserWallet), 1500, "DODGECOINGAGA", ErrInvalidCurrency},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ownerPtr *uuid.UUID
			if tt.rawType == string(AccountUserWallet) {
				ownerPtr = &ownerID
			}

			got, err := ParseAccount(
				id,
				ownerPtr,
				tt.rawType,
				tt.balanceMinor,
				tt.currency,
				1,
				now,
				now,
			)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ParseAccount() error = %v, want %v", err, tt.wantErr)
			}

			if tt.wantErr == nil {
				if got.ID != id {
					t.Errorf("got ID %v, want %v", got.ID, id)
				}
				if got.OwnerID != ownerPtr {
					t.Errorf("got OwnerID %v, want %v", got.OwnerID, ownerPtr)
				}
				if string(got.Type) != tt.rawType {
					t.Errorf("got Type %v, want %v", got.Type, tt.rawType)
				}
				if got.Balance.Minor() != tt.balanceMinor {
					t.Errorf("got balance minor %d, want %d", got.Balance.Minor(), tt.balanceMinor)
				}
				if string(got.Balance.Currency()) != tt.currency {
					t.Errorf("got Currency %v, want %v", got.Balance.Currency(), tt.currency)
				}
				if got.Version != 1 {
					t.Errorf("got Version %d, want 1", got.Version)
				}
				if !got.CreatedAt.Equal(now) {
					t.Errorf("got CreatedAt %v, want %v", got.CreatedAt, now)
				}
				if !got.UpdatedAt.Equal(now) {
					t.Errorf("got UpdatedAt %v, want %v", got.UpdatedAt, now)
				}
			}
		})
	}
}
