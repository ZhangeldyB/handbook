# ADR-002: Repositories Own Mapping and the Transaction

**Status:** Accepted
**Date:** 2026-10-28

## Context

If usecases build rows, open transactions, or write outbox records themselves, transaction boundaries become implicit, events get forgotten, and storage types leak into application code. If handlers apply persistence, the boundary leaks further.

## Decision

- The application-facing persistence port is semantic: `Get(ctx, tenantID, id)` and `Save(ctx, aggregate, records ...IdempotencyRecord)`, plus `SaveAll` for a same-type atomic batch.
- `Save` owns row mapping, the version check, outbox appends, idempotency inserts, and exactly one database transaction.
- Ports never expose rows, transactions, query builders, or `any`.
- A real invariant spanning different aggregate types in one store gets a dedicated, tested coordinator port — still one semantic operation from the usecase's point of view.
- Work spanning stores or providers uses state plus durable intent plus reconciliation, never a pretend transaction.

## Alternatives Considered

- **Unit of Work passed to usecases** (`uow.Do(ctx, func(tx) ...)`): flexible, but makes every usecase responsible for transaction correctness and leaks the transaction type inward. Kept only as an internal mechanism of a coordinator adapter.
- **Generic repository (`Repository[T]`)**: hides the per-aggregate mapping and version rules that matter most.

## Consequences

- **Easier:** usecases read as workflows; transaction scope is reviewable in one place; failure-path tests target one method.
- **Harder:** cross-aggregate atomic writes need an explicit coordinator instead of ad hoc composition.

## Enforcement

`depguard` rule `application` denies storage drivers and adapter imports in usecases, queries, and ports. Repository tests (round trip, retained work after failed commit, conflict, duplicate key, tenant isolation) are required by the [repo rules](../.agents/rules/repo-rules.md).
