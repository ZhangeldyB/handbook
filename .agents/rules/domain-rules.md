---
description: Rules for domain layer files (aggregates, value objects, events, domain errors)
paths:
  - "**/domain/**"
---

# Domain Layer Rules

Apply these to bounded contexts that own business behavior. Do not manufacture aggregates or a domain layer in a library, CLI, or CRUD-only package.

## Purity and Ownership

- The domain owns business invariants and state transitions. Aggregate state is private and changes only through invariant-preserving methods.
- Domain methods are deterministic and perform no I/O. They **must not** accept or store `context.Context` — adding `ctx` as the first parameter is a common reflex; never do it here.
- Do not import database drivers, transport or protobuf packages, loggers, configuration/env loaders, adapters, usecases, or other infrastructure. Pure platform primitives (the aggregate tracker) and other domain packages are allowed. `depguard` enforces this.
- Time, IDs, and randomness are parameters supplied by the application layer.
- Reference other aggregates by ID. Cross-aggregate rules live in a pure domain service or an explicitly coordinated application workflow.
- Never nest a mutable sub-aggregate. Anything with its own identity, lifecycle, or repository is a separate aggregate referenced by ID; only identity-free value objects live inside an aggregate.

## Aggregate Lifecycle

- Keep an `aggregate.Tracker[Event]` in an unexported field; create it with `aggregate.NewForCreate` in the factory and `aggregate.NewForReconstitute(version)` in `Reconstitute`. The zero tracker panics.
- State-changing methods call `tracker.Changed()` or `tracker.Record(event)`; they never return events. Expose `BeginPersist()` (delegating to `tracker.Begin`) for the repository and read-only `IsDirty()`/`PendingEvents()`; never export `Record`.
- Never add destructive `PullEvents`/`DrainEvents` methods — the repository acknowledges events only after commit.
- Provide `Snapshot()` returning complete state and `Reconstitute(Snapshot)` that maps every field, records nothing, and rejects structurally impossible state.

## Behavior

- One method owns each transition: validate everything first, then mutate state and record the event. A rejected call leaves state and pending events unchanged.
- Keep the legal-transition table in one place (`<entity>_status.go` or `status.go`) as `map[From][]To`.
- Decide same-state behavior from the operation contract (idempotent no-op, return existing result, or reject); it is not universal.
- Events are past-tense facts with stable event ID, tenant, aggregate ID, and occurrence time. Evolve them compatibly.
- Never perform an external side effect from an aggregate.

## API and Types

- Export only what callers need: the aggregate type, constructors, commands, read-only getters, stable errors, events, and value types.
- Getters returning slices or maps return `slices.Clone`/`maps.Clone`; constructors copy incoming slices.
- Getters have no `Get` prefix. Use functional options only for genuinely optional, extensible inputs.
- Money uses an exact representation (integer minor units or a decimal type) with an explicit currency — never `float32`/`float64`.
- Domain enums are domain-owned types whose zero value is invalid; map to protobuf and storage values explicitly in adapters.

## Errors

- Stable sentinel/typed errors for conditions callers classify; lowercase text, no trailing punctuation.
- Wrap with useful operation and identity context (`fmt.Errorf("place order %q: %w", id, err)`).
- No transport status codes in the domain.

## Verification

- Test valid and invalid transitions, unchanged state after rejection, create versus reconstitute, recorded events, alias safety of returned collections, and snapshot round trips.
- Aggregates are single-owner; race tests belong around the code that shares them, not the aggregate itself.

See `.handbook/layers/domain/aggregates.md` and `.handbook/templates/domain.md`.
