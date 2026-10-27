package persistence

import (
	"encoding/json"
	"fmt"
	"time"

	"example.com/orders/internal/orders/domain"
	"example.com/orders/internal/platform/outbox"
)

// Published topics. A topic name is a contract: an incompatible payload change
// gets a new version suffix instead of changing the existing one.
const (
	TopicOrderPlaced    = "orders.order_placed.v1"
	TopicOrderPaid      = "orders.order_paid.v1"
	TopicOrderCancelled = "orders.order_cancelled.v1"
)

type orderPlacedV1 struct {
	OrderID    string    `json:"order_id"`
	CustomerID string    `json:"customer_id"`
	TotalMinor int64     `json:"total_minor"`
	Currency   string    `json:"currency"`
	OccurredAt time.Time `json:"occurred_at"`
}

type orderPaidV1 struct {
	OrderID    string    `json:"order_id"`
	PaymentID  string    `json:"payment_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

type orderCancelledV1 struct {
	OrderID    string    `json:"order_id"`
	Reason     string    `json:"reason"`
	OccurredAt time.Time `json:"occurred_at"`
}

// toOutboxRecord converts a domain event into its published contract. An
// unknown event type is an error, never a silent skip.
func toOutboxRecord(event domain.Event) (outbox.Record, error) {
	var (
		topic                          string
		payload                        any
		eventID, tenantID, aggregateID string
		occurredAt                     time.Time
	)
	switch e := event.(type) {
	case domain.OrderPlaced:
		topic, eventID, tenantID, aggregateID, occurredAt = TopicOrderPlaced, e.EventID, e.TenantID, e.OrderID, e.OccurredAt
		payload = orderPlacedV1{
			OrderID: e.OrderID, CustomerID: e.CustomerID,
			TotalMinor: e.Total.Minor(), Currency: string(e.Total.Currency()), OccurredAt: e.OccurredAt,
		}
	case domain.OrderPaid:
		topic, eventID, tenantID, aggregateID, occurredAt = TopicOrderPaid, e.EventID, e.TenantID, e.OrderID, e.OccurredAt
		payload = orderPaidV1{OrderID: e.OrderID, PaymentID: e.PaymentID, OccurredAt: e.OccurredAt}
	case domain.OrderCancelled:
		topic, eventID, tenantID, aggregateID, occurredAt = TopicOrderCancelled, e.EventID, e.TenantID, e.OrderID, e.OccurredAt
		payload = orderCancelledV1{OrderID: e.OrderID, Reason: e.Reason, OccurredAt: e.OccurredAt}
	default:
		return outbox.Record{}, fmt.Errorf("convert event %T: unsupported event type", event)
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return outbox.Record{}, fmt.Errorf("encode %s payload: %w", topic, err)
	}
	return outbox.Record{
		ID:          eventID,
		TenantID:    tenantID,
		Topic:       topic,
		AggregateID: aggregateID,
		Payload:     raw,
		OccurredAt:  occurredAt,
	}, nil
}
