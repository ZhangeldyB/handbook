package usecases

import (
	"context"
	"errors"
	"fmt"

	"example.com/orders/internal/orders/ports"
	"example.com/orders/internal/platform/deps"
)

const scopePaymentCaptured = "consume:payments.payment_captured.v1"

// MarkPaidDeps are the required collaborators of MarkPaid.
type MarkPaidDeps struct {
	Orders ports.OrderRepository
	Clock  ports.Clock
	IDs    ports.IDGenerator
}

// MarkPaid records a captured payment. It is driven by an at-least-once event
// consumer, so it must be safe to run more than once for the same event.
type MarkPaid struct {
	orders ports.OrderRepository
	clock  ports.Clock
	ids    ports.IDGenerator
}

// NewMarkPaid validates its dependencies once.
func NewMarkPaid(d MarkPaidDeps) (*MarkPaid, error) {
	if err := errors.Join(
		deps.Require("Orders", d.Orders),
		deps.Require("Clock", d.Clock),
		deps.Require("IDs", d.IDs),
	); err != nil {
		return nil, fmt.Errorf("new mark paid: %w", err)
	}
	return &MarkPaid{orders: d.Orders, clock: d.Clock, ids: d.IDs}, nil
}

// MarkPaidRequest carries the consumed event's identity.
type MarkPaidRequest struct {
	TenantID  string
	OrderID   string
	PaymentID string
	// EventID is the consumed event's stable ID, used to deduplicate.
	EventID string
}

// Execute marks the order paid. A redelivered event commits nothing: either
// the order is already paid with this payment (domain no-op) or the
// idempotency record for EventID already exists.
func (c *MarkPaid) Execute(ctx context.Context, req MarkPaidRequest) error {
	if req.EventID == "" {
		return errors.New("mark paid: event ID is required for deduplication")
	}
	order, err := c.orders.Get(ctx, req.TenantID, req.OrderID)
	if err != nil {
		return fmt.Errorf("get order %q: %w", req.OrderID, err)
	}
	if err := order.MarkPaid(c.ids.NewID(), req.PaymentID, c.clock.Now()); err != nil {
		return err
	}
	if !order.IsDirty() {
		return nil // already applied
	}

	err = c.orders.Save(ctx, order, ports.IdempotencyRecord{Scope: scopePaymentCaptured, Key: req.EventID})
	if errors.Is(err, ports.ErrDuplicate) {
		return nil // a concurrent delivery of the same event already committed
	}
	if err != nil {
		return fmt.Errorf("save order %q: %w", req.OrderID, err)
	}
	return nil
}
