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
WHERE id = ANY(sqlc.arg('account_ids')::uuid[])
ORDER BY id
FOR UPDATE;



-- name: UpdateAccountBalance :one
-- relative update prevents lost updates; the lock gives the funds check a current balance and, taken in ID order, prevents deadlocks. 
UPDATE accounts
SET
    balance_minor = balance_minor + sqlc.arg('amount_minor'),
    version = version + 1,
    updated_at = NOW()
WHERE id = sqlc.arg('id')
RETURNING id, owner_id, type, currency, balance_minor, version, created_at, updated_at;

-- name: CreateTransaction :one
INSERT INTO transactions(
    id,
    idempotency_key, 
    description 
) VALUES (
    $1, $2, $3
)
RETURNING id, idempotency_key, description, created_at;

-- name: CreateEntry :one
INSERT INTO entries(
id,
transaction_id,  
account_id,  
amount_minor,
currency
)VALUES(
$1, $2, $3, $4, $5
)
RETURNING id, transaction_id, account_id, amount_minor, currency, created_at;
