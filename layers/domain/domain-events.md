# Domain Events

A domain event is a stable, past-tense fact produced by a successful transition: `OrderPlaced`, not `PlaceOrder` and not `OrderUpdated`. It is not a log line or a generic change notification.

## Shape

```go
type Event interface{ isOrderEvent() }

type OrderPlaced struct {
	EventID    string
	TenantID   string
	OrderID    string
	CustomerID string
	Total      Money
	OccurredAt time.Time
}

func (OrderPlaced) isOrderEvent() {}
```

Include what consumers need to act on the fact: a stable event ID, tenant, aggregate ID, occurrence time, and the payload. Avoid embedding the aggregate or a database row, credentials or sensitive data, generic maps, and IDs generated during publication (they must be stable across retries).

## Granularity

Prefer events consumers can act on. Avoid both a single `OrderUpdated` that forces consumers to diff snapshots and one event per field assignment that exposes implementation details. Use a snapshot-style event only when merge and version semantics are defined.

## Recording

The transition method validates, mutates, then records — see [Aggregates](aggregates.md#behavior). The event ID and time come from the application layer:

```go
if err := order.Place(c.ids.NewID(), c.clock.Now()); err != nil {
	return err
}
```

A rejected transition records nothing.

## Converting at the Adapter

The domain event is converted to its published contract in the persistence adapter, inside `Save`, before the transaction opens:

```go
func toOutboxRecord(event domain.Event) (outbox.Record, error) {
	switch e := event.(type) {
	case domain.OrderPlaced:
		raw, err := json.Marshal(orderPlacedV1{OrderID: e.OrderID, CustomerID: e.CustomerID,
			TotalMinor: e.Total.Minor(), Currency: string(e.Total.Currency()), OccurredAt: e.OccurredAt})
		if err != nil {
			return outbox.Record{}, fmt.Errorf("encode %s payload: %w", TopicOrderPlaced, err)
		}
		return outbox.Record{ID: e.EventID, TenantID: e.TenantID, Topic: TopicOrderPlaced,
			AggregateID: e.OrderID, Payload: raw, OccurredAt: e.OccurredAt}, nil
	default:
		return outbox.Record{}, fmt.Errorf("convert event %T: unsupported event type", event)
	}
}
```

- An unknown event type is an error, never a silent skip.
- Encoding errors fail the save; the aggregate keeps its pending events.
- The published payload (`orderPlacedV1`) is a separate, versioned type — internal refactors of the domain event do not change the wire contract.

## Topics and Versions

- Topic names are contracts: `<context>.<event>.v<N>` (`orders.order_placed.v1`).
- Compatible changes (adding optional fields) keep the version. Removing, renaming, or changing the meaning or type of a field is a new version, published alongside the old one until consumers migrate.
- Consumers tolerate unknown fields and unknown enum values.
- If payloads are protobuf, follow the [proto rules](../../.agents/rules/proto-rules.md): never reuse field numbers, reserve removed ones.

## Delivery

Events reach consumers through the [transactional outbox](../../patterns/outbox.md): at least once, possibly out of order across keys, possibly duplicated. Every consumer deduplicates on `EventID` ([Event Consumer](../../patterns/event-consumer.md)). Do not promise ordering unless the whole chain enforces it per key.

## Testing

- A valid transition records exactly one event with the expected contents.
- An invalid transition records nothing.
- The converter produces the documented payload for every event type, and rejects unknown types.
- A failed save retains the event; the retry writes it to the outbox exactly once.
- Payload fixtures from older versions still decode in consumers.

## Review Checklist

- [ ] Past-tense fact with stable ID, tenant, aggregate ID, and time.
- [ ] Recorded in the same method that changes the state.
- [ ] Converted in the adapter with checked errors; unknown types rejected.
- [ ] Versioned topic and payload; compatible evolution reviewed with consumers.
- [ ] No sensitive data in payloads.
