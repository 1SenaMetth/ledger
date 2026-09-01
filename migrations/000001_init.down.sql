DROP VIEW IF EXISTS balance_drift;

DROP TABLE IF EXISTS idempotency_keys;

DROP TRIGGER IF EXISTS entries_currency_matches_account ON entries;
DROP TRIGGER IF EXISTS entries_must_balance ON entries;
DROP FUNCTION IF EXISTS assert_entry_currency_matches_account();
DROP FUNCTION IF EXISTS assert_transaction_balances();

DROP TABLE IF EXISTS entries;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS users;
