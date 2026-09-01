# Ledger

A double-entry ledger service in Go. Money moves between accounts through
balanced transactions; it is never created, destroyed, or lost, including under
concurrent load.

> Scaffolding for phase 1 of the roadmap. Sections marked TODO are the work.

## Invariants

The system enforces four rules, most of them in the database rather than in
application code, so that a bug in Go still cannot corrupt the books.

1. **Transactions balance.** Every transaction's entries sum to exactly zero
   per currency. Enforced by a deferred constraint trigger that fires at commit.
2. **Entries are immutable.** `UPDATE` and `DELETE` on `entries` are rewritten
   to no-ops by a rule. Mistakes are corrected with reversing transactions.
3. **Wallets never go negative.** A check constraint on `accounts`. System
   accounts are exempt, which is how deposits from outside the system balance.
4. **Idempotency is exact.** A replayed `Idempotency-Key` returns the original
   response and causes no second effect.

## Architecture

TODO: add a diagram once phase 2 introduces the second service.

```
cmd/api              entrypoint, wiring, graceful shutdown
internal/domain      business rules; no database, no HTTP, fully unit-testable
internal/storage     Postgres pool, transactions, sqlc-generated queries
internal/httpapi     routing, middleware, error translation
migrations           schema, forward and back
docs/adr             why things are the way they are
```

## Running it

Requires Go 1.25+, Docker, and `make`.

```bash
cp .env.example .env
make tools        # one-off: installs migrate, sqlc, golangci-lint, govulncheck
make up           # starts Postgres and Redis, waits for readiness
make migrate-up
make run
```

Then:

```bash
curl localhost:8080/healthz
curl localhost:8080/readyz
curl localhost:8080/version
```

Useful targets: `make test-race`, `make lint`, `make cover`, `make drift`,
`make reset`, `make help`.

## Testing

- **Unit tests** cover `internal/domain` with no external dependencies.
- **Integration tests** run against a real Postgres via testcontainers. The
  database is not mocked: the invariants being verified live in the database.
- Everything runs under `-race` in CI. A flaky test under the race detector is
  a real bug, not a reason to remove the flag.

## Status

- [x] Phase 1.1 project scaffolding
- [x] Phase 1.2 schema and migrations
- [ ] Phase 1.3 domain layer (`domain.NewTransfer`, `Transaction.Validate`)
- [ ] Phase 1.4 storage layer
- [ ] Phase 1.5 HTTP handlers
- [ ] Phase 1.6 authentication
- [ ] Phase 1.7 idempotency
- [ ] Phase 1.8 concurrency correctness
- [ ] Phase 1.9 test suite
- [ ] Phase 1.10 OpenAPI spec

## License

MIT
