# Roadmap: Distributed Ledger Service in Go

A single repository, built in five phases over roughly four to five months at ~10 hours/week. Each phase is independently presentable, so you can start applying to jobs after Phase 2 and keep improving the same project.

**Assumption:** you know Go syntax, have written at least a small HTTP API, and are comfortable with SQL. If not, do Phase 0 first.

**Rule for the whole project:** every phase ends with something that runs, is tested, and is documented. A half-finished Phase 3 is worth less than a finished Phase 2.

---

## The domain

A double-entry ledger with user wallets. Chosen because the invariants are strict, unfakeable, and directly relevant to the fintech companies that hire Go in Brazil.

### Core rules the system must never break

1. **Every transaction balances.** The sum of all entries in a transaction is exactly zero. Money moves between accounts, it is never created or destroyed inside the system.
2. **Entries are immutable.** You never `UPDATE` or `DELETE` an entry. A mistake is corrected by writing a new reversing transaction.
3. **Wallet balances never go negative.** System accounts (like the external funding account) are allowed to go negative; user wallets are not.
4. **Idempotency is exact.** The same idempotency key replayed returns the original result and produces no second effect.

### Money representation

`int64` in minor units (centavos), plus a 3-letter currency code. Never `float64`. Never `float32`. A `Money` value type with `Add`, `Sub`, and `Neg` methods that return errors on overflow.

If you want a different domain, event ticketing with seat reservation and freight tracking have equally hard invariants. The phases below apply unchanged.

---

## Phase 0 — Foundations (skip if you already have these)

Only if you need it. Two to three weeks.

- Go tour plus *Learn Go with Tests* (quii) for the testing habits
- `context.Context`: cancellation, deadlines, why it is the first parameter
- Goroutines, channels, `sync.Mutex`, `sync.WaitGroup`, `errgroup`
- The race detector: `go test -race`. Use it from day one and never turn it off
- SQL transactions and isolation levels. Read about `READ COMMITTED` vs `REPEATABLE READ` in Postgres specifically

---

## Phase 1 — The API, done properly

**Goal:** a single service that provably cannot lose money under concurrent load.
**Time:** 4–6 weeks.
**This is the phase most people rush. Don't.**

### Milestones

**1.1 — Scaffolding**
- Repo layout: `cmd/api/`, `internal/domain/`, `internal/storage/`, `internal/http/`, `internal/config/`, `migrations/`
- `Makefile` with `run`, `test`, `lint`, `migrate-up`, `migrate-down`
- `docker-compose.yml` with Postgres 16 and Redis
- `golangci-lint` configured and passing
- Config from environment variables, no config files committed

**1.2 — Schema and migrations**

Tables: `accounts`, `transactions`, `entries`, `idempotency_keys`, `users`.

```
accounts(id, owner_id, currency, type, created_at)
transactions(id, idempotency_key, description, created_at)
entries(id, transaction_id, account_id, amount_minor, created_at)
```

Constraints to add at the database level, not just in Go:
- `CHECK (amount_minor <> 0)` on entries
- A deferred constraint or trigger asserting entries per transaction sum to zero
- Unique index on `idempotency_keys(key, user_id)`

Whether you store balance as a materialized column on `accounts` or compute it as `SUM(entries.amount_minor)` is your first real architecture decision. Write an ADR for it. (Recommendation: materialized column updated in the same transaction, with a periodic reconciliation job that recomputes from entries and alerts on drift. That job is itself a good talking point.)

**1.3 — Domain layer**
- Pure Go, zero database imports. `Money`, `Account`, `Transaction`, `Entry`
- A `Transfer` function that takes amounts and returns entries, validating the balance rule
- 100% unit-tested with table-driven tests. This is the one place where high coverage is genuinely worth chasing

**1.4 — Storage layer**
- `pgx/v5` connection pool
- `sqlc` for generated, type-safe queries from raw SQL
- A `WithTx(ctx, fn)` helper so business logic can compose multiple queries in one transaction without leaking `*sql.Tx` everywhere

**1.5 — HTTP layer**
- Endpoints: register, login, create account, deposit, withdraw, transfer, get balance, list entries (paginated, cursor-based not offset-based)
- Request validation with `go-playground/validator`
- Error responses in RFC 9457 Problem Details format. Never leak internal errors to clients
- Middleware: request ID, structured logging, panic recovery, timeout

**1.6 — Authentication**
- Argon2id password hashing (`golang.org/x/crypto/argon2`)
- JWT access tokens (short-lived) plus refresh tokens stored server-side so they can be revoked
- RBAC middleware: a user can only touch their own accounts; an admin role can read all

**1.7 — Idempotency**
- Middleware that reads the `Idempotency-Key` header on all mutating endpoints
- First request: insert the key with status `in_progress`, run the handler, store the response, mark `completed`
- Replay: return the stored response
- Concurrent replay while the first is still running: return `409 Conflict`

**1.8 — The concurrency milestone (the important one)**

Write a test that spawns 500 goroutines, each transferring 1 centavo from account A to account B, and asserts the final balances are exactly correct.

Then write the harder one: 250 goroutines transferring A→B and 250 transferring B→A **simultaneously**. This will deadlock on your first attempt. The fix is to always acquire row locks in a deterministic order (sort the two account IDs and lock the lower one first). Finding and fixing this yourself is the single most valuable hour in the whole roadmap, and it is a story you can tell in every interview.

Run both with `-race`.

**1.9 — Test suite**
- Unit tests for domain (no DB)
- Integration tests with `testcontainers-go` spinning up real Postgres. Do not mock the database. Mocked SQL tests prove nothing about the behaviour you care about here
- One end-to-end test that hits the HTTP layer

**1.10 — API documentation**
- An OpenAPI 3.1 spec, hand-written or generated
- Swagger UI served at `/docs` in dev

### Libraries

| Concern | Pick |
|---|---|
| Router | `gin-gonic/gin`, or stdlib `net/http` (the Go 1.22+ mux is genuinely sufficient) |
| Postgres driver | `jackc/pgx/v5` |
| Query generation | `sqlc` |
| Migrations | `golang-migrate/migrate` or `pressly/goose` |
| Validation | `go-playground/validator/v10` |
| Logging | `log/slog` (stdlib) |
| JWT | `golang-jwt/jwt/v5` |
| UUIDs | `google/uuid` |
| Testing | `stretchr/testify`, `testcontainers/testcontainers-go` |
| Lint | `golangci-lint` |

### Done when

- [ ] `docker compose up` then `make migrate-up` gives a working API on a clean machine
- [ ] Both concurrency tests pass reliably, with `-race`, ten runs in a row
- [ ] Replaying any mutating request with the same idempotency key produces no second effect
- [ ] A reconciliation script confirms every account's stored balance equals the sum of its entries
- [ ] README explains the domain rules and how to run everything

### Traps

- Floats for money. Instant credibility loss.
- Mocking the database in tests.
- Building hexagonal architecture with six layers of interfaces before you have a working feature. Add abstraction when a second implementation actually appears.
- Passing `context.Background()` inside handlers instead of threading the request context.

---

## Phase 2 — Event-driven split

**Goal:** the same behaviour, now spread across services communicating asynchronously, without losing a single event.
**Time:** 4–5 weeks.
**This is the phase that gets you interviews.**

### Milestones

**2.1 — The transactional outbox**

The core pattern. You cannot atomically write to Postgres and publish to Kafka, so you don't try.

- `outbox` table: `id, aggregate_id, event_type, payload (jsonb), created_at, published_at`
- Every domain change writes its event row inside the **same** database transaction as the balance change
- A relay goroutine polls for unpublished rows, publishes to Kafka, marks them published
- Use `FOR UPDATE SKIP LOCKED` so multiple relay instances can run without stepping on each other

Write down in your ADR why this gives at-least-once delivery and not exactly-once, and what that forces consumers to do.

**2.2 — Kafka setup**
- Use **Redpanda** in docker-compose instead of Kafka + Zookeeper. Kafka-API compatible, one container, starts in two seconds
- Topics: `ledger.transactions.v1`, `ledger.accounts.v1`, plus a `.dlq` topic per consumer
- **Partition key = account ID.** This preserves per-account ordering while still allowing parallelism. Be ready to explain why in an interview
- Event schema versioned in the topic name; payloads as JSON now, protobuf in Phase 3

**2.3 — Consumer services**

Two new services, same repo (a monorepo is fine and arguably better here):
- **Notification service** — consumes transaction events, sends (mock) emails
- **Projection service** — consumes events and builds a read-optimised monthly statement table

**2.4 — Idempotent consumers**
- A `processed_events(event_id, consumer_name, processed_at)` table with a unique constraint
- Consumer checks-and-inserts before acting. Duplicate delivery becomes a no-op
- Test it: deliver the same event three times, assert one effect

**2.5 — Failure handling**
- Retry with exponential backoff and jitter
- After N failures, publish to the DLQ topic with the error attached
- A small admin endpoint to list and replay DLQ messages

**2.6 — A saga with compensation**

Model an external withdrawal against a mock payment provider:

1. Reserve funds (move from wallet to a `pending_withdrawals` system account)
2. Call the provider
3. On success: move from pending to the external account
4. On failure or timeout: compensating transaction moves funds back to the wallet

Persist the saga state machine in a table. Make the mock provider fail 30% of the time and time out 10% of the time, then prove no money is ever lost or duplicated.

**2.7 — Integration tests for the whole flow**

testcontainers running Postgres *and* Redpanda. Publish, consume, assert. Slow tests are fine; wrong tests are not.

### Libraries

| Concern | Pick |
|---|---|
| Kafka client | `twmb/franz-go` (recommended) or `segmentio/kafka-go` (simpler API) |
| Backoff | `cenkalti/backoff/v4` |
| Concurrency | `golang.org/x/sync/errgroup` |
| Local broker | Redpanda via docker-compose |

### Done when

- [ ] Killing the relay mid-publish and restarting it loses nothing and duplicates no effects
- [ ] Killing a consumer mid-processing and restarting it produces the same final state
- [ ] The saga test passes with a flaky provider across 1000 runs
- [ ] An architecture diagram in the README shows services, topics, and data flow

---

## Phase 3 — gRPC between services

**Goal:** typed, versioned internal contracts with REST still on the edge.
**Time:** ~2 weeks.

### Milestones

- `proto/` directory with service and message definitions
- **`buf`** for linting, breaking-change detection, and code generation. Do not hand-roll `protoc` invocations
- Internal service-to-service calls move from HTTP to gRPC
- `grpc-gateway/v2` generates a REST/JSON facade so external clients are unaffected
- Interceptors for auth, logging, panic recovery, and (in Phase 5) tracing
- gRPC health checking protocol implemented, plus server reflection so `grpcurl` works
- Deadlines propagated: a client timeout must actually cancel server work. Write a test proving it

### Libraries

`grpc/grpc-go`, `grpc-ecosystem/grpc-gateway/v2`, `grpc-ecosystem/go-grpc-middleware/v2`, `bufbuild/buf`, `protobuf-go`.

### Done when

- [ ] `buf breaking` runs in CI and fails on an incompatible proto change
- [ ] Cancelling a client request visibly stops server-side work
- [ ] Both the gRPC and REST surfaces are documented

---

## Phase 4 — Ship it

**Goal:** it runs somewhere with a public URL, deployed by a pipeline, not by you.
**Time:** 3–4 weeks.

### Milestones

**4.1 — Containers**
- Multi-stage Dockerfile, `CGO_ENABLED=0`, final stage `gcr.io/distroless/static` or `scratch`
- Version and commit SHA injected via `-ldflags`, exposed on a `/version` endpoint
- Target under 20MB per image

**4.2 — Kubernetes**
- Deployment, Service, ConfigMap, Secret
- Liveness, readiness, **and** startup probes. Readiness must actually check the database connection
- Resource requests and limits set deliberately, with a note on how you chose them
- HorizontalPodAutoscaler on CPU, PodDisruptionBudget
- Graceful shutdown: `SIGTERM` drains in-flight requests before exit. Test it under load

**4.3 — Helm**
- One chart, values files per environment
- `helm lint` and `helm template` in CI

**4.4 — Terraform**
- Managed Postgres, Redis, and the cluster
- Remote state, not local
- Keep it cheap: develop against `kind` or `k3d` locally, then deploy once to something inexpensive (Oracle Cloud free tier, Hetzner, Fly.io, or GCP/AWS free credits). Set a billing alert before you apply anything

**4.5 — CI/CD with GitHub Actions**
- On PR: lint, unit tests, integration tests with service containers, `buf breaking`, build
- On merge to main: build and push to GHCR, deploy
- Branch protection requiring the checks to pass
- Trivy or govulncheck scanning images and dependencies

### Done when

- [ ] A public URL a recruiter can open
- [ ] `git push` to main results in a deployed change with no manual steps
- [ ] Rolling deploy under active load drops zero requests
- [ ] No secret has ever been committed (verify with `gitleaks`)

---

## Phase 5 — Observability and performance

**Goal:** turn the project into numbers you can put on a résumé.
**Time:** 3–4 weeks. Highest résumé value per hour.

### Milestones

**5.1 — Instrumentation**
- OpenTelemetry SDK with OTLP export
- `otelhttp`, `otelgrpc`, and `otelpgx` for automatic spans on HTTP, gRPC, and database calls
- Manual spans around business logic worth measuring
- Trace context propagated through Kafka headers so a trace spans producer and consumer. This is the impressive one
- `trace_id` injected into every structured log line

**5.2 — Metrics and dashboards**
- Prometheus `/metrics` endpoint
- RED metrics (rate, errors, duration) per endpoint, plus business metrics: transactions per second, total value moved, outbox lag, consumer lag
- Grafana dashboard committed as JSON in the repo
- Alert rules: outbox lag over threshold, error rate over 1%, p99 over target

Stack choice: Grafana + Prometheus + Tempo, or SigNoz as a single self-hosted alternative. Both accept OTLP.

**5.3 — Load testing**
- k6 scripts for realistic scenarios: read-heavy, write-heavy, and hot-account contention
- Run against a deployed environment, not your laptop
- Record baseline numbers before optimising anything

**5.4 — Profiling and optimisation**
- `net/http/pprof` behind auth
- Capture CPU and heap profiles under load, find a real bottleneck, fix it, re-measure
- Likely candidates: N+1 queries on statement listing, JSON marshalling, allocations in hot paths, missing indexes

**5.5 — Caching, correctly**
- Redis cache on balance reads with explicit invalidation on write
- `golang.org/x/sync/singleflight` to prevent cache stampede when a hot key expires
- Measure and record the hit rate

**5.6 — Resilience**
- Distributed rate limiter: token bucket in Redis via a Lua script for atomicity. Do not use a naive `INCR`, and be ready to explain why
- Circuit breaker (`sony/gobreaker`) around the external payment provider
- Bounded worker pools with backpressure on consumers, so a burst does not exhaust memory

**5.7 — The performance report**

A `docs/performance.md` with a table: metric, before, after, what changed. This is where your résumé bullets come from.

### Done when

- [ ] One trace shows a request flowing API → Kafka → consumer → database
- [ ] The Grafana dashboard is screenshotted in the README
- [ ] `docs/performance.md` contains real measured before/after numbers
- [ ] You can explain every number in it

---

## Repository presentation

The README is judged before the code. Structure it as:

1. One paragraph: what this is and what problem it solves
2. Architecture diagram (Excalidraw or Mermaid, committed to the repo)
3. The domain invariants, stated plainly
4. Quickstart: `docker compose up` and one command
5. Tech stack, grouped by concern
6. Link to `docs/adr/` and `docs/performance.md`
7. Screenshots: Grafana dashboard, a Jaeger trace, k6 output

Write 5–8 short ADRs in `docs/adr/`. Each one: context, decision, alternatives considered, consequences. Suggested topics: materialized vs computed balances, outbox vs CDC, at-least-once with idempotent consumers, monorepo vs polyrepo, gRPC internally with REST at the edge, Redis rate limiter design.

**Write everything in English.** Brazilian remote listings for Go roles list English as a requirement alongside the technical skills, and this repo is your English writing sample.

Commit in small, well-messaged increments throughout. A repo with 400 commits over four months reads very differently from one with three commits called "initial commit", "wip", "final".

---

## Résumé bullets this project produces

Fill in your own measured numbers. Do not invent them.

- Built a double-entry ledger service in Go handling N req/s with p99 latency of Xms, guaranteeing balance consistency under concurrent writes via row-level locking with deterministic lock ordering
- Implemented the transactional outbox pattern with Kafka to publish domain events at-least-once, with idempotent consumers and a dead-letter queue, achieving zero event loss across failure-injection tests
- Designed a saga with compensating transactions for external withdrawals against an unreliable provider, verified across 1000 runs with 30% induced failure rate
- Cut p99 latency from Xms to Yms by eliminating N+1 queries and adding a Redis cache layer with singleflight stampede protection
- Instrumented N services with OpenTelemetry, propagating trace context through Kafka to give end-to-end visibility across async boundaries
- Deployed to Kubernetes via Helm and Terraform with a GitHub Actions pipeline running lint, unit, and integration tests on every PR

---

## Suggested schedule at 10 hours/week

| Weeks | Phase |
|---|---|
| 1–6 | Phase 1 |
| 7–11 | Phase 2 |
| 12–13 | Phase 3 |
| 14–17 | Phase 4 |
| 18–21 | Phase 5 |

Start applying to jobs at week 11. The project does not need to be finished for it to be worth discussing, and interview feedback will tell you which phases to prioritise.

If you fall behind, cut Phase 3 before you cut Phase 2 or Phase 5. gRPC is the easiest of the three to learn on the job; distributed data consistency and observability are not.
