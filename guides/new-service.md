# New Service From Scratch

A step-by-step bootstrap for a new gRPC service. Each step ends in a working, tested state; do them in order. The [`scaffold-service`](../.agents/skills/scaffold-service/SKILL.md) skill automates the skeleton.

## 0. Decide Before Coding

Write a short ADR (template: [000-template.md](../decisions/000-template.md)) recording:

- the bounded context(s) this service owns and the facts it is authoritative for;
- the database and migration tool;
- the broker (or "outbox only, broker later");
- whether events are JSON or protobuf;
- tenancy model and how the principal is authenticated;
- whether generated code is committed.

The handbook's ADRs already decide the rest (pure domain, `Save` with outbox, optimistic versions, exact money, central error mapping, buf).

## 1. Module and Tooling

```bash
mkdir orders && cd orders
go mod init example.com/orders
go mod edit -go=1.26.0   # or the current stable release; set it deliberately
mkdir -p cmd/server internal/platform proto migrations
cp .handbook/templates/golangci.yml .golangci.yml            # replace example.com/orders
cp .handbook/templates/buf.yaml .handbook/templates/buf.gen.yaml .
cp .handbook/templates/Makefile.service Makefile
```

Commit with `make lint` and `make test` passing on the empty module.

## 2. Platform Primitives

Copy the dependency-free platform packages from [`examples/orders/internal/platform`](../examples/orders/internal/platform/) and keep their tests:

| Package | Purpose |
|---|---|
| `aggregate` | Retry-safe pending-work tracker |
| `deps` | Constructor dependency validation |
| `outbox` | Outbox record and relay (replace the `memstore` calls with your database) |
| `messaging` | Message envelope and publisher/handler contracts |

And from [`examples/grpctransport`](../examples/grpctransport/): `auth` (principal in context), `grpcerr` (central error mapping), `validation` (field violations). Add `platform/<database>` (connection, `InTx` helper, migrations runner) and `platform/telemetry` (slog + OpenTelemetry providers).

## 3. Contract First

Write `proto/acme/orders/v1/orders.proto` following [Protobuf Contracts](../layers/infrastructure/proto-contracts.md). Run `buf lint` and `buf generate`. The first contract only needs the RPCs for the first slice of behavior.

## 4. Composition Root and Lifecycle

`cmd/server/main.go`: `main` → `run(ctx, getenv) error`; config loading and validation ([Configuration](../layers/shared/configuration.md)); logger and tracer provider; database; gRPC server with recovery, authentication, and error-mapping interceptors plus the health service; an `errgroup` running the server and the outbox relay; bounded graceful shutdown ([Graceful Shutdown](../layers/infrastructure/graceful-shutdown.md)).

Verify: the binary starts against a local database, serves the health check, and exits cleanly on SIGTERM within the budget.

## 5. First Vertical Slice

Implement one command end to end, in this order, with tests at each step:

1. **Migration**: the aggregate table (with `tenant_id`, `version`), the outbox table, the idempotency table ([Migrations](../layers/infrastructure/migrations.md)).
2. **Domain**: the aggregate, its status table, events, errors ([Aggregates](../layers/domain/aggregates.md), skill `create-aggregate`).
3. **Ports**: repository, clock, ID generator.
4. **Repository**: `Get`/`Save` with version check, outbox, idempotency; round-trip and failure-path tests; integration tests against the real database ([Repositories](../layers/infrastructure/repositories.md), skill `add-repository`).
5. **Usecase**: with `Deps` validation and fakes ([Usecases](../layers/application/usecases.md), skill `implement-usecase`).
6. **Handler**: validation, principal, mapping; register it ([gRPC Handlers](../layers/infrastructure/grpc-handlers.md), skill `add-grpc-handler`).
7. **Error rules**: add the new domain errors to the mapper table.

Verify with an E2E test through the generated client against the running binary ([E2E Testing](../testing/e2e-testing.md)).

## 6. Events

Wire the relay to a real broker adapter implementing `messaging.Publisher`. Add consumers with the [Event Consumer](../patterns/event-consumer.md) pattern. Alert on outbox backlog age and dead records.

## 7. CI

The pipeline runs, in order: formatting and lint (`golangci-lint run`), `buf lint` and `buf breaking`, generation cleanliness (regenerate and fail on diff), unit tests, `-race` on concurrent packages, migrations against an empty database plus integration tests, `govulncheck`, container build and image scan, then E2E against a deployed test environment. See [Testing and CI](../testing/testing-ci.md).

## 8. Operate

Before the first production deploy: dashboards for request rate/errors/latency, dependency health, and outbox and consumer backlog age; alerts on SLO burn and backlog age; a runbook for dead-lettered events and stuck reconciliations; verified shutdown under the real termination grace period.

## Done Checklist

- [ ] ADR for service-specific choices.
- [ ] Lint, buf, tests, race, and govulncheck green in CI.
- [ ] One vertical slice with unit, integration, and E2E tests.
- [ ] Outbox relay running and observable.
- [ ] Graceful shutdown verified.
- [ ] Handbook installed (`.handbook/`, `AGENTS.md`, `CLAUDE.md`, `.claude/`).
