---
description: Rules for application commands, queries, and ports
paths:
  - "**/usecases/**"
  - "**/queries/**"
  - "**/ports/**"
---

# Application Layer Rules

## Ownership

- Usecases own application authorization and workflow sequencing. Domain methods own invariants. Repositories and adapters own persistence and protocol mapping.
- Keep protobuf, database rows, transactions, and provider SDK types out of application contracts.
- Usecases and queries **never** import `adapters/` packages or storage drivers. Port errors (`ports.ErrNotFound`, `ports.ErrConcurrentModification`, `ports.ErrDuplicate`) live in `ports/`, so application code can classify them without an adapter import.
- One operation per file with an operation-named type: `usecases/place.go` → `type Place` with `Execute`. Large contexts may group by aggregate (`usecases/order/place.go`). Never `usecases/place/place.go`, never a generic `Interactor`.
- A usecase never imports another usecase for reuse; move shared policy to the domain or a focused port.

## Persistence

- Load through `repo.Get`, call domain methods, persist through `repo.Save`. The usecase never opens a transaction, writes outbox rows, or touches the aggregate tracker.
- Same-type atomic batches use `repo.SaveAll`. Different aggregate types with one real same-store invariant need a dedicated, tested coordinator that acknowledges all only after one commit. No shared invariant → save independently with explicit partial-result semantics.
- Cross-store or provider effects: persist durable intent (outbox) with the state change, then process idempotently and reconcile.
- Idempotency is **required** when (a) a client-facing create or batch write may be retried after a timeout, or (b) a remote side effect follows the local commit. Pass an `IdempotencyRecord` to `Save` so it commits atomically. See `.handbook/patterns/idempotency.md`.
- On `ports.ErrConcurrentModification`, either return it (mapped to `Aborted`) or reload and re-apply the command a bounded number of times — never retry blindly with stale state.

## Dependencies and Results

- Inject only the collaborators the operation uses via a `<Operation>Deps` value; validate once in `New<Operation>` with `deps.Require`. No request-path nil checks for required collaborators afterwards.
- Constructor tests reject each missing dependency and accept valid wiring.
- Tenant and actor identity arrive in the request from the authenticated principal; they reach every key and port call.
- Generate time and IDs through injected ports, capture them once per operation, and pass them to the domain.
- Configuration arrives as typed values from the composition root; never call `os.Getenv` in application or domain code.
- Commands return `error`, plus a small result type only when the caller needs created or computed data.

## Queries

- Queries are read-only: no `Save`, no hidden writes, no cache writes that change business state.
- They enforce tenant scope and authorization, bound every list (default and maximum page size), use deterministic ordering with a unique tie-breaker, and return DTOs — never aggregates or wire types.

## Context, Errors, and Concurrency

- `context.Context` is the first parameter of `Execute` and every port method; never stored.
- Wrap with operation and identity context once (`%w`); do not log and return.
- Prefer synchronous code; bound, cancel, and wait for any justified concurrency.

## Verification

- Test authorization and tenant propagation, success, domain rejection, repository failure (pending work retained), conflict handling, idempotent replay and race-loser recovery, cancellation, and external unknown outcomes.
- Check handler, job, and consumer wiring plus configuration in the full diff.

See `.handbook/layers/application/usecases.md` and `.handbook/templates/usecase.md`.
