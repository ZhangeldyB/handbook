---
name: create-aggregate
description: >-
  Design and implement a domain aggregate with private state, the aggregate
  tracker, a complete snapshot, a transition table, recorded events, and domain
  tests. Use when a bounded context needs a new aggregate root, or when the user
  asks to create or scaffold an aggregate, entity, or state machine.
---

# Create an Aggregate

Create a cohesive aggregate that owns a real consistency boundary. Do not generate DDD ceremony for a data container or a CRUD-only package.

## Inputs

- Bounded context
- Aggregate name
- The invariant(s) it owns — if the user cannot name one, propose a simpler design instead

## Read First

1. `.handbook/.agents/rules/domain-rules.md` (the checklist).
2. `.handbook/layers/domain/aggregates.md` and `.handbook/layers/domain/domain-events.md`.
3. `.handbook/templates/domain.md` and `.handbook/templates/state-machine.md`.
4. The compiled example `.handbook/examples/orders/internal/orders/domain/` and its tests.
5. Existing aggregates in this repository, for language and conventions.

## Steps

1. Write down the invariant, the lifecycle states, and what the aggregate deliberately does not own (other aggregates referenced by ID).
2. `status.go`: domain-owned `Status` string type with an invalid zero value, `Valid`, `ParseStatus`, and one `allowedTransitions` table.
3. `events.go`: closed `Event` interface and past-tense event structs carrying event ID, tenant, aggregate ID, and occurrence time.
4. `errors.go`: sentinels for conditions callers branch on.
5. `<entity>.go`: unexported fields plus `tracker aggregate.Tracker[Event]`; `New<Entity>(params)` with `NewForCreate`; `Snapshot` type and `Reconstitute(Snapshot)` with `NewForReconstitute(version)` that rejects impossible state; `Snapshot()`; getters returning copies of collections; command methods that validate → mutate → record; `BeginPersist`, `IsDirty`, `PendingEvents`.
6. Value objects (money, quantities) with validating constructors.
7. Tests (see below).

## Guardrails

- No `context.Context`, I/O, logging, protobuf, or storage imports (`depguard` will fail the build).
- Time and IDs are parameters.
- No exported `Record`, no event-draining methods, no duplicate "isNew" flags.
- No mutable sub-aggregates.
- Money is exact.

## Verify

Table-driven tests for every transition including same-state behavior; unchanged state and zero pending events after rejection; recorded event contents; create versus reconstitute; reconstitution rejects unknown status; returned collections cannot mutate the aggregate; `Reconstitute(x.Snapshot())` round trip.

```bash
go test ./internal/<context>/domain/...
make lint
```

## Done

- [ ] A documented invariant justifies the aggregate.
- [ ] Private state; copies returned; no infrastructure imports.
- [ ] Factory and `Reconstitute` initialize the tracker correctly.
- [ ] Every command validates before mutating and records its event in the same method.
- [ ] Transition table is the single source of truth.
- [ ] Tests listed above pass; lint passes.
