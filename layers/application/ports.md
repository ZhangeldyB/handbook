# Ports

Ports are the interfaces the application layer consumes and the stable errors adapters translate into. They live in `internal/<context>/ports` (repository and provider ports) or in the consuming package for narrow local seams (`queries.OrderReader`).

## Repository Port

```go
var (
	ErrNotFound               = errors.New("order not found")
	ErrConcurrentModification = errors.New("order was modified concurrently")
	ErrDuplicate              = errors.New("duplicate idempotency key")
)

type OrderRepository interface {
	// Get returns ErrNotFound when the order does not exist in the tenant.
	Get(ctx context.Context, tenantID, orderID string) (*domain.Order, error)
	// Save writes pending state, pending events (outbox), and idempotency
	// records in one transaction. On failure the order keeps its pending work.
	Save(ctx context.Context, order *domain.Order, records ...IdempotencyRecord) error
}
```

- Semantic operations only. No rows, transactions, query builders, mutations, or `any`.
- Document the errors callers may match, the atomicity guarantee, and what remains pending on failure.
- Add `SaveAll` to a separate, narrower interface only for the consumers that batch.
- Do not add list or search methods to a command repository; reads go through a query reader port.

## Provider Ports

Express application capabilities, not SDK methods:

```go
type PaymentGateway interface {
	// Capture charges an authorized payment. key makes retries safe; an
	// ErrOutcomeUnknown result means the charge may or may not have happened.
	Capture(ctx context.Context, tenantID, paymentID string, amount domain.Money, key string) (CaptureResult, error)
}
```

Include tenant and idempotency identity in the signature when the effect needs them. Document retry behavior, stable errors, and ownership of returned readers.

## Clock and IDs

```go
type Clock interface{ Now() time.Time }
type IDGenerator interface{ NewID() string }
```

Use them wherever deterministic business inputs matter. A plain `func() time.Time` field is fine for adapters and platform code.

## Rules

- `context.Context` is the first parameter of every port method; never stored.
- Interfaces are as small as their consumers need; never mirror an SDK.
- Port types are domain or application types; no protobuf, driver, or SDK types cross inward.
- Port errors live with the port, so application code never imports an adapter to classify errors.
- Implementations assert conformance: `var _ ports.OrderRepository = (*Repository)(nil)`.
- Do not create an interface for a concrete type that has one implementation and needs no seam.

Template: [ports.md](../../templates/ports.md). Example: [ports.go](../../examples/orders/internal/orders/ports/ports.go).
