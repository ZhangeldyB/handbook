# Layers

Documentation organized by architectural layer. Dependencies point inward: infrastructure → application → domain.

| Layer | Documents |
|---|---|
| [Domain](domain/README.md) | [Aggregates](domain/aggregates.md) · [Domain Events](domain/domain-events.md) · [Domain Services](domain/domain-services.md) · [Policies](domain/policies.md) · [Domain Errors](domain/domain-errors.md) |
| [Application](application/README.md) | [Usecases](application/usecases.md) · [Queries](application/queries.md) · [Ports](application/ports.md) |
| [Infrastructure](infrastructure/README.md) | [Repositories](infrastructure/repositories.md) · [Migrations](infrastructure/migrations.md) · [gRPC Handlers](infrastructure/grpc-handlers.md) · [gRPC Clients](infrastructure/grpc-clients.md) · [Protobuf Contracts](infrastructure/proto-contracts.md) · [Wiring](infrastructure/wiring.md) · [Graceful Shutdown](infrastructure/graceful-shutdown.md) |
| [Shared concerns](shared/README.md) | [Context and Concurrency](shared/context-concurrency.md) · [Configuration](shared/configuration.md) · [Observability](shared/observability.md) · [Money](shared/money.md) |

## Allowed Dependencies

| From | May import |
|---|---|
| `domain` | other domain packages, pure `platform` primitives |
| `ports`, `usecases`, `queries` | `domain`, `ports`, `platform/deps` |
| `adapters/*` | `domain`, `ports`, `queries`, `usecases`, `platform/*`, drivers, SDKs, generated code |
| `platform/*` | stdlib, drivers, SDKs — never a bounded context |
| `cmd/*` | everything |

Inner layers never import outer ones. The [lint config](../templates/golangci.yml) enforces the domain and application rows with `depguard`.
