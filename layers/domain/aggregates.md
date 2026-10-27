# Aggregates

An aggregate is a consistency boundary that owns business invariants and state transitions. Do not create one for every table or CRUD shape — create one when several pieces of state must change together under a rule.

## Shape

```go
type Order struct {
	tenantID   string
	id         string
	customerID string
	currency   Currency
	status     Status
	lines      []Line
	createdAt  time.Time
	placedAt   time.Time
	paymentID  string

	tracker aggregate.Tracker[Event]
}
```

- All fields are unexported; state changes only through methods.
- The [`aggregate.Tracker`](../../examples/orders/internal/platform/aggregate/tracker.go) records whether the aggregate is new, whether it has uncommitted changes, its committed version, and its pending events. It is an unexported field so only the aggregate's own methods can record events.
- Events are a closed set (`type Event interface{ isOrderEvent() }`).

Copy `internal/platform/aggregate` into your service from the example; it has no dependencies.

## Construction

Two constructors initialize different lifecycles:

```go
// NewOrder creates a draft. Time and ID are supplied by the application.
func NewOrder(p NewOrderParams) (*Order, error) {
	if p.TenantID == "" || p.OrderID == "" || p.CustomerID == "" || !p.Currency.Valid() || p.Now.IsZero() {
		return nil, fmt.Errorf("new order: tenant, order, customer, currency, and time are required: %w", ErrInvalidOrder)
	}
	return &Order{
		tenantID: p.TenantID, id: p.OrderID, customerID: p.CustomerID,
		currency: p.Currency, status: StatusDraft, createdAt: p.Now,
		tracker: aggregate.NewForCreate[Event](),
	}, nil
}

// Reconstitute restores persisted state. It records nothing and replays nothing.
func Reconstitute(s Snapshot) (*Order, error) {
	// validate identity, status, version, line currencies, ...
	return &Order{ /* every field from s */ tracker: aggregate.NewForReconstitute[Event](s.Version)}, nil
}
```

- `NewForCreate` starts new and dirty so the first save inserts. `NewForReconstitute(version)` starts clean at the stored version. The zero tracker panics — an uninitialized aggregate is a programmer error.
- A create factory that must announce creation records its event before returning, so no caller can forget it.
- `Reconstitute` rejects structurally impossible state (unknown status, missing identity) instead of building a half-valid aggregate.

## Snapshot: One Mapping Surface

```go
type Snapshot struct {
	TenantID, OrderID, CustomerID string
	Currency                      Currency
	Status                        Status
	Lines                         []Line
	CreatedAt, PlacedAt           time.Time
	PaymentID, CancelReason       string
	Version                       int64
}

func (o *Order) Snapshot() Snapshot // complete copy of state
```

The repository maps `Snapshot` ↔ row in exactly two functions, and a round-trip test whose fixture sets *every* field (checked by reflection) fails when someone adds a field without mapping it. That replaces scattered getters as the persistence contract.

## Behavior

```go
func (o *Order) MarkPaid(eventID, paymentID string, now time.Time) error {
	if paymentID == "" {
		return fmt.Errorf("mark order %q paid: payment ID required: %w", o.id, ErrInvalidOrder)
	}
	if o.status == StatusPaid && o.paymentID == paymentID {
		return nil // same payment again: idempotent no-op
	}
	if err := validateTransition(o.status, StatusPaid); err != nil {
		return fmt.Errorf("mark order %q paid: %w", o.id, err)
	}

	o.status = StatusPaid
	o.paymentID = paymentID
	o.tracker.Record(OrderPaid{EventID: eventID, TenantID: o.tenantID, OrderID: o.id, PaymentID: paymentID, OccurredAt: now})
	return nil
}
```

1. **Validate everything first.** On error, state and pending events are unchanged.
2. **Mutate**, then **record** the event (or call `tracker.Changed()` when no event is needed) in the same method. One method owns each transition, so no entry point can implement half of it.
3. **Decide same-state behavior per operation.** Here a repeated payment is a no-op (it makes redelivered events harmless); placing an already-placed order is an error.

Keep legal transitions in one table — see [State Machine template](../../templates/state-machine.md):

```go
var allowedTransitions = map[Status][]Status{
	StatusDraft:  {StatusPlaced, StatusCancelled},
	StatusPlaced: {StatusPaid, StatusCancelled},
}
```

## Persistence Surface

The aggregate exposes exactly what the repository needs, and nothing that lets other layers record events:

```go
func (o *Order) BeginPersist() (aggregate.Pending[Event], func(), bool) { return o.tracker.Begin() }
func (o *Order) IsDirty() bool                                         { return o.tracker.IsDirty() }
func (o *Order) PendingEvents() int                                    { return o.tracker.EventCount() }
```

The repository writes the captured state and events in one transaction and calls the returned `ack` only after commit ([ADR-004](../../decisions/004-acknowledge-after-commit.md)). A failed commit leaves everything pending for a retry.

## Boundaries Between Aggregates

- Reference other aggregates by ID; never hold a pointer to another aggregate.
- Never nest a mutable sub-aggregate. A line item without its own identity or lifecycle is a value object inside the order; a shipment with its own ID, status, and repository is a separate aggregate.
- A rule across aggregates goes in a pure [domain service](domain-services.md) fed by the application layer; one in-memory calculation does not make several aggregates atomic.
- Keep aggregates small. A large aggregate means broad loads, contention, and version conflicts.

## Value Objects and Collections

- Value objects (`Line`, `Money`) validate in their constructor and are immutable; they compare by value.
- Getters returning slices or maps return copies (`slices.Clone`); constructors copy incoming slices. An aliased collection lets callers bypass invariants.
- Domain enums are domain-owned string types whose zero value is invalid, with a `Parse` function that rejects unknown values. Map them explicitly to protobuf and storage in adapters.

## Testing

- Every valid and invalid transition (table-driven), including same-state behavior.
- State and pending-event count unchanged after a rejected command.
- Create versus reconstitute: new and dirty versus clean; reconstitution records no events.
- Recorded event contents (ID, tenant, time, payload).
- Returned collections cannot mutate the aggregate.
- `Reconstitute(o.Snapshot())` round trip.

See [order_test.go](../../examples/orders/internal/orders/domain/order_test.go).

## Review Checklist

- [ ] The aggregate owns a real invariant.
- [ ] State is private; returned collections are copies.
- [ ] No `context.Context`, I/O, logging, or infrastructure types.
- [ ] Time and IDs are parameters.
- [ ] Each method validates before mutating and records its event in the same method.
- [ ] Transitions live in one table.
- [ ] `Reconstitute` maps every field, records nothing, and rejects impossible state.
- [ ] No destructive event-draining method exists.
