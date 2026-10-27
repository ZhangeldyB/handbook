# Request Flow

One command, `PlaceOrder`, traced from the wire to the broker. Every snippet is taken from the compiled example in [`examples/orders`](../examples/orders/README.md).

```text
gRPC PlaceOrder
  → interceptors: recovery → auth (Principal in ctx) → error mapping
  → handler: validate wire input, read Principal, map request
  → usecases.Place.Execute
       → repo.Get (reconstitute at version N)
       → order.Place(eventID, now)          domain: validate → mutate → record OrderPlaced
       → repo.Save(order)
            → BeginPersist: capture state + events
            → one transaction:
                 compare-and-set row at version N → N+1
                 append OrderPlaced to outbox
            → commit → ack (pending work cleared)
  → handler returns PlaceOrderResponse
...later...
outbox relay: claim with lease → publish → mark published
consumers: deduplicate on event ID inside their own transaction
```

## Responsibilities

| Layer | Owns | Does not own |
|---|---|---|
| Interceptors | Panic recovery, authentication, error-to-status mapping, telemetry | Business decisions |
| Handler | Wire validation, principal → request mapping, response mapping | Transitions, storage, retries |
| Usecase | Authorization, loading, orchestration, time and ID generation | Wire types, rows, transactions |
| Aggregate | Invariants, state transitions, recorded events | Context, I/O, persistence mechanics |
| Repository | Row mapping, version check, outbox and idempotency writes, one transaction | Business policy |
| Outbox relay | Claiming, publishing, retry/backoff, dead-lettering | Business state |

## Domain

```go
func (o *Order) Place(eventID string, now time.Time) error {
	if err := validateTransition(o.status, StatusPlaced); err != nil {
		return fmt.Errorf("place order %q: %w", o.id, err)
	}
	if len(o.lines) == 0 {
		return fmt.Errorf("place order %q: %w", o.id, ErrEmptyOrder)
	}
	total, err := o.Total()
	if err != nil {
		return err
	}

	o.status = StatusPlaced
	o.placedAt = now
	o.tracker.Record(OrderPlaced{
		EventID: eventID, TenantID: o.tenantID, OrderID: o.id,
		CustomerID: o.customerID, Total: total, OccurredAt: now,
	})
	return nil
}
```

Everything is validated before the first assignment, so a rejected call leaves state and pending events untouched. The event ID and time are parameters: retries and tests are deterministic.

## Application

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

`req.TenantID` was set by the handler from the authenticated principal. The usecase never opens a transaction or touches the outbox.

## Repository

```go
func (r *Repository) Save(ctx context.Context, order *domain.Order, records ...ports.IdempotencyRecord) error {
	pending, ack, dirty := order.BeginPersist()
	if !dirty && len(records) == 0 {
		return nil
	}
	// ...map row and outbox records before opening the transaction...
	err := r.db.InTx(ctx, func(tx *memstore.Tx) error {
		if dirty {
			if err := writeOrder(tx, row, pending.IsNew, pending.ExpectedVersion); err != nil {
				return err
			}
			for _, rec := range outboxRecords {
				if err := outbox.Append(tx, rec); err != nil {
					return err
				}
			}
		}
		for _, rec := range records {
			if err := insertIdempotency(tx, order.TenantID(), rec); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err // pending work is retained for a retry
	}
	if dirty {
		ack()
	}
	return nil
}
```

The in-memory store stands in for any transactional database; the contract is identical for SQL or a document store. See [Repositories](../layers/infrastructure/repositories.md).

## Failure Paths

| Failure | Result | Recovery |
|---|---|---|
| Domain rejects the command | Error returned; no state or event change | Caller corrects the request (`FailedPrecondition`/`InvalidArgument`) |
| Mapping or event conversion fails | Nothing written; pending work retained | Bug — fix the mapper; the test suite should have caught it |
| Version conflict | Transaction rolls back; `ErrConcurrentModification` | Caller reloads and retries (`Aborted`) |
| Commit fails (outage) | Transaction rolls back; pending work retained | Retry the same aggregate or the whole command |
| Commit outcome unknown (timeout) | Maybe committed | Never a double apply: re-saving the same aggregate fails the version check, a replayed create hits its idempotency record and returns the original result, and reloading shows whether it applied (idempotent transitions such as `MarkPaid` simply become no-ops) |
| Process dies after commit, before publish | Outbox row is durable | Relay publishes it on its next cycle |
| Publish fails | Row stays pending with backoff | Relay retries; permanent failures dead-letter |
| Publish times out but the broker accepted it | Duplicate delivery later | Consumers deduplicate on event ID |
| Consumer crashes after its commit, before ack | Redelivery | The consumer's idempotency record turns it into a no-op |

## What This Flow Deliberately Avoids

- Publishing to the broker inside or before the transaction.
- Clearing events before the commit (`DrainEvents` patterns).
- Calling a provider from the aggregate or inside the transaction callback.
- Letting the handler or usecase see rows, transactions, or the outbox.
- Trusting a tenant ID from the request body.
