package domain

import "errors"

// Stable domain errors. Callers classify them with errors.Is; the transport
// layer maps them to public status codes.
var (
	// ErrInvalidTransition means the requested change is not allowed from the
	// order's current status.
	ErrInvalidTransition = errors.New("invalid order status transition")
	// ErrEmptyOrder means an order without lines cannot be placed.
	ErrEmptyOrder = errors.New("order has no lines")
	// ErrInvalidLine means a line item violates its invariants.
	ErrInvalidLine = errors.New("invalid order line")
	// ErrInvalidOrder means creation input or persisted state is structurally
	// invalid.
	ErrInvalidOrder = errors.New("invalid order")
)
