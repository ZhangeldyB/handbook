---
name: scaffold-service
description: >-
  Bootstrap a new Go gRPC service skeleton following the engineering handbook:
  module, lint and buf configuration, Makefile, platform packages (aggregate
  tracker, deps, auth, grpcerr, validation, outbox, messaging), a composition
  root with lifecycle and graceful shutdown, and CI-ready verification. Use when
  starting a new service or repository from scratch.
---

# Scaffold a Service

Create a working, tested skeleton — not a pile of empty directories.

## Inputs

- Module path (for example `example.com/orders`)
- First bounded context name
- Database and migration tool, broker (or "outbox only for now"), event encoding (JSON or protobuf)

Ask for any input you cannot infer. Record the service-specific choices in `docs/adr/0001-<service>-foundations.md` from `.handbook/decisions/000-template.md`.

## Read First

1. `.handbook/guides/new-service.md` — the authoritative step list; follow it in order.
2. `.handbook/architecture/service-layout.md`.
3. `.handbook/templates/composition-root.md`, `.handbook/layers/infrastructure/wiring.md`, `.handbook/layers/infrastructure/graceful-shutdown.md`, `.handbook/layers/shared/configuration.md`.

## Steps

1. `go mod init <module>`; set the `go` directive deliberately (current stable, at least 1.26).
2. Copy `.handbook/templates/golangci.yml` → `.golangci.yml` (replace `example.com/orders` with the module and the context path), `buf.yaml`, `buf.gen.yaml` (replace `<module>`), `Makefile.service` → `Makefile`, `Dockerfile`.
3. Pin protoc plugins: `go get -tool google.golang.org/protobuf/cmd/protoc-gen-go@<version>` and `go get -tool google.golang.org/grpc/cmd/protoc-gen-go-grpc@<version>` (versions from the `buf.gen.yaml` header).
4. Copy the platform packages **with their tests** from `.handbook/examples/orders/internal/platform/{aggregate,deps,messaging,outbox}` and `.handbook/examples/grpctransport/{auth,grpcerr,validation}` into `internal/platform/`, rewriting import paths. Replace the outbox's `memstore` usage with the chosen database's transaction API.
5. Create `internal/platform/<database>` (connection, `InTx` helper that commits or rolls back and checks the rollback error) and `internal/platform/telemetry` (slog JSON logger, OpenTelemetry tracer provider).
6. Write `internal/<service>/config` (or `cmd/server/config.go`) with a `Load(getenv)` that validates everything and joins errors.
7. Write `cmd/server/main.go` from the composition-root template: `main` → `run(ctx, ...) error`, interceptor chain (recovery → auth → error mapping), health service, `errgroup` lifecycle with the outbox relay, bounded shutdown.
8. Create `proto/<org>/<context>/v1/` with a minimal service, then `buf lint` and `make generate`.
9. Create `migrations/` with the outbox and idempotency tables (see `.handbook/patterns/outbox.md` and `.handbook/layers/infrastructure/migrations.md`).
10. Install the handbook into the new repository if it is not already (`scripts/install.sh` from a handbook checkout).

## Guardrails

- No business code in this skill; the first vertical slice uses `create-aggregate`, `add-repository`, `implement-usecase`, and `add-grpc-handler`.
- No `util`/`common` packages; no DI framework; no global state.
- Do not invent tool versions — use the pinned versions from the handbook templates or look them up.

## Verify

```bash
make lint
go test ./...
go build ./cmd/server
```

Then start the binary against a local database, call the health check, and send SIGTERM: it must exit cleanly within the configured shutdown budget.

## Done

- [ ] Lint, tests, and build pass on the skeleton.
- [ ] Platform packages copied with their tests passing.
- [ ] Composition root with validated config, lifecycle group, and bounded shutdown.
- [ ] buf lint and generate succeed; generated code compiles.
- [ ] Foundations ADR written.
