# Quick Reference

Defaults for services with real business invariants. Repository evidence and accepted ADRs come first.

## Change Workflow

```text
map callers, invariants, I/O, contracts, failure paths
  → choose the smallest complete design
  → implement at the owning boundary
  → verify success, failure/retry, and wiring
  → read the complete diff
```

## Layout

```text
cmd/server/main.go                      composition root
internal/<ctx>/domain                   aggregates, events, errors (pure)
internal/<ctx>/ports                    interfaces + port errors
internal/<ctx>/usecases/<op>.go         commands
internal/<ctx>/queries                  reads + reader port
internal/<ctx>/adapters/{grpc,persistence,eventconsumer,clients,jobs}
internal/platform/{aggregate,deps,auth,grpcerr,outbox,messaging,...}
proto/ gen/ migrations/
```

## Domain

```go
func (o *Order) Place(eventID string, now time.Time) error {
	if err := validateTransition(o.status, StatusPlaced); err != nil {
		return fmt.Errorf("place order %q: %w", o.id, err)
	}
	// ...validate everything first...
	o.status = StatusPlaced
	o.tracker.Record(OrderPlaced{EventID: eventID, OrderID: o.id, OccurredAt: now /* ... */})
	return nil
}
```

Private state · no `ctx` · time and IDs as parameters · validate then mutate then record · `NewForCreate` / `NewForReconstitute(version)` · `Snapshot()` for mapping.

## Usecase

```go
func (c *Place) Execute(ctx context.Context, req PlaceRequest) error {
	order, err := c.orders.Get(ctx, req.TenantID, req.OrderID)
	if err != nil {
		return fmt.Errorf("get order %q: %w", req.OrderID, err)
	}
	if err := order.Place(c.ids.NewID(), c.clock.Now()); err != nil {
		return err
	}
	if err := c.orders.Save(ctx, order); err != nil {
		return fmt.Errorf("save order %q: %w", req.OrderID, err)
	}
	return nil
}
```

`<Op>Deps` validated once with `deps.Require` · tenant from the principal · no transactions · idempotency record for retried creates and consumers.

## Repository

`Get` · `Save(ctx, agg, records...)` · `SaveAll` for atomic batches.
`BeginPersist` → map outside the tx → one tx: version-checked state + outbox + idempotency inserts → commit → `ack()`.
Errors: `ErrNotFound`, `ErrConcurrentModification` (→ `Aborted`), `ErrDuplicate`.

## Handler

```go
principal, ok := auth.FromContext(ctx)
if !ok {
	return nil, status.Error(codes.Unauthenticated, "authentication required")
}
if err := s.place.Execute(ctx, usecases.PlaceRequest{TenantID: principal.TenantID, OrderID: req.GetOrderId()}); err != nil {
	return nil, err // grpcerr interceptor maps it
}
```

## Error Codes

| Condition | Code |
|---|---|
| Invalid fields | `InvalidArgument` + `BadRequest` |
| Not found | `NotFound` |
| Version conflict | `Aborted` |
| Invalid transition / precondition | `FailedPrecondition` |
| Duplicate on create | `AlreadyExists` |
| Dependency outage | `Unavailable` |
| Unmapped | `Internal` (sanitized, logged once) |

Full table: [Domain Errors](../layers/domain/domain-errors.md#transport-mapping).

## Events and External Effects

```text
state + outbox (one tx) → relay (lease, backoff, dead-letter) → broker
consumer: decode → one command with event ID → Save with inbox record → ack
provider: intent (one tx) → worker → call with idempotency key → success | failure | UNKNOWN → reconcile
```

## Context and Concurrency

`ctx` first on I/O, never stored, never in the domain · `errgroup` + `SetLimit` for fan-out · every goroutine owned and awaited · capped jittered backoff · no sleeps in tests.

## Errors

Sentinels only for conditions callers branch on · wrap with `%w` and context once · lowercase, no punctuation · `errors.Join` for independent failures · log once at the boundary · classify with `errors.Is`/`As`.

## Testing

| Level | Proves |
|---|---|
| Domain unit | Transitions, events, rejected-state immutability |
| Usecase unit | Orchestration at ports, idempotency, failures |
| Repository integration | Mapping, version checks, outbox atomicity, constraints |
| Handler | Identity, validation, mapping, codes |
| E2E | Critical public workflows |

## Commands

```bash
make lint                       # golangci-lint (+ formatters)
go test ./...
go test -race ./internal/...    # concurrent paths
buf lint && buf breaking --against '.git#branch=main'
govulncheck ./...
```
