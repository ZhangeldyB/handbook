// Package domain holds the orders bounded context's business rules. It performs
// no I/O, takes no context.Context, and receives time and IDs from callers.
package domain

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"example.com/orders/internal/platform/aggregate"
)

// Line is an immutable order line value object.
type Line struct {
	sku       string
	quantity  int64
	unitPrice Money
}

// NewLine validates and returns a line.
func NewLine(sku string, quantity int64, unitPrice Money) (Line, error) {
	if sku == "" || quantity <= 0 || unitPrice.Minor() < 0 || !unitPrice.Currency().Valid() {
		return Line{}, fmt.Errorf("line %q x%d: %w", sku, quantity, ErrInvalidLine)
	}
	if _, err := unitPrice.Times(quantity); err != nil {
		return Line{}, fmt.Errorf("line %q subtotal: %w", sku, err)
	}
	return Line{sku: sku, quantity: quantity, unitPrice: unitPrice}, nil
}

// SKU returns the product identifier.
func (l Line) SKU() string { return l.sku }

// Quantity returns the number of units.
func (l Line) Quantity() int64 { return l.quantity }

// UnitPrice returns the price of one unit.
func (l Line) UnitPrice() Money { return l.unitPrice }

// Subtotal returns quantity times unit price. NewLine guarantees it fits.
func (l Line) Subtotal() Money {
	subtotal, _ := l.unitPrice.Times(l.quantity)
	return subtotal
}

// Order is the consistency boundary for one customer order. All state is
// private; it changes only through methods that enforce the lifecycle.
type Order struct {
	tenantID     string
	id           string
	customerID   string
	currency     Currency
	status       Status
	lines        []Line
	createdAt    time.Time
	placedAt     time.Time
	paymentID    string
	cancelReason string

	tracker aggregate.Tracker[Event]
}

// NewOrderParams are the inputs for creating a draft order. Time and ID come
// from the application layer so behavior is deterministic.
type NewOrderParams struct {
	TenantID   string
	OrderID    string
	CustomerID string
	Currency   Currency
	Now        time.Time
}

// NewOrder returns a new draft order.
func NewOrder(p NewOrderParams) (*Order, error) {
	if p.TenantID == "" || p.OrderID == "" || p.CustomerID == "" || !p.Currency.Valid() || p.Now.IsZero() {
		return nil, fmt.Errorf("new order: tenant, order, customer, currency, and time are required: %w", ErrInvalidOrder)
	}
	return &Order{
		tenantID:   p.TenantID,
		id:         p.OrderID,
		customerID: p.CustomerID,
		currency:   p.Currency,
		status:     StatusDraft,
		createdAt:  p.Now,
		tracker:    aggregate.NewForCreate[Event](),
	}, nil
}

// Snapshot is the complete persisted state of an order. Repositories map it to
// and from storage rows; adding a field here without mapping it fails the
// repository round-trip test.
type Snapshot struct {
	TenantID     string
	OrderID      string
	CustomerID   string
	Currency     Currency
	Status       Status
	Lines        []Line
	CreatedAt    time.Time
	PlacedAt     time.Time
	PaymentID    string
	CancelReason string
	Version      int64
}

// Reconstitute restores an order from persisted state. It records no events and
// rejects structurally impossible state instead of building a half-valid order.
func Reconstitute(s Snapshot) (*Order, error) {
	var errs []error
	if s.TenantID == "" || s.OrderID == "" || s.CustomerID == "" || !s.Currency.Valid() {
		errs = append(errs, errors.New("missing identity or currency"))
	}
	if !s.Status.Valid() {
		errs = append(errs, fmt.Errorf("unknown status %q", s.Status))
	}
	if s.Version <= 0 {
		errs = append(errs, fmt.Errorf("non-positive version %d", s.Version))
	}
	for _, line := range s.Lines {
		if line.UnitPrice().Currency() != s.Currency {
			errs = append(errs, fmt.Errorf("line %q currency %s", line.SKU(), line.UnitPrice().Currency()))
		}
	}
	if s.Status != StatusDraft && s.Status != StatusCancelled && s.PlacedAt.IsZero() {
		errs = append(errs, fmt.Errorf("status %s without placement time", s.Status))
	}
	if err := errors.Join(errs...); err != nil {
		return nil, fmt.Errorf("reconstitute order %q: %w: %w", s.OrderID, ErrInvalidOrder, err)
	}
	return &Order{
		tenantID:     s.TenantID,
		id:           s.OrderID,
		customerID:   s.CustomerID,
		currency:     s.Currency,
		status:       s.Status,
		lines:        slices.Clone(s.Lines),
		createdAt:    s.CreatedAt,
		placedAt:     s.PlacedAt,
		paymentID:    s.PaymentID,
		cancelReason: s.CancelReason,
		tracker:      aggregate.NewForReconstitute[Event](s.Version),
	}, nil
}

// Snapshot returns a copy of the complete state for persistence mapping.
func (o *Order) Snapshot() Snapshot {
	return Snapshot{
		TenantID:     o.tenantID,
		OrderID:      o.id,
		CustomerID:   o.customerID,
		Currency:     o.currency,
		Status:       o.status,
		Lines:        slices.Clone(o.lines),
		CreatedAt:    o.createdAt,
		PlacedAt:     o.placedAt,
		PaymentID:    o.paymentID,
		CancelReason: o.cancelReason,
		Version:      o.tracker.Version(),
	}
}

// TenantID returns the owning tenant.
func (o *Order) TenantID() string { return o.tenantID }

// ID returns the order ID.
func (o *Order) ID() string { return o.id }

// CustomerID returns the ordering customer.
func (o *Order) CustomerID() string { return o.customerID }

// Status returns the current status.
func (o *Order) Status() Status { return o.status }

// Lines returns a copy of the order lines; mutating it does not affect the order.
func (o *Order) Lines() []Line { return slices.Clone(o.lines) }

// Total returns the sum of all line subtotals.
func (o *Order) Total() (Money, error) {
	total := Zero(o.currency)
	for _, line := range o.lines {
		var err error
		if total, err = total.Add(line.Subtotal()); err != nil {
			return Money{}, fmt.Errorf("order %q total: %w", o.id, err)
		}
	}
	return total, nil
}

// AddLine appends a line to a draft order.
func (o *Order) AddLine(line Line) error {
	if o.status != StatusDraft {
		return fmt.Errorf("add line to %s order %q: %w", o.status, o.id, ErrInvalidTransition)
	}
	if line.UnitPrice().Currency() != o.currency {
		return fmt.Errorf("add line %q: %w", line.SKU(), ErrCurrencyMismatch)
	}
	if line.Quantity() <= 0 {
		return fmt.Errorf("add line %q: %w", line.SKU(), ErrInvalidLine)
	}
	if _, err := o.totalWith(line); err != nil {
		return fmt.Errorf("add line %q: %w", line.SKU(), err)
	}
	o.lines = append(o.lines, line)
	o.tracker.Changed()
	return nil
}

// Place submits a draft order. It validates everything before changing state,
// so a rejected call leaves the order and its pending events untouched.
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
		EventID:    eventID,
		TenantID:   o.tenantID,
		OrderID:    o.id,
		CustomerID: o.customerID,
		Total:      total,
		OccurredAt: now,
	})
	return nil
}

// MarkPaid records a confirmed payment. Repeating the call with the same
// payment ID is a no-op, which makes redelivered payment events harmless.
func (o *Order) MarkPaid(eventID, paymentID string, now time.Time) error {
	if paymentID == "" {
		return fmt.Errorf("mark order %q paid: payment ID required: %w", o.id, ErrInvalidOrder)
	}
	if o.status == StatusPaid && o.paymentID == paymentID {
		return nil
	}
	if err := validateTransition(o.status, StatusPaid); err != nil {
		return fmt.Errorf("mark order %q paid: %w", o.id, err)
	}

	o.status = StatusPaid
	o.paymentID = paymentID
	o.tracker.Record(OrderPaid{
		EventID:    eventID,
		TenantID:   o.tenantID,
		OrderID:    o.id,
		PaymentID:  paymentID,
		OccurredAt: now,
	})
	return nil
}

// Cancel cancels a draft or placed order. Cancelling a cancelled order is a
// no-op; a paid order cannot be cancelled (it needs a refund workflow).
func (o *Order) Cancel(eventID, reason string, now time.Time) error {
	if o.status == StatusCancelled {
		return nil
	}
	if err := validateTransition(o.status, StatusCancelled); err != nil {
		return fmt.Errorf("cancel order %q: %w", o.id, err)
	}

	o.status = StatusCancelled
	o.cancelReason = reason
	o.tracker.Record(OrderCancelled{
		EventID:    eventID,
		TenantID:   o.tenantID,
		OrderID:    o.id,
		Reason:     reason,
		OccurredAt: now,
	})
	return nil
}

// BeginPersist captures pending state and events for a repository. See
// aggregate.Tracker.Begin for the acknowledgement contract.
func (o *Order) BeginPersist() (aggregate.Pending[Event], func(), bool) {
	return o.tracker.Begin()
}

// IsDirty reports whether the order has changes that have not been committed.
func (o *Order) IsDirty() bool { return o.tracker.IsDirty() }

// PendingEvents returns the number of events not yet committed.
func (o *Order) PendingEvents() int { return o.tracker.EventCount() }

func (o *Order) totalWith(line Line) (Money, error) {
	total, err := o.Total()
	if err != nil {
		return Money{}, err
	}
	return total.Add(line.Subtotal())
}
