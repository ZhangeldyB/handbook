# Unit Testing

## Domain Tests

No infrastructure, no fakes, fixed times:

```go
var now = time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)

func TestPlaceRejectsEmptyOrderWithoutChangingState(t *testing.T) {
	order, _ := NewOrder(NewOrderParams{TenantID: "tenant-1", OrderID: "order-1", CustomerID: "customer-1", Currency: "EUR", Now: now})

	err := order.Place("event-1", now)

	if !errors.Is(err, ErrEmptyOrder) {
		t.Fatalf("Place() error = %v, want %v", err, ErrEmptyOrder)
	}
	if order.Status() != StatusDraft || order.PendingEvents() != 0 {
		t.Fatalf("status=%s events=%d, want unchanged draft", order.Status(), order.PendingEvents())
	}
}
```

Cover every valid and invalid transition (a table with behavior in the rows works well), state immutability after rejection, recorded event contents, create versus reconstitute, alias safety of returned collections, and the snapshot round trip. See [order_test.go](../examples/orders/internal/orders/domain/order_test.go).

## Table Rows May Carry Behavior

When cases differ by action rather than data, put a function in the row and name every case:

```go
tests := []struct {
	name    string
	from    Status
	act     func(*Order) error
	want    Status
	wantErr error
}{
	{name: "placed can be paid", from: StatusPlaced, act: func(o *Order) error { return o.MarkPaid("e", "pay-2", now) }, want: StatusPaid},
	{name: "paid cannot be cancelled", from: StatusPaid, act: func(o *Order) error { return o.Cancel("e", "too late", now) }, want: StatusPaid, wantErr: ErrInvalidTransition},
}
```

Unnamed or index-named cases are rejected in review; the name is the first line of a failure.

## Object Mothers

Builders return an aggregate in a known-good state; each test changes only the property under test:

```go
// draftWithLine returns a valid draft order with one line; tests change only
// the property under test.
func draftWithLine(t *testing.T) *Order {
	t.Helper()
	// ...
}
```

Name mothers by the state they guarantee (`draftWithLine`, `placedOrder`). Never share mutable package-level fixtures.

## Usecase Tests With Fakes

Fake the ports, use the real domain:

```go
// fakeStore is a small in-memory implementation of the repository and
// idempotency ports. It commits all-or-nothing like the real adapter.
type fakeStore struct {
	orders  map[string]*domain.Order
	records map[string]ports.IdempotencyRecord
	saveErr error
	saves   int
}
```

- A fake must behave like the contract: all-or-nothing writes, `ErrDuplicate` on an existing key, acknowledging pending work only on success. A fake that partially mutates before failing hides real bugs.
- Record effects on the fake (`saves`, captured records) and assert on outcomes.
- To simulate races deterministically, wrap the fake and inject the competing write at the exact point ([`racingStore`](../examples/orders/internal/orders/usecases/usecases_test.go)).
- Use a strict mock only when the interaction itself is the contract (a lease protocol, a call that must not happen after an authentication failure).
- Prefer a hand-written fake that embeds the port interface when you only need a couple of methods — calling an unimplemented method panics, which is a free "nothing else was touched" assertion.

Cover: constructor rejects each missing dependency; success; domain rejection; not-found; save failure with pending work retained; idempotent replay; key reuse; race-loser recovery; duplicate events; cancellation.

## Deterministic Time and IDs

Inject the smallest dependency — a `Clock` port or a `func() time.Time` field — and use fixed values. Generate IDs from a sequential fake. Never sleep to advance time; for timer-driven concurrent code, use `testing/synctest`.

## Assert Errors by Identity

- Sentinels: `errors.Is(err, domain.ErrInvalidTransition)`
- Typed errors: `errors.As` (or `errors.AsType`) plus field assertions
- Transport: `status.Code(err) == codes.NotFound`; validation details by (field, reason)

Never `strings.Contains(err.Error(), ...)` — messages are prose, not contracts.

## Helpers

Helpers that call `t.Fatal`/`t.Error` start with `t.Helper()`. Setup helpers may fail fast; validation helpers return values so the test owns the assertion. Use `t.Cleanup`, `t.Context()`, `t.TempDir()`, `t.Setenv()`. Failure messages use `got, want` order.

## Explicitly Rejected

- Asserting the full sequence of fake calls ("expected actions") — it tests implementation, not behavior.
- A general-purpose in-memory fake of the database for persistence tests — use the real database in integration tests; fake the *port* for usecase tests.
- Mock generators producing interfaces nobody else uses.
