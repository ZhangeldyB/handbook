# ADR-004: Acknowledge Pending Work Only After Commit

**Status:** Accepted
**Date:** 2026-10-28

## Context

An aggregate's state change and the events it records are one logical transition. A common implementation exposes `PullEvents()`, which returns and *clears* the pending events before the repository writes them. If event conversion or the transaction then fails, the in-memory aggregate has lost its events; a retry commits the state without them, silently violating the outbox guarantee.

## Decision

Aggregates track pending work with `aggregate.Tracker[E]` ([implementation](../examples/orders/internal/platform/aggregate/tracker.go)):

- `NewForCreate` starts dirty and new; `NewForReconstitute(version)` starts clean.
- Domain methods call `Changed()` or `Record(event)`.
- The repository calls `Begin()`, which returns an immutable snapshot (`IsNew`, `ExpectedVersion`, `NextVersion`, `Events`) and an `ack` function.
- `ack` is called only after the transaction committed. It acknowledges exactly the captured work; anything recorded later stays pending; stale or repeated acks are ignored.
- A batch begins every aggregate, commits once, then acknowledges all.

No destructive `Pull`/`Drain` methods exist.

## Consequences

- **Easier:** a failed save can be retried on the same in-memory aggregate without losing events; the failure path is directly testable.
- **Harder:** aggregates need two small delegation methods (`BeginPersist`, `IsDirty`); batch writers must collect acks.
- Delivery remains at least once — consumers own deduplication.

## Enforcement

Repository tests must prove that a failed commit retains pending events and that the retry writes the outbox exactly once ([example](../examples/orders/internal/orders/adapters/persistence/repository_test.go)).
