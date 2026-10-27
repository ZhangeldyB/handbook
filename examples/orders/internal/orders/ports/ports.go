// Package ports declares the capabilities the orders application layer
// consumes. Adapters implement them; the composition root wires them.
package ports

import (
	"context"
	"errors"
	"time"

	"example.com/orders/internal/orders/domain"
)

// Stable persistence errors. Adapters translate storage-specific errors into
// these so application code never imports an adapter package.
var (
	// ErrNotFound means no order exists for the tenant and ID.
	ErrNotFound = errors.New("order not found")
	// ErrConcurrentModification means the order changed after it was loaded.
	// The caller may reload, re-apply the command, and save again.
	ErrConcurrentModification = errors.New("order was modified concurrently")
	// ErrDuplicate means an IdempotencyRecord with the same scope and key was
	// already committed, so the whole save was rolled back.
	ErrDuplicate = errors.New("duplicate idempotency key")
)

// Clock supplies the time used for business decisions.
type Clock interface {
	Now() time.Time
}

// IDGenerator returns new unique identifiers.
type IDGenerator interface {
	NewID() string
}

// IdempotencyRecord marks a unit of work that must commit at most once. Saving
// it inserts (never upserts) the record in the same transaction as the
// aggregate, so a concurrent duplicate loses the whole commit with
// ErrDuplicate.
type IdempotencyRecord struct {
	// Scope names the operation or consumer, e.g. "CreateDraft".
	Scope string
	// Key is the client idempotency key or the consumed event ID.
	Key string
	// Fingerprint identifies the request payload; reusing a key with a
	// different payload is a client error.
	Fingerprint string
	// Response is the encoded original result returned on replay.
	Response string
}

// OrderRepository loads and atomically persists Order aggregates.
type OrderRepository interface {
	// Get returns ErrNotFound when the order does not exist in the tenant.
	Get(ctx context.Context, tenantID, orderID string) (*domain.Order, error)
	// Save writes the order's pending state, its pending events (to the
	// outbox), and any idempotency records in one transaction. On failure the
	// order keeps its pending work so the call can be retried. It returns
	// ErrConcurrentModification on a version conflict and ErrDuplicate when a
	// record already exists.
	Save(ctx context.Context, order *domain.Order, records ...IdempotencyRecord) error
}

// IdempotencyStore looks up committed idempotency records.
type IdempotencyStore interface {
	// Lookup reports whether a record exists for tenant, scope, and key.
	Lookup(ctx context.Context, tenantID, scope, key string) (IdempotencyRecord, bool, error)
}
