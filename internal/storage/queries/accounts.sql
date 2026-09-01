-- Queries in this directory are compiled into type-safe Go by sqlc.
-- Run: make sqlc
--
-- TODO(phase1): write the queries you need. Two are sketched below to show the
-- annotation format; the rest are yours.

-- name: GetAccountByID :one
SELECT id, owner_id, type, currency, balance_minor, version, created_at, updated_at
FROM accounts
WHERE id = $1;

-- name: LockAccountsForUpdate :many
-- Deterministic ordering prevents deadlocks between concurrent opposing
-- transfers. See the note in internal/storage/postgres/db.go.
SELECT id, owner_id, type, currency, balance_minor, version, created_at, updated_at
FROM accounts
WHERE id = ANY($1::uuid[])
ORDER BY id
FOR UPDATE;
