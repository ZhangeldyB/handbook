package domain

import "time"

// Event is the closed set of facts an Order records. Events are immutable
// values; their IDs are assigned by the application layer so that retries
// reuse them.
type Event interface {
	isOrderEvent()
}

// OrderPlaced is recorded when a draft order is submitted.
type OrderPlaced struct {
	EventID    string
	TenantID   string
	OrderID    string
	CustomerID string
	Total      Money
	OccurredAt time.Time
}

// OrderPaid is recorded when payment for a placed order is confirmed.
type OrderPaid struct {
	EventID    string
	TenantID   string
	OrderID    string
	PaymentID  string
	OccurredAt time.Time
}

// OrderCancelled is recorded when an order is cancelled.
type OrderCancelled struct {
	EventID    string
	TenantID   string
	OrderID    string
	Reason     string
	OccurredAt time.Time
}

func (OrderPlaced) isOrderEvent()    {}
func (OrderPaid) isOrderEvent()      {}
func (OrderCancelled) isOrderEvent() {}
