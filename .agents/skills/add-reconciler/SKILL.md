---
name: add-reconciler
description: >-
  Add a durable reconciliation workflow for an external effect whose outcome
  can be confirmed, failed, or unknown: persisted intent, leased workers,
  provider calls with idempotency keys, outcome recording, unknown-outcome
  reconciliation, and capped retries. Use for payments, provisioning, shipping,
  or any provider call that must happen exactly once in effect.
---

# Add a Reconciler

## Read First

1. `.handbook/patterns/reconciliation.md` (fully) and `.handbook/patterns/idempotency.md` (pattern B).
2. `.handbook/layers/infrastructure/grpc-clients.md` and `.handbook/templates/adapters.md`.
3. The provider's API: does it accept an idempotency key? Does it offer a status lookup?

## Trace and Decide First

Write down: the desired authoritative state; the single transaction boundary; the stable tenant-scoped operation ID; the provider outcomes (confirmed success, confirmed retryable, confirmed terminal, unknown); how unknown is resolved (status lookup, or manual); the lease and version strategy; retry, age, and dead-letter policy. Do not scaffold until these are explicit.

## Steps

1. Model the operation as an aggregate with phases (`PENDING`, `IN_FLIGHT`, `CONFIRMED_SUCCESS`, `CONFIRMED_FAILURE_RETRYABLE`, `CONFIRMED_FAILURE_TERMINAL`, `OUTCOME_UNKNOWN`) and its transition table (`create-aggregate`).
2. The initiating usecase saves the business change and the `PENDING` operation in one `Save`/coordinator transaction.
3. Implement the provider port and adapter: send the operation ID as the idempotency key; classify outcomes from status codes and typed errors; timeouts on non-idempotent calls are `ErrOutcomeUnknown`.
4. Implement one `Reconcile(ctx, tenantID, operationID)` entry point: load, check lease and version, perform or look up, persist the outcome in a new transaction. If the local write fails after provider success, leave the operation unresolved — never re-issue the effect blindly.
5. Implement the worker: claim due operations with a lease, bounded concurrency, capped jittered backoff from durable attempts, the three cycle outcomes (converged / transient / permanent), and terminal states that alert instead of silently dropping.
6. Wire the worker into the lifecycle group; add metrics and alerts for unknown and pending age.

## Guardrails

- Never call the provider inside a database transaction.
- Never collapse unknown into failed.
- A global lock does not replace per-operation leases and idempotency.
- In-memory queues are scheduling aids, never the source of truth.

## Verify

Use a provider fake that can apply the effect and then time out. Valid and invalid phase transitions; intent and state commit atomically; provider success followed by a failed local write converges without a duplicate; timeout before and after apply both reconcile; expired and duplicate leases are safe; retry caps and terminal classification; termination at every boundary recovers; shutdown stops claims and waits.

## Done

- [ ] Intent committed before the provider call.
- [ ] Operation ID used as the provider idempotency key.
- [ ] Unknown outcomes reconciled before retry.
- [ ] Leases, versions, and capped retries; nothing dropped silently.
- [ ] Observable and tested at every boundary.
