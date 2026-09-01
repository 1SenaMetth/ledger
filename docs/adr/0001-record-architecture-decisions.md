# ADR-0001: Record architecture decisions

- **Status:** accepted
- **Date:** 2026-09-01

## Context

This project is deliberately built to demonstrate engineering judgement, not
just working code. Judgement is invisible in a diff: a reviewer sees that
balances are stored on the account row, but not that computed balances were
considered and rejected, nor why.

## Decision

Record every significant technical decision as a short markdown file in
`docs/adr/`, numbered sequentially and never edited after acceptance. A decision
that changes gets a new ADR that supersedes the old one, leaving the reasoning
trail intact.

## Alternatives considered

- **A wiki or Notion page.** Drifts out of sync with the code and is invisible
  to anyone reading the repository.
- **Long code comments.** Good for explaining *how* a function works, poor for
  explaining why a whole approach was chosen over another.
- **No documentation.** The default, and the reason most portfolio projects
  cannot answer "why did you do it this way" in an interview.

## Consequences

Writing an ADR takes about fifteen minutes and forces the alternatives to be
articulated before committing to one. The set of ADRs doubles as interview
preparation: each one is a rehearsed answer to a design question.

Planned ADRs for this project:

- 0002 Materialised balances versus computed balances
- 0003 Money as int64 minor units
- 0004 Transactional outbox versus change data capture
- 0005 At-least-once delivery with idempotent consumers
- 0006 Standard library router versus a framework
- 0007 Distributed rate limiter design
