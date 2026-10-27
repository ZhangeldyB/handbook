---
name: add-repository
description: >-
  Implement a persistence adapter for an aggregate: complete row mapping,
  Get/Save/SaveAll with a version check, outbox and idempotency writes in one
  transaction, acknowledgement after commit, error translation, and the
  required round-trip and failure-path tests. Use when adding persistence for
  an aggregate or implementing a repository port.
---

# Add a Repository

## Inputs

- Bounded context and aggregate
- The database and its transaction helper in `internal/platform/<database>`

## Read First

1. `.handbook/.agents/rules/repo-rules.md`.
2. `.handbook/layers/infrastructure/repositories.md` and `.handbook/templates/repository.md`.
3. `.handbook/patterns/outbox.md` (record schema) and `.handbook/patterns/idempotency.md`.
4. Compiled example: `.handbook/examples/orders/internal/orders/adapters/persistence/` (all four files).
5. The aggregate's `Snapshot`, events, and the port in `ports/`.

## Steps

1. Migration (use `add-migration`): the aggregate table with `tenant_id` leading the primary key and a `version` column; outbox and idempotency tables if absent.
2. `rows.go`: unexported `<entity>Row`; `toRow(Snapshot, version)` and `toSnapshot(row)` mapping every field, validating on read through domain constructors.
3. `events.go`: topic constants, versioned payload types, `toOutboxRecord` with an error for unknown event types.
4. `repository.go`:
   - `Get`: tenant-scoped read, "no rows" → `ports.ErrNotFound`, `Reconstitute`.
   - `Save`: `BeginPersist` → map outside the transaction → one transaction (insert or version compare-and-set, outbox appends, idempotency inserts) → `ack()` only after commit.
   - `SaveAll` when callers batch: begin all, one transaction, ack all.
   - Translate unique violations and version mismatches to `ErrConcurrentModification`/`ErrDuplicate`.
   - `var _ ports.<Entity>Repository = (*Repository)(nil)`.
5. Reader methods for queries, if needed (keyset pagination, tenant-first predicates).
6. Wire it in the composition root.

## Guardrails

- No business rules or transport mapping here.
- No external calls inside the transaction; callbacks deterministic.
- Parameterized SQL only; allowlisted dynamic columns.
- Never upsert idempotency records; never acknowledge before commit.

## Verify

Reflection-checked round trip; state + outbox commit together; failed commit keeps pending work and the retry writes the outbox exactly once; stale writer → `ErrConcurrentModification`; duplicate idempotency key rolls back everything; tenant isolation; `SaveAll` acknowledges nothing on failure. Run these against the real database in integration tests.

```bash
go test ./internal/<context>/adapters/persistence/...
go test -tags=integration -count=1 ./internal/<context>/adapters/persistence/...
make lint
```

## Done

- [ ] Every field mapped both ways; round-trip test fixture sets every field.
- [ ] One transaction per save; version check; outbox; idempotency; ack after commit.
- [ ] Storage errors translated to port errors.
- [ ] Unit and integration tests pass.
