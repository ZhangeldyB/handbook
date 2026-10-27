# Domain Layer

The domain owns business meaning and invariants. It is the pure center of a bounded context that actually needs one.

## Responsibilities

- Aggregate state transitions and the invariants that guard them.
- Value objects with validated construction (money, quantities, identifiers with rules).
- Stable domain errors and past-tense domain events.
- Pure policies and domain services for rules that span values or aggregates.

## Boundaries

- No `context.Context`, no I/O, no logging, no configuration or environment reads.
- No database, transport, protobuf, or SDK types.
- Private state; callers use intention-revealing methods and read-only getters.
- Other aggregates are referenced by ID.
- Time, IDs, and randomness arrive as parameters.
- Repository interfaces are **not** here — they live in [`ports/`](../application/ports.md).

## Contents

- [Aggregates](aggregates.md) — shape, lifecycle tracker, constructors, transitions
- [Domain Events](domain-events.md) — recording, converting, and evolving events
- [Domain Services](domain-services.md) — pure cross-aggregate logic
- [Policies](policies.md) — named, reusable business decisions
- [Domain Errors](domain-errors.md) — error design and the canonical transport mapping
- [Money](../shared/money.md) — exact money values

Templates: [domain.md](../../templates/domain.md), [state-machine.md](../../templates/state-machine.md). Compiled example: [`examples/orders/internal/orders/domain`](../../examples/orders/internal/orders/domain/order.go).
