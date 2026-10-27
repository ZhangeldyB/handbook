package domain

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

var now = time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)

func eur(t *testing.T, minor int64) Money {
	t.Helper()
	m, err := NewMoney(minor, "EUR")
	if err != nil {
		t.Fatalf("NewMoney() error = %v", err)
	}
	return m
}

// draftWithLine returns a valid draft order with one line; tests change only
// the property under test.
func draftWithLine(t *testing.T) *Order {
	t.Helper()
	order, err := NewOrder(NewOrderParams{TenantID: "tenant-1", OrderID: "order-1", CustomerID: "customer-1", Currency: "EUR", Now: now})
	if err != nil {
		t.Fatalf("NewOrder() error = %v", err)
	}
	line, err := NewLine("sku-1", 2, eur(t, 1250))
	if err != nil {
		t.Fatalf("NewLine() error = %v", err)
	}
	if err := order.AddLine(line); err != nil {
		t.Fatalf("AddLine() error = %v", err)
	}
	return order
}

func reconstituted(t *testing.T, status Status) *Order {
	t.Helper()
	line, _ := NewLine("sku-1", 1, eur(t, 500))
	order, err := Reconstitute(Snapshot{
		TenantID: "tenant-1", OrderID: "order-1", CustomerID: "customer-1", Currency: "EUR",
		Status: status, Lines: []Line{line}, CreatedAt: now, PlacedAt: now, PaymentID: "pay-1", Version: 3,
	})
	if err != nil {
		t.Fatalf("Reconstitute() error = %v", err)
	}
	return order
}

func TestPlaceRecordsEventWithTotal(t *testing.T) {
	order := draftWithLine(t)

	if err := order.Place("event-1", now); err != nil {
		t.Fatalf("Place() error = %v", err)
	}

	if got, want := order.Status(), StatusPlaced; got != want {
		t.Errorf("Status() = %s, want %s", got, want)
	}
	pending, _, ok := order.BeginPersist()
	if !ok || len(pending.Events) != 1 {
		t.Fatalf("pending events = %v, want one OrderPlaced", pending.Events)
	}
	placed, ok := pending.Events[0].(OrderPlaced)
	if !ok {
		t.Fatalf("event type = %T, want OrderPlaced", pending.Events[0])
	}
	if placed.EventID != "event-1" || placed.Total.Minor() != 2500 || !placed.OccurredAt.Equal(now) {
		t.Errorf("event = %+v, want ID event-1, total 2500, time %v", placed, now)
	}
}

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

func TestTransitions(t *testing.T) {
	tests := []struct {
		name    string
		from    Status
		act     func(*Order) error
		want    Status
		wantErr error
	}{
		{name: "placed can be paid", from: StatusPlaced, act: func(o *Order) error { return o.MarkPaid("e", "pay-2", now) }, want: StatusPaid},
		{name: "paid again with same payment is a no-op", from: StatusPaid, act: func(o *Order) error { return o.MarkPaid("e", "pay-1", now) }, want: StatusPaid},
		{name: "paid with another payment is rejected", from: StatusPaid, act: func(o *Order) error { return o.MarkPaid("e", "pay-9", now) }, want: StatusPaid, wantErr: ErrInvalidTransition},
		{name: "draft cannot be paid", from: StatusDraft, act: func(o *Order) error { return o.MarkPaid("e", "pay-2", now) }, want: StatusDraft, wantErr: ErrInvalidTransition},
		{name: "placed can be cancelled", from: StatusPlaced, act: func(o *Order) error { return o.Cancel("e", "changed mind", now) }, want: StatusCancelled},
		{name: "cancelled again is a no-op", from: StatusCancelled, act: func(o *Order) error { return o.Cancel("e", "again", now) }, want: StatusCancelled},
		{name: "paid cannot be cancelled", from: StatusPaid, act: func(o *Order) error { return o.Cancel("e", "too late", now) }, want: StatusPaid, wantErr: ErrInvalidTransition},
		{name: "placed cannot be placed", from: StatusPlaced, act: func(o *Order) error { return o.Place("e", now) }, want: StatusPlaced, wantErr: ErrInvalidTransition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := reconstituted(t, tt.from)

			err := tt.act(order)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if got := order.Status(); got != tt.want {
				t.Errorf("Status() = %s, want %s", got, tt.want)
			}
			if tt.wantErr != nil && order.IsDirty() {
				t.Error("rejected command left pending work")
			}
		})
	}
}

func TestAddLineRejectsOtherCurrency(t *testing.T) {
	order := draftWithLine(t)
	usd, _ := NewMoney(100, "USD")
	line, _ := NewLine("sku-2", 1, usd)

	if err := order.AddLine(line); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("AddLine() error = %v, want %v", err, ErrCurrencyMismatch)
	}
	if got := len(order.Lines()); got != 1 {
		t.Fatalf("len(Lines()) = %d, want 1", got)
	}
}

func TestLinesReturnsCopy(t *testing.T) {
	order := draftWithLine(t)

	lines := order.Lines()
	lines[0] = Line{}

	if order.Lines()[0].SKU() != "sku-1" {
		t.Fatal("mutating Lines() result changed the order")
	}
}

func TestReconstituteRejectsUnknownStatus(t *testing.T) {
	_, err := Reconstitute(Snapshot{TenantID: "t", OrderID: "o", CustomerID: "c", Currency: "EUR", Status: "SHIPPED", Version: 1})

	if !errors.Is(err, ErrInvalidOrder) {
		t.Fatalf("Reconstitute() error = %v, want %v", err, ErrInvalidOrder)
	}
}

func TestReconstituteRecordsNoEvents(t *testing.T) {
	order := reconstituted(t, StatusPlaced)

	if order.IsDirty() || order.PendingEvents() != 0 {
		t.Fatal("reconstituted order has pending work")
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	original := reconstituted(t, StatusPaid)

	copied, err := Reconstitute(original.Snapshot())
	if err != nil {
		t.Fatalf("Reconstitute(Snapshot()) error = %v", err)
	}
	if got, want := copied.Snapshot(), original.Snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}
