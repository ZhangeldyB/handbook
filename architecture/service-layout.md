# Service Layout

One repository, one deployable service, one or more cohesive bounded contexts. Use the shallowest layout that keeps ownership obvious; add a directory only when it owns something.

```text
<service>/
├── cmd/
│   └── server/
│       └── main.go              # composition root: config, wiring, run(ctx) error, shutdown
├── internal/
│   ├── <context>/               # one bounded context, e.g. orders
│   │   ├── domain/              # aggregates, value objects, events, domain errors (pure)
│   │   ├── ports/               # interfaces the application consumes + port errors
│   │   ├── usecases/            # one command per file: place.go → type Place
│   │   ├── queries/             # read-only operations, DTOs, reader port
│   │   └── adapters/
│   │       ├── grpc/            # handlers + proto ↔ application mapping
│   │       ├── persistence/     # repository, reader, rows, outbox conversion
│   │       ├── eventconsumer/   # one handler per consumed topic
│   │       ├── clients/         # outbound providers and other services
│   │       └── jobs/            # scheduled jobs that call one command each
│   └── platform/                # context-independent infrastructure
│       ├── aggregate/           # pending-work tracker (pure)
│       ├── deps/                # constructor dependency validation
│       ├── auth/                # Principal in context, auth interceptor
│       ├── grpcerr/             # central error → status mapping
│       ├── outbox/              # outbox record + relay
│       ├── messaging/           # message envelope, publisher/handler contracts
│       ├── postgres|...         # database client, transactions, migrations runner
│       └── telemetry/           # slog, OpenTelemetry providers
├── proto/                       # .proto sources (buf module)
├── gen/                         # generated code (never edited)
├── migrations/                  # versioned schema migrations
├── buf.yaml, buf.gen.yaml
├── .golangci.yml
├── Makefile                     # generate, lint, test, test-integration, run
└── Dockerfile
```

## Rules

- **`domain/`** imports only other domain packages and pure platform primitives. Large contexts may split per aggregate (`domain/order`, `domain/shipment`) with shared value types at the context root.
- **`ports/`** holds application-owned interfaces and the stable errors adapters translate into (`ErrNotFound`, `ErrConcurrentModification`, `ErrDuplicate`). Repository interfaces live here, not in `domain/`, so the domain never sees `context.Context`.
- **`usecases/`** is flat for small contexts (`usecases/place.go`) and grouped by aggregate for large ones (`usecases/order/place.go`). Never one package per operation (`usecases/place/place.go`).
- **`queries/`** defines its reader port and DTOs; the persistence adapter implements it.
- **`adapters/`** may import `ports`, `domain`, `queries`, and `usecases` (handlers and consumers call commands). Nothing inward imports an adapter. `depguard` enforces this ([config](../templates/golangci.yml)).
- **`platform/`** never imports a bounded context.
- **`cmd/`** is the only place that knows every concrete type. Keep `main` tiny: `main` calls `run(ctx)`, `run` returns an error, `main` decides the exit code.
- **Generated code** (`gen/`) and migrations have exactly one source of truth and are regenerated, never hand-edited.
- No `util`, `common`, `helpers`, or `models` packages. Name packages by what they provide.

## Several Contexts in One Service

Each context gets its own `internal/<context>/` tree. Contexts do not import each other's `domain`, `usecases`, or `adapters`. They integrate through published events, a narrow read port implemented by an anti-corruption adapter, or the owning context's public API — exactly as if they were separate services. That keeps a later split cheap. See [Bounded Contexts](bounded-contexts.md).

## Where Things Go

| You are adding | Put it in |
|---|---|
| A business rule about one aggregate | A method on the aggregate |
| A rule spanning aggregates, no I/O | A pure function or policy type in `domain/` |
| A new write operation | `usecases/<operation>.go` |
| A new read endpoint | `queries/` + reader method in `adapters/persistence` |
| A new RPC | Proto in `proto/`, handler in `adapters/grpc`, registration in `cmd/` |
| Reaction to another context's event | `adapters/eventconsumer/<topic>.go` calling one command |
| Call to an external provider | Port in `ports/`, adapter in `adapters/clients`, durable intent if correctness matters |
| Periodic work | `adapters/jobs/<job>.go` calling one command |
| A new column | Migration + row mapping + snapshot + round-trip test |

See [New Service](../guides/new-service.md) for a step-by-step bootstrap.
