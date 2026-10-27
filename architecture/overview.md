# Architecture Overview

Services combine **ports and adapters** (dependencies point inward), a small amount of **domain-driven design** (aggregates own invariants), and **CQRS-lite** (commands go through the domain; queries read projections directly). The goal is understandable ownership, durable correctness, and cheap change — not the maximum number of layers.

## Use the Model Proportionally

Start from the problem, not from a template.

| Situation | Shape |
|---|---|
| State changes guarded by business rules (lifecycles, money, quotas) | Aggregate + usecase + repository |
| Straightforward CRUD with only validation and authorization | A usecase calling a repository directly is fine; no aggregate ceremony |
| Read-only endpoints | A query over a read port; never load aggregates just to read |
| Libraries, CLIs, generators | Idiomatic package-oriented Go, no layers |

Do not introduce an aggregate, facade, interface, or event because the handbook has an example of one. Every abstraction must pay for a real variant, seam, invariant, or lifecycle.

## Dependency Direction

```text
cmd/<binary> (composition root) ──► wires everything
adapters/grpc, adapters/eventconsumer, adapters/jobs ──► usecases, queries
usecases, queries ──► ports, domain
adapters/persistence, adapters/clients ──► implement ports (import ports, domain, queries types)
domain ──► nothing outside the domain except pure platform primitives
```

- The **domain** owns business invariants. It has no I/O, takes no `context.Context`, and receives time and IDs as parameters.
- The **application layer** owns authorization and workflow orchestration. It passes `context.Context` to I/O and never stores it.
- **Adapters** translate protocols and storage, manage resources, and implement ports.
- **Transports** validate wire input, obtain trusted identity, call one application operation, and return its error to a central mapper.

Interfaces belong to the consumer that needs substitution. A concrete dependency is the right default when there is one implementation and no test seam is needed.

## Principles

- **One owner per fact.** Every invariant and authoritative value has exactly one owning aggregate or service. Others consume a contract or a non-authoritative projection.
- **Explicit inputs.** Pass tenant, actor, time, and IDs explicitly. A context carries cancellation, deadlines, tracing, and the authenticated principal — never business parameters.
- **Constructor injection.** Components receive the collaborators they call from one composition root and validate them once. No service locators, no globals.
- **Fail closed.** Unknown enum values, missing identity, and unmapped errors are rejected or treated as internal failures, never guessed.
- **Exact money.** Integer minor units or exact decimals with an explicit currency. Never binary floating point.
- **Honest atomicity.** Only one database transaction is atomic. Everything else is at-least-once delivery plus idempotency plus reconciliation.

## The Command Flow

```text
handler
  → usecase: authorize, load, orchestrate
  → aggregate: validate invariant, change state, record event
  → repository.Save
       → one transaction: state (version-checked) + outbox records + idempotency records
       → acknowledge pending work only after commit
  → handler returns the result; the error mapper translates failures
outbox relay (later, asynchronously) → broker → consumers (idempotent)
```

Details and every failure path: [Request Flow](request-flow.md).

## Non-Negotiable Correctness Boundaries

- Never call an external provider inside a database transaction or from an aggregate.
- Never discard pending events before the commit succeeds.
- Never accept a tenant from the request when an authenticated principal exists.
- Never claim atomicity across databases, brokers, caches, or providers.
- Make idempotency scope, retry classification, timeouts, and unknown outcomes explicit.
- Every goroutine has a bounded purpose, a cancellation path, and an owner that waits for it.
- Schema, API, event, and generated-code changes include a compatibility and rollout analysis.
- Telemetry is safe and actionable: no secrets, no unbounded label cardinality.

## Verification Philosophy

Tests follow risk, not a coverage percentage. The highest-value tests prove business transitions, mapping completeness, state-plus-outbox atomicity, retention of pending work after a failed commit, tenant isolation, error classification, and the failure boundary most likely to corrupt state. See [Testing](../testing/README.md).

## Continue Reading

- [Request Flow](request-flow.md)
- [Service Layout](service-layout.md)
- [Domain Layer](../layers/domain/README.md)
- [Repositories](../layers/infrastructure/repositories.md)
- [Transactional Outbox](../patterns/outbox.md)
