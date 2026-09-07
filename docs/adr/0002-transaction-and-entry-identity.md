# ADR-0002: Generate transaction and entry identity in the domain layer

- **Status:** accepted
- **Date:** 2026-09-07

## Context
The database schema requires every ledger entry to have a transaction ID before it can be written to Postgres,
because that column is strictly non-null and has no default value. This physical constraint forces the system 
to produce and link these identifiers before the actual database rows exist.

## Decision
The domain layer generates the transaction ID and both entry IDs upfront before any database interaction occurs.
However, the creation timestamps are intentionally left blank so that Postgres can fill them in using its own internal clock when the rows are saved.

## Alternatives considered
- **Storage layer generation (Postgres default UUIDs + returning clause):** I considered inserting the transaction
row first to let the database generate the ID, retrieving it, and then stitching it onto the entries before saving them. 
This was rejected because it forces the application to wait for an I/O operation to complete before it has an identity to work with.
If a request needs to be traced end-to-end or logged immediately, this approach creates a blind spot where the transaction exists in memory
but has no identifiable name until the database round-trip finishes.

- **Using the client-supplied idempotency key as the primary key:** I considered using the idempotency key passed by the caller as the actual
transaction ID. This was rejected because it conflates two different lifecycles: idempotency is a transport-level concern for safe retries,
while the transaction ID is the immutable core identity of a ledger event. Merging them forces the database's primary key structure to trust the client's 
formatting and uniqueness guarantees, which makes debugging harder if clients generate poor keys.

## Consequences
This decision makes it much easier to observe and track a transaction throughout its lifecycle, starting from the exact moment it is requested.
The trade-off is that Go's UUID generation relies on the system's cryptographic random number generator, which means the domain layer now touches the outside world
rather than remaining pure computation. Furthermore, having the domain generate the identity while the database generates the time creates an architectural
asymmetry that might confuse future developers if the reasoning isn't clearly documented here.
