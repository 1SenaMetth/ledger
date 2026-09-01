# First run

## 1. Rename the module

The scaffold uses a placeholder module path. Replace it with your own before
the first commit, because changing it later means touching every import.

```bash
NEW=github.com/<your-github-username>/ledger
grep -rl 'github.com/1SenaMetth/ledger' . | xargs sed -i.bak "s|github.com/1SenaMetth/ledger|$NEW|g"
find . -name '*.bak' -delete
```

macOS uses BSD sed, so `sed -i.bak` above is written to work on both macOS and
Linux.

## 2. Resolve dependencies

The scaffold declares two direct dependencies. Let Go work out the rest:

```bash
go mod tidy
```

That creates `go.sum` and pulls `pgx` and `uuid`. Everything else so far is
standard library.

## 3. Install the tooling

```bash
make tools
```

Installs `migrate`, `sqlc`, `golangci-lint`, and `govulncheck` into
`$(go env GOPATH)/bin`. Make sure that directory is on your `PATH`.

## 4. Start the dependencies and run

```bash
cp .env.example .env
make up
make migrate-up
make run
```

In another terminal:

```bash
curl -i localhost:8080/healthz
curl -i localhost:8080/readyz
curl -i localhost:8080/version
```

`/readyz` returning 503 means the database is unreachable, which is exactly
what it is supposed to do.

## 5. Confirm the invariants are live

The interesting part of the scaffold is in the database, not the Go code. Prove
it to yourself before writing any handlers:

```bash
psql "postgres://ledger:ledger@localhost:5432/ledger?sslmode=disable"
```

```sql
-- an unbalanced transaction is rejected at COMMIT, not at INSERT
BEGIN;
INSERT INTO transactions (id) VALUES ('11111111-1111-1111-1111-111111111111');
INSERT INTO entries (transaction_id, account_id, amount_minor, currency)
SELECT '11111111-1111-1111-1111-111111111111', id, -100, 'BRL'
FROM accounts WHERE type = 'system' AND currency = 'BRL';
COMMIT;  -- ERROR: transaction ... does not balance in BRL: net movement is -100
```

Note where the error appears. The insert succeeds; the commit fails. That is
the deferred constraint trigger doing its job, and it is why a transfer can
insert its two legs one at a time.

```sql
-- entries cannot be rewritten
UPDATE entries SET amount_minor = 999999;  -- UPDATE 0
DELETE FROM entries;                       -- DELETE 0
```

## 6. Your first task

```bash
make test-race
```

Four tests are skipped. Open `internal/domain/ledger.go`, implement
`Transaction.Validate` and `NewTransfer`, delete the `t.Skip` lines in
`internal/domain/ledger_test.go`, and make them pass.

Do not touch the database for this. Both functions are pure: they take accounts
and an amount, and return entries or an error. That separation is what lets the
business rules be tested in milliseconds without Docker running.

After that, work through the phase 1 checklist in `README.md` in order.

## Notes

- `make test-race` rather than `make test`. The race detector costs a few
  seconds and catches the class of bug this project is built to demonstrate.
- Commit in small increments from the start. The commit history is part of what
  a reviewer reads.
- `git init && git add . && git commit -m "scaffold ledger service"` before you
  change anything, so you can always diff against the starting point.
