-- Initial ledger schema.
--
-- Design notes worth writing an ADR about (docs/adr/):
--   * Balances are materialised on accounts.balance_minor and updated in the
--     same transaction as the entries. The alternative is computing
--     SUM(entries.amount_minor) on every read, which is always correct but gets
--     slower forever. The materialised column needs a reconciliation job; see
--     the view at the bottom of this file.
--   * Money is BIGINT in minor units. NUMERIC would also be exact but is
--     slower and invites accidental fractional cents.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ---------------------------------------------------------------------------
-- users
-- ---------------------------------------------------------------------------
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        NOT NULL,
    password_hash TEXT        NOT NULL,
    role          TEXT        NOT NULL DEFAULT 'user',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT users_role_valid CHECK (role IN ('user', 'admin'))
);

-- Case-insensitive uniqueness without the citext extension.
CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email));

-- ---------------------------------------------------------------------------
-- accounts
-- ---------------------------------------------------------------------------
CREATE TABLE accounts (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id      UUID        REFERENCES users (id) ON DELETE RESTRICT,
    type          TEXT        NOT NULL,
    currency      CHAR(3)     NOT NULL,
    balance_minor BIGINT      NOT NULL DEFAULT 0,
    version       BIGINT      NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT accounts_type_valid CHECK (type IN ('user_wallet', 'system')),

    -- A user wallet must belong to someone; a system account must not.
    CONSTRAINT accounts_ownership_valid CHECK (
        (type = 'user_wallet' AND owner_id IS NOT NULL) OR
        (type = 'system'      AND owner_id IS NULL)
    ),

    -- The invariant that matters most, enforced by the database rather than by
    -- application code. Even a bug in Go cannot drive a wallet negative.
    CONSTRAINT accounts_wallet_never_negative CHECK (
        type <> 'user_wallet' OR balance_minor >= 0
    ),

    CONSTRAINT accounts_currency_valid CHECK (currency ~ '^[A-Z]{3}$')
);

CREATE INDEX accounts_owner_id_idx ON accounts (owner_id) WHERE owner_id IS NOT NULL;

-- One wallet per user per currency.
CREATE UNIQUE INDEX accounts_owner_currency_key
    ON accounts (owner_id, currency)
    WHERE type = 'user_wallet';

-- ---------------------------------------------------------------------------
-- transactions
-- ---------------------------------------------------------------------------
CREATE TABLE transactions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key TEXT,
    description     TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX transactions_idempotency_key_key
    ON transactions (idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- ---------------------------------------------------------------------------
-- entries (append-only)
-- ---------------------------------------------------------------------------
CREATE TABLE entries (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID        NOT NULL REFERENCES transactions (id) ON DELETE RESTRICT,
    account_id     UUID        NOT NULL REFERENCES accounts (id)     ON DELETE RESTRICT,
    amount_minor   BIGINT      NOT NULL,
    currency       CHAR(3)     NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT entries_amount_nonzero CHECK (amount_minor <> 0)
);

CREATE INDEX entries_transaction_id_idx ON entries (transaction_id);
CREATE INDEX entries_account_id_created_at_idx ON entries (account_id, created_at DESC, id DESC);

-- Entries are immutable. Corrections are new reversing transactions.
CREATE RULE entries_no_update AS ON UPDATE TO entries DO INSTEAD NOTHING;
CREATE RULE entries_no_delete AS ON DELETE TO entries DO INSTEAD NOTHING;

-- ---------------------------------------------------------------------------
-- The zero-sum invariant
-- ---------------------------------------------------------------------------
-- A deferred constraint trigger, so it fires once at COMMIT rather than after
-- each INSERT. Without DEFERRABLE INITIALLY DEFERRED you could never insert the
-- first leg of a transfer, because a single entry never sums to zero.
CREATE OR REPLACE FUNCTION assert_transaction_balances() RETURNS TRIGGER AS $$
DECLARE
    offending RECORD;
BEGIN
    FOR offending IN
        SELECT currency, SUM(amount_minor) AS total
        FROM entries
        WHERE transaction_id = NEW.transaction_id
        GROUP BY currency
        HAVING SUM(amount_minor) <> 0
    LOOP
        RAISE EXCEPTION
            'transaction % does not balance in %: net movement is %',
            NEW.transaction_id, offending.currency, offending.total
            USING ERRCODE = 'check_violation';
    END LOOP;

    IF (SELECT count(*) FROM entries WHERE transaction_id = NEW.transaction_id) < 2 THEN
        RAISE EXCEPTION 'transaction % has fewer than two entries', NEW.transaction_id
            USING ERRCODE = 'check_violation';
    END IF;

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER entries_must_balance
    AFTER INSERT ON entries
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION assert_transaction_balances();

-- An entry's currency must match the account it touches.
CREATE OR REPLACE FUNCTION assert_entry_currency_matches_account() RETURNS TRIGGER AS $$
DECLARE
    account_currency CHAR(3);
BEGIN
    SELECT currency INTO account_currency FROM accounts WHERE id = NEW.account_id;
    IF account_currency IS DISTINCT FROM NEW.currency THEN
        RAISE EXCEPTION 'entry currency % does not match account % currency %',
            NEW.currency, NEW.account_id, account_currency
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER entries_currency_matches_account
    BEFORE INSERT ON entries
    FOR EACH ROW EXECUTE FUNCTION assert_entry_currency_matches_account();

-- ---------------------------------------------------------------------------
-- idempotency
-- ---------------------------------------------------------------------------
CREATE TABLE idempotency_keys (
    key             TEXT        NOT NULL,
    user_id         UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    request_hash    TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'in_progress',
    response_status INTEGER,
    response_body   JSONB,
    transaction_id  UUID        REFERENCES transactions (id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,

    PRIMARY KEY (key, user_id),
    CONSTRAINT idempotency_status_valid CHECK (status IN ('in_progress', 'completed', 'failed'))
);

-- For the cleanup job that expires stale keys.
CREATE INDEX idempotency_keys_created_at_idx ON idempotency_keys (created_at);

-- ---------------------------------------------------------------------------
-- reconciliation
-- ---------------------------------------------------------------------------
-- The materialised balance must always equal the sum of the account's entries.
-- Run this on a schedule; any row returned is a bug worth an alert.
CREATE VIEW balance_drift AS
SELECT
    a.id AS account_id,
    a.balance_minor AS stored_balance,
    COALESCE(SUM(e.amount_minor), 0) AS computed_balance,
    a.balance_minor - COALESCE(SUM(e.amount_minor), 0) AS drift
FROM accounts a
LEFT JOIN entries e ON e.account_id = a.id
GROUP BY a.id, a.balance_minor
HAVING a.balance_minor <> COALESCE(SUM(e.amount_minor), 0);

-- ---------------------------------------------------------------------------
-- seed: system accounts
-- ---------------------------------------------------------------------------
INSERT INTO accounts (type, currency, owner_id) VALUES
    ('system', 'BRL', NULL),
    ('system', 'USD', NULL);
