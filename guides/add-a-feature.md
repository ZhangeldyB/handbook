# Add a Feature

A change workflow, not a mandate to create every layer. Reuse the existing context and package structure; add only the boundaries the feature needs.

## 1. Build the Change Map

Before editing, trace:

- public entry points and callers;
- the owner of every business invariant involved;
- state reads and writes, and the transaction boundary;
- events, jobs, external calls, caches, and consumers;
- protobuf, schema, and generated-code contracts;
- wiring, configuration, deployment, and observability;
- the success path and the riskiest failure, retry, or cancellation path;
- existing unit, integration, and E2E coverage.

For substantial work, write the plan down. A feature is not complete because one new file compiles.

## 2. Evolve Contracts First

When the feature changes protobufs, events, or schemas: keep field numbers, reserve removed ones, add compatible fields instead of changing meanings, regenerate with the repository target, run `buf breaking`, update every producer and consumer, and plan the rolling deploy and rollback ([API Contract Evolution](api-contract-evolution.md), [Migrations](../layers/infrastructure/migrations.md)).

## 3. Domain Behavior

If the feature belongs to an aggregate, add a method that validates, mutates, and records its event ([Aggregates](../layers/domain/aggregates.md)). New persisted fields go into the `Snapshot`, both mapping functions, the round-trip fixture, and a migration. Do not create an aggregate for pure transport mapping or a read.

## 4. Persistence

Extend the row mapping and the event converter. If a new event type is recorded, the converter must handle it — the unknown-type branch fails the save otherwise. Keep `Save`'s single transaction; never add a second write path ([Repositories](../layers/infrastructure/repositories.md)).

## 5. Usecase or Query

Add `usecases/<operation>.go` with its `Deps`, or a query method plus reader method. Inject time and IDs, take the tenant from the request, and decide idempotency with the [selection rule](../patterns/idempotency.md#when-it-is-required). External effects go through durable intent and [Reconciliation](../patterns/reconciliation.md).

## 6. Transport

Add the RPC handler: validation, principal, mapping, one call, errors returned unchanged. Add mapper rules for new domain errors. Register the RPC in the composition root and update authorization configuration.

## 7. Wire and Observe

Update the constructor graph, server, consumer and job registration, configuration, and manifests. Add telemetry at the owning boundary only: stable operation names, outcome classes, bounded labels, no secrets.

## 8. Verify at the Right Levels

```bash
make lint
go test ./...
go test -race ./path/to/affected/concurrent/...
```

- Domain tests for transitions, rejected-state immutability, and events.
- Repository round-trip, failed-commit retention, and conflict tests; integration tests for new queries and constraints.
- Usecase tests at ports, including idempotent replay where applicable.
- Handler tests for identity, validation, mapping, and codes.
- E2E only for critical public composition.
- `govulncheck` if dependencies changed.

## Final Checkpoint

- [ ] Every caller, consumer, and generated contract updated.
- [ ] One owner per invariant.
- [ ] State and events commit atomically; failure retains pending work.
- [ ] Batch, cross-store, and external effects have explicit durable semantics.
- [ ] Tenant, authorization, idempotency, cancellation, and lifecycle are correct.
- [ ] Wiring, config, telemetry, and shutdown complete.
- [ ] Tests prove success and meaningful failures.
- [ ] The diff has no stale path, dead code, accidental dependency churn, or second implementation of a policy.
