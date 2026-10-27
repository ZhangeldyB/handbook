# Orders Example

A complete bounded context built exactly as the handbook prescribes, using only the standard library.

```bash
go test ./...
go run ./cmd/demo
```

The demo creates a draft (and retries the create with the same idempotency key, getting the same order back), places it, relays `OrderPlaced` through the outbox to a fake payments service, which publishes `PaymentCaptured` twice, and shows the order marked paid exactly once.

## Map

| Path | Shows | Docs |
|---|---|---|
| `internal/platform/aggregate` | Pending-work tracker: acknowledge only after commit | [ADR-004](../../decisions/004-acknowledge-after-commit.md) |
| `internal/platform/deps` | Constructor dependency validation (catches typed nils) | [Wiring](../../layers/infrastructure/wiring.md) |
| `internal/platform/memstore` | Transactional in-memory store with commit-failure injection | [Examples](../README.md#why-an-in-memory-store) |
| `internal/platform/messaging` | Message envelope, publisher/handler contracts, `ErrPermanent`, in-process router | [Event Consumer](../../patterns/event-consumer.md) |
| `internal/platform/outbox` | Outbox record and relay: leases with claim tokens, capped jittered backoff, dead-lettering | [Outbox](../../patterns/outbox.md) |
| `internal/orders/domain` | `Order` aggregate, `Money`, `Line`, status table, events, errors | [Aggregates](../../layers/domain/aggregates.md) |
| `internal/orders/ports` | Repository, idempotency, clock, and ID ports with port errors | [Ports](../../layers/application/ports.md) |
| `internal/orders/usecases` | `CreateDraft` (idempotency key), `Place`, `MarkPaid` (event-driven) | [Usecases](../../layers/application/usecases.md) |
| `internal/orders/queries` | `Get`, `List` with tenant-bound keyset page tokens | [Queries](../../layers/application/queries.md) |
| `internal/orders/adapters/persistence` | `Get`/`Save`/`SaveAll`, row mapping, event conversion, reader | [Repositories](../../layers/infrastructure/repositories.md) |
| `internal/orders/adapters/eventconsumer` | `PaymentCaptured` handler with outcome classification | [Event Consumer](../../patterns/event-consumer.md) |
| `cmd/demo` | Composition root | [Wiring](../../layers/infrastructure/wiring.md) |

## Tests Worth Reading

- `adapters/persistence/repository_test.go` — reflection-checked mapping completeness, failed commit retains work and the retry writes the outbox once, stale writer conflict, duplicate key rollback, tenant isolation, `SaveAll` atomicity.
- `platform/outbox/relay_test.go` — publish, backoff, dead-letter, expired lease with the stale outcome ignored.
- `usecases/usecases_test.go` — idempotent replay, key reuse, deterministic race-loser recovery, duplicate event delivery.
- `domain/order_test.go` — table-driven transitions with behavior in the rows.

## Not Modeled

Real database transactions and isolation levels, a real broker, gRPC transport (see [grpctransport](../grpctransport/README.md)), authentication, telemetry, and graceful shutdown of long-running loops. The [composition-root template](../../templates/composition-root.md) shows the production shape.
