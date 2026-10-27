# Infrastructure Layer

Adapters implement inward-facing ports and own protocols, storage mapping, resources, and runtime integration.

| Topic | Document |
|---|---|
| Repositories, transactions, outbox writes | [repositories.md](repositories.md) |
| Schema migrations | [migrations.md](migrations.md) |
| Inbound gRPC: handlers, interceptors, validation, errors | [grpc-handlers.md](grpc-handlers.md) |
| Outbound gRPC and HTTP clients | [grpc-clients.md](grpc-clients.md) |
| Protobuf contracts and buf | [proto-contracts.md](proto-contracts.md) |
| Composition root and dependency injection | [wiring.md](wiring.md) |
| Readiness and graceful shutdown | [graceful-shutdown.md](graceful-shutdown.md) |

## Review Checklist

- [ ] The adapter implements a consumer-owned port and leaks no SDK, row, or wire types inward.
- [ ] Tenant, idempotency, and transaction scope are explicit.
- [ ] Mapping is complete and covered by round-trip tests.
- [ ] Retries match the effect's idempotency; ambiguous outcomes are kept as *unknown*.
- [ ] Resource and goroutine lifecycles have an owner and are tested.
- [ ] Wiring passes focused dependencies validated once.
- [ ] Errors are translated and sanitized; logs and metrics hold no secrets or unbounded labels.
- [ ] Integration tests cover the real infrastructure behavior most likely to break correctness.
