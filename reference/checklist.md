# PR Checklist

## The Whole Change

- [ ] One coherent change; its size is justified by the behavior, not limited by an arbitrary file count.
- [ ] Callers, invariant owners, persistence, side effects, contracts, configuration, generated code, tests, and wiring were traced — not just the edited file.
- [ ] Incident-driven changes separate facts, hypotheses, and unknowns and were verified against the deployed revision ([Evidence-Driven Debugging](../guides/evidence-driven-debugging.md)).
- [ ] No stale path, dead code, unrelated dependency churn, or second implementation of the same policy.

## Gates

- [ ] `make lint` passes (formatters, linters, `buf lint`).
- [ ] `go test ./...` passes; `-race` for changed concurrent code.
- [ ] Integration tests for changed persistence, queries, or adapters.
- [ ] `buf breaking` passes for proto changes; generated code regenerated and clean.
- [ ] `govulncheck` for dependency, toolchain, parser, or authentication changes.
- [ ] Every check reported as passing actually ran.

## Architecture

- [ ] Domain code has no context, I/O, logging, or infrastructure types; time and IDs are parameters.
- [ ] Each aggregate method validates before mutating and records its event in the same method; no event-draining methods.
- [ ] Independently identified objects are separate aggregates referenced by ID.
- [ ] One operation per usecase file; operation-named type; no adapter imports.
- [ ] Components receive only the collaborators they call, validated once with `deps.Require`; no request-path nil checks.
- [ ] Persistence goes through `Save`/`SaveAll`: one transaction, version check, outbox, idempotency records, ack after commit.
- [ ] Handlers: identity from the principal, wire validation, one call, errors returned for the central mapper.
- [ ] New domain errors that clients should see have a mapper rule.

## Correctness

- [ ] Every new or changed persisted field is in the snapshot, both mapping functions, the round-trip fixture, and a migration.
- [ ] Public and persisted fields define presence, default, validation, units, mutability, concurrency, and old/new reader behavior; absent versus zero is tested where it matters.
- [ ] Retried creates and event consumers are idempotent; remote effects use durable intent and reconciliation.
- [ ] No provider call inside a transaction or an aggregate; unknown outcomes are recorded, not retried blindly.
- [ ] Money is exact (minor units or decimal) everywhere, including tests.
- [ ] Tenant from the principal reaches every key, query, idempotency record, cache key, and event.
- [ ] Concurrency is bounded, cancellable, and awaited; retries are capped and jittered.

## Contracts and Rollout

- [ ] Proto field numbers never reused; removed numbers and names reserved.
- [ ] Schema changes follow expand → migrate → contract and tolerate the rolling deploy.
- [ ] Event topics and payloads evolve compatibly; consumers were checked.
- [ ] Feature flags have an owner, defaults, both-state tests, telemetry, rollback, and a removal condition.

## Operability

- [ ] Errors logged once at the boundary; no secrets or raw payloads in logs, spans, errors, or metrics.
- [ ] Metric labels are bounded.
- [ ] New background work exposes backlog/age and participates in graceful shutdown.
- [ ] Changes to the server or `cmd/` keep readiness, bounded drain, and dependency close order correct ([Graceful Shutdown](../layers/infrastructure/graceful-shutdown.md)).
