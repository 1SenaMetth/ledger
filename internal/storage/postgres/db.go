// Package postgres owns the database connection and transaction handling.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config mirrors the database settings from internal/config, kept separate so
// this package does not import the config package.
type Config struct {
	URL      string
	MaxConns int32
	MinConns int32
}

// NewPool opens a connection pool and verifies it can reach the database.
func NewPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = time.Hour
	poolCfg.MaxConnIdleTime = 30 * time.Minute
	poolCfg.HealthCheckPeriod = time.Minute

	// Without jitter every connection opened at startup also expires at the same
	// moment, and the pool empties in one go under steady load.
	poolCfg.MaxConnLifetimeJitter = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

// WithTx runs fn inside a database transaction, committing when fn returns nil
// and rolling back otherwise. The deferred rollback is a safety net: it runs
// after a successful commit too, where pgx returns ErrTxClosed, which is why
// the error is deliberately ignored there.
//
// Every multi-step write in this service goes through this helper. Reading the
// balance and writing the entries in separate transactions is exactly the bug
// milestone 1.8 is designed to expose.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// TODO(phase1, milestone 1.8): add a helper that locks two accounts in a
// deterministic order.
//
// Two concurrent transfers, A->B and B->A, will deadlock if each locks its own
// source account first. Postgres detects the deadlock and kills one of them,
// so the symptom is intermittent "deadlock detected" errors under load rather
// than a hang. The fix is to always take locks in the same order, for example
// by sorting the two account IDs and locking the lower one first:
//
//	SELECT id, balance_minor, currency, type, version
//	FROM accounts
//	WHERE id = ANY($1)
//	ORDER BY id
//	FOR UPDATE;
//
// Write the failing concurrent test first, watch it deadlock, then fix it.
// That sequence is worth more than the fix itself.
