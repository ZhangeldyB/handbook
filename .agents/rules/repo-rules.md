---
description: Rules for persistence adapters (repositories, readers, migrations)
paths:
  - "**/adapters/persistence/**"
  - "**/migrations/**"
---

# Repository Rules

Repositories map storage rows to domain state and implement the ports consumed by usecases. They own transactions and storage mechanics, never business policy.

## Retry-Safe Save

The usecase-facing API is `Get(ctx, tenantID, id)` and `Save(ctx, aggregate, records ...IdempotencyRecord)` (plus `SaveAll` for a same-type batch). `Save` **must**:

1. Call `aggregate.BeginPersist()`; return early when nothing is pending and no records were passed.
2. Map the complete state and every pending event *before* opening the transaction, so mapping bugs fail without touching storage.
3. In **one** transaction: insert (new) or compare-and-set update on the stored version (existing); append every event to the outbox; insert (never upsert) every idempotency record.
4. Return `ports.ErrConcurrentModification` on a version mismatch and `ports.ErrDuplicate` on an existing idempotency key.
5. Call `ack()` only after the commit succeeded. On any error, return without acknowledging so the aggregate stays retryable.

`SaveAll` begins every aggregate, writes all of them in one transaction, and acknowledges all only after commit. It never calls `Save` in a loop and never acknowledges an aggregate before the shared commit.

## Mapping

- Map every persisted field in both directions through `Snapshot()`/`Reconstitute`. Keep a round-trip test whose fixture sets every snapshot field (reflect over the struct) so a new field cannot be silently dropped.
- Validate stored values on read (parse enums, money, invariants); never trust rows blindly.
- Unknown event types are an error, never a silent skip.
- Store enums as stable string values, money as integer minor units or exact decimals with currency, times in UTC.

## Transactions and Resources

- Transaction callbacks may be retried by the driver: keep them deterministic and free of external effects, logging of "success", or process-global mutation.
- Never hold a transaction open across a network call to another service or provider.
- Close rows, cursors, and statements on every path; check `rows.Err()`.
- Every key, query, unique constraint, and index that needs isolation includes the tenant ID.
- Use parameterized queries only; never concatenate request values into SQL. Allowlist sortable/filterable columns explicitly.

## Errors

- Translate expected storage conditions (no rows, unique violation, serialization failure) into port errors at this boundary; wrap unexpected ones with operation context. Never leak driver error types upward.
- Do not log and return.

## Prohibited

- Business transitions or transport mapping in repositories.
- Exposing rows, transactions, query builders, or `any` through a port.
- Destructive event draining before commit; acknowledging before commit.
- Hand-written fakes that partially mutate before discovering an error (fakes are all-or-nothing too).

## Verification

- Round-trip mapping completeness; insert versus update routing; clean no-op.
- State and outbox commit together; a failed commit retains pending work and the retry writes the outbox exactly once.
- Stale writer gets `ErrConcurrentModification`; duplicate idempotency key rolls back the whole save.
- Tenant isolation; integration tests against the real database (containers or a dedicated test instance) for queries, constraints, and transactions.

See `.handbook/layers/infrastructure/repositories.md` and `.handbook/templates/repository.md`.
