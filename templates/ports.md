# Ports Template

Working version: [ports.go](../examples/orders/internal/orders/ports/ports.go).

```go
// Package ports declares the capabilities the <context> application layer
// consumes. Adapters implement them; the composition root wires them.
package ports

import (
	"context"
	"errors"
	"time"

	"<module>/internal/<context>/domain"
)

// Stable persistence errors. Adapters translate storage errors into these.
var (
	ErrNotFound               = errors.New("<entity> not found")
	ErrConcurrentModification = errors.New("<entity> was modified concurrently")
	ErrDuplicate              = errors.New("duplicate idempotency key")
)

type Clock interface{ Now() time.Time }

type IDGenerator interface{ NewID() string }

// IdempotencyRecord marks a unit of work that must commit at most once. It is
// inserted (never upserted) in the same transaction as the aggregate.
type IdempotencyRecord struct {
	Scope       string // operation name or "consume:<topic>"
	Key         string // client key or event ID
	Fingerprint string // request hash; mismatch on replay is a client error
	Response    string // encoded original result
}

// <Entity>Repository loads and atomically persists <Entity> aggregates.
type <Entity>Repository interface {
	// Get returns ErrNotFound when the <entity> does not exist in the tenant.
	Get(ctx context.Context, tenantID, id string) (*domain.<Entity>, error)
	// Save writes pending state, pending events (outbox), and idempotency
	// records in one transaction. On failure the aggregate keeps its pending
	// work. Returns ErrConcurrentModification or ErrDuplicate.
	Save(ctx context.Context, e *domain.<Entity>, records ...IdempotencyRecord) error
}

// IdempotencyStore looks up committed idempotency records.
type IdempotencyStore interface {
	Lookup(ctx context.Context, tenantID, scope, key string) (IdempotencyRecord, bool, error)
}

// <Capability> is an external capability, named by what the consumer needs.
// Document idempotency and which errors mean "outcome unknown".
type <Capability> interface {
	<Verb>(ctx context.Context, tenantID string, in <Input>, idempotencyKey string) (<Result>, error)
}
```

Rules: `ctx` first; no storage, wire, or SDK types; as small as the consumer needs; implementations assert `var _ ports.<Entity>Repository = (*Repository)(nil)`.
