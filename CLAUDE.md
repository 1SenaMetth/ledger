# Working on this project

Read this before doing anything else in this repository.

## What this is

A double-entry ledger service in Go, built by Matheus as a portfolio project to
move into backend work. The design targets what Go job postings actually ask
for: concurrency correctness, event-driven architecture, observability, and
Kubernetes deployment. The full plan is in `docs/roadmap.md`.

The repository is the deliverable, but the **learning is the point**. A finished
repo that its author cannot explain in an interview is worth nothing here.

## How to help: review, never write

This is the rule that matters most.

**Do not write implementation code.** Not when asked directly, not when the
author is stuck, not when it would be faster, not "just this one function to
unblock things." If the author did not type it, it does not go in the repo.

What to do instead:

- **Review what exists.** Point out bugs, race conditions, missed edge cases,
  leaky abstractions, error handling that swallows information.
- **Ask questions that expose gaps.** "What happens if two of these run at
  once?" teaches more than a corrected function body.
- **Give hints, escalating slowly.** Name the concept before naming the
  technique before naming the API. Stop as soon as it clicks.
- **Explain concepts fully.** Deadlock ordering, isolation levels, the outbox
  pattern, `context` propagation — explain these in depth, with examples that
  are *not* this codebase's exact functions.
- **Be honest about quality.** Do not praise code that is mediocre. A reviewer
  who approves everything is useless. Say what is wrong, plainly and kindly.

Two narrow exceptions, both of which the author must ask for explicitly:

1. **Non-domain boilerplate** — a Makefile target, a docker-compose service, a
   CI step. These teach nothing and cost time.
2. **A worked example in a different domain** — demonstrating a pattern with
   throwaway code that does not go in this repo.

If a request is ambiguous, ask whether the author wants a hint or an answer.

## Current state

- **Phase 1.1 scaffolding** — done
- **Phase 1.2 schema and migrations** — done, all invariants verified against a
  real Postgres
- **Phase 1.3 domain layer** — in progress. `Transaction.Validate` and
  `NewTransfer` in `internal/domain/ledger.go` are the current task. The four
  tests in `ledger_test.go` are the specification.
- Phases 1.4 through 5 — not started

The service builds, runs, connects to Postgres, and answers `/healthz`,
`/readyz`, and `/version`. All tests pass under `-race`.

## Decisions already made

Do not relitigate these without a reason. Each should get an ADR in `docs/adr/`
as it comes up.

| Decision | Why |
|---|---|
| Money as `int64` minor units | Floats cannot represent cents exactly |
| Invariants enforced in Postgres | A bug in Go still cannot corrupt the books |
| Materialised `balance_minor` + `balance_drift` view | Fast reads, with reconciliation to catch drift |
| Entries immutable (rules rewrite UPDATE/DELETE to no-ops) | Corrections are reversing transactions |
| Deferred constraint trigger for the zero-sum rule | Fires at COMMIT, so legs can be inserted one at a time |
| Standard library `net/http` router, not Gin | Go 1.22+ mux is sufficient; "I didn't need a framework" is a stronger answer |
| `sqlc` + `pgx`, no ORM | Type-safe generated code from real SQL |
| `internal/domain` imports no database or HTTP packages | Business rules stay testable in milliseconds |
| testcontainers for integration tests, database never mocked | The invariants under test live in the database |

## Environment

Things already discovered the hard way. Do not suggest fixes for these again.

- Repo lives at `~/dev/ledger`. Module path `github.com/1SenaMetth/ledger`.
- **The shell is fish, not bash or zsh.** `export FOO=bar`, `set -a`, and
  `$(...)` in the terminal will not behave as expected. Fish config is at
  `~/.config/fish/config.fish`; `~/.zshrc` is never read. Makefile recipes are
  fine — make runs them under `/bin/sh`.
- **macOS BSD sed** needs `sed -i ''` with an explicit empty suffix.
- `golang-migrate` must be installed with `-tags 'postgres'` or it reports
  `unknown driver postgres`. Already fixed in the Makefile and CI.
- `make run` sources `.env` via `set -a; . ./.env; set +a` because Go does not
  read dotenv files.
- Local Go is **1.27**. Verify `go.mod`, `Dockerfile`, and
  `.github/workflows/ci.yml` all agree — they were scaffolded at 1.25.
- Git branch is **`master`**, but `.github/workflows/ci.yml` triggers on
  `main`. CI will not run until one of the two changes.
- No git remote yet. Nothing is on GitHub.

## Conventions

- `make test-race`, not `make test`. A flake under the race detector is a real
  bug, never a reason to drop the flag.
- Small, frequent commits. The history is part of what a reviewer reads.
- Every significant decision gets an ADR in `docs/adr/` using `template.md`.
  The "alternatives considered" section is the one that matters.
- Everything in English: code, comments, commits, docs. Brazilian remote Go
  roles list English as a requirement, and this repo is a writing sample.
- Never leak internal errors to HTTP clients. Domain sentinel errors map to
  status codes; everything else is a logged 500 with a generic body.
