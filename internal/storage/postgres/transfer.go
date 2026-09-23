package postgres

import (
	"context"
	"fmt"

	"github.com/1SenaMetth/ledger/internal/domain"
	"github.com/1SenaMetth/ledger/internal/storage/postgres/internal/sqlcgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateTransfer(ctx context.Context, from, to uuid.UUID, amount domain.Money, description string, idempotencyKey string, pool *pgxpool.Pool) (domain.Transaction, error) {
	if from == to {
		return domain.Transaction{}, domain.ErrSameAccount
	}
	var resultTx domain.Transaction

	err := WithTx(ctx, pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)

		rows, err := q.LockAccountsForUpdate(ctx, []uuid.UUID{from, to})

		if err != nil {
			return err
		}

		if len(rows) != 2 {
			return domain.ErrAccountNotFound
		}

		var fromRow, toRow sqlcgen.Account

		if rows[0].ID == from {
			fromRow = rows[0]
			toRow = rows[1]
		} else {
			fromRow = rows[1]
			toRow = rows[0]

		}

		fromAcc, err := toDomainAccount(fromRow)
		if err != nil {
			return err
		}
		toAcc, err := toDomainAccount(toRow)
		if err != nil {
			return err
		}

		newTx, err := domain.NewTransfer(fromAcc, toAcc, amount, description, idempotencyKey)
		if err != nil {
			return err
		}

		for _, entry := range newTx.Entries {
			_, err = q.UpdateAccountBalance(ctx, sqlcgen.UpdateAccountBalanceParams{
				ID:          entry.AccountID,
				AmountMinor: entry.Amount.Minor(),
			})
			if err != nil {
				return err
			}
		}
		var idempKeyPtr *string
		if newTx.IdempotencyKey != "" {
			idempKeyPtr = &newTx.IdempotencyKey
		}

		txRow, err := q.CreateTransaction(ctx, sqlcgen.CreateTransactionParams{
			ID:             newTx.ID,
			IdempotencyKey: idempKeyPtr,
			Description:    newTx.Description,
		})

		if err != nil {
			return err
		}

		if !txRow.CreatedAt.Valid {
			return fmt.Errorf("database returned invalid timestamp for transaction %s", newTx.ID)
		}

		newTx.CreatedAt = txRow.CreatedAt.Time

		for i := range newTx.Entries {
			entry := &newTx.Entries[i]
			entryRow, err := q.CreateEntry(ctx, sqlcgen.CreateEntryParams{
				ID:            entry.ID,
				TransactionID: entry.TransactionID,
				AccountID:     entry.AccountID,
				AmountMinor:   entry.Amount.Minor(),
				Currency:      string(entry.Amount.Currency()),
			})
			if err != nil {
				return err
			}
			if !entryRow.CreatedAt.Valid {
				return fmt.Errorf("database returned invalid timestamp for entry %s", entry.ID)
			}
			entry.CreatedAt = entryRow.CreatedAt.Time
		}
		resultTx = newTx
		return nil
	})

	if err != nil {
		return domain.Transaction{}, err
	}
	return resultTx, nil
}

func toDomainAccount(row sqlcgen.Account) (domain.Account, error) {
	var ownerID *uuid.UUID
	if row.OwnerID.Valid {
		id := row.OwnerID.UUID
		ownerID = &id
	}

	if !row.CreatedAt.Valid || !row.UpdatedAt.Valid {
		return domain.Account{}, fmt.Errorf("account %s has invalid timestamps", row.ID)
	}

	return domain.ParseAccount(
		row.ID,
		ownerID,
		row.Type,
		row.BalanceMinor,
		row.Currency,
		row.Version,
		row.CreatedAt.Time,
		row.UpdatedAt.Time,
	)
}
