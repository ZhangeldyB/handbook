# ADR-005: Optimistic Concurrency With Versioned Full-State Writes

**Status:** Accepted
**Date:** 2026-10-28

## Context

Two requests can load the same aggregate, both pass their domain checks, and both save. Without a guard, the second write silently overwrites the first ("lost update"). Field-level change tracking reduces the overlap but does not remove it, and adds a mapping per field.

## Decision

- Every aggregate row carries an integer `version`.
- Inserts write version 1. Updates are compare-and-set: `UPDATE ... SET ..., version = $next WHERE tenant_id = $t AND id = $id AND version = $expected`; zero affected rows means `ports.ErrConcurrentModification`.
- `Save` writes the aggregate's full state when it is dirty; there is no per-field change tracking.
- The transport maps `ErrConcurrentModification` to `codes.Aborted`. Callers either surface it or reload and re-apply the command a bounded number of times.
- Very wide or hot rows may justify partial updates later; that is a measured, per-aggregate exception recorded in its own ADR.

## Alternatives Considered

- **Last-write-wins with field tracking:** cheaper writes, but lost updates remain possible and every field needs a hand-written mapping.
- **Pessimistic row locks (`SELECT ... FOR UPDATE`):** correct but holds locks across application logic and couples usecases to transactions.

## Consequences

- **Easier:** lost updates become an explicit, testable error; mapping is one function per direction.
- **Harder:** callers must handle `Aborted`; very hot aggregates may contend and need redesign.

## Enforcement

Repository tests must include a stale-writer case ([example](../examples/orders/internal/orders/adapters/persistence/repository_test.go)).
