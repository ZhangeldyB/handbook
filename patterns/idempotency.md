# Idempotency

Callers retry after timeouts, `Unavailable` errors, and crashes. A retried write must return the original result — never create a duplicate, never "start over".

## When It Is Required

| Situation | Requirement |
|---|---|
| Client-facing create or batch write that callers retry | **Required** — pattern A |
| An event consumer's effect | **Required** — pattern A with the event ID as key ([Event Consumer](event-consumer.md)) |
| A remote side effect follows the local commit | **Required** — pattern B |
| Naturally idempotent operations (set status to X, delete by ID) | Usually satisfied by domain semantics; verify |
| Everything else | Recommended where duplicates would be harmful |

Do not retrofit idempotency into code you are touching for unrelated reasons; do add it when you build an operation that meets the table.

## Pattern A: Idempotency Record in the Same Transaction

Implemented end to end in the example ([`CreateDraft`](../examples/orders/internal/orders/usecases/create_draft.go)):

```text
lookup (tenant, scope, key)
  → hit, same fingerprint: return the stored response
  → hit, different fingerprint: ErrIdempotencyKeyReused (INVALID_ARGUMENT)
  → miss: run the command
       → repo.Save(aggregate, IdempotencyRecord{Scope, Key, Fingerprint, Response})
            the record is INSERTED in the same transaction as the state
       → ErrDuplicate (a concurrent request won): lookup again, return the winner's response
```

Contract:

- **Same result, never "start over".** A replay returns the originally created resource. If the stored response references something that no longer exists, return an explicit error — never silently create again.
- **Scoped keys.** The key is unique per `(tenant, scope, key)`, where scope names the operation (`CreateDraft`) or consumer (`consume:payments.payment_captured.v1`). One client key may be reused across different operations.
- **Fingerprint.** A hash of the request fields that define its meaning (excluding the key). Reusing a key with a different payload is a client error, not a replay.
- **Insert, never upsert.** A racing request with the same key must lose its whole commit, not overwrite the record.
- **Race-loser recovery.** On `ErrDuplicate`, look up again and return the winner's result.
- **Expiry.** Records may expire after a documented window (hours to days) chosen from client retry behavior; afterwards the key behaves as new. Reads treat expired records as misses; garbage collection deletes them later.

## Pattern B: Durable Operation Record for Remote Effects

When a remote effect follows the local commit, the local transaction cannot cover it, so the intent must be durable. Model the operation as an aggregate committed together with the business change:

| Field | Purpose |
|---|---|
| `operation_id` | Stable identity, also sent to the provider as its idempotency key |
| `kind` (versioned, e.g. `capture_payment_v1`) | Incompatible replay semantics get a new version |
| `fingerprint` | Same key + different request → conflict |
| `status` | `PENDING` → `COMPLETED` / `FAILED_RETRYABLE` / `FAILED_TERMINAL` / `OUTCOME_UNKNOWN` / `RECONCILIATION_REQUIRED` |
| `attempts`, `next_attempt_at`, `last_error_class` | Retry policy from durable state |
| `response` | The recorded result returned on replay |
| `expires_at` | Never expires while incomplete; a bounded retention window after completion |

A retry finds the operation and branches on status: completed → return the recorded response; pending or retryable → resume only the remote step; unknown → reconcile with the provider first; reconciliation-required → surface the conflict, never auto-replay. Full workflow: [Reconciliation](reconciliation.md).

## Both Patterns

- The tenant is part of every key.
- A failed or timed-out commit does not prove nothing committed — the idempotency record is exactly what makes the retry safe.
- Keys and fingerprints are not logged in full.

## Testing

- Replay returns the original result; the store saw one write.
- Key reuse with a different payload is rejected.
- A concurrent duplicate (simulated by committing the competing record just before save) returns the winner's result.
- The record rolls back with the state when the save fails.
- Expired records behave as misses.

See the [usecase tests](../examples/orders/internal/orders/usecases/usecases_test.go) and [repository tests](../examples/orders/internal/orders/adapters/persistence/repository_test.go).
