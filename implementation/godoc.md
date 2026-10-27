# Doc Comments

Documentation is part of an API's correctness. Write enough for a caller to use the code safely without reading its implementation. Follow the [Go doc comment guide](https://go.dev/doc/comment) and check the result with `go doc`.

## What to Document

- Packages and exported API.
- Non-obvious invariants, units, ownership, and lifecycle.
- Errors callers are expected to classify.
- Ordering, idempotency, consistency, retry, and concurrency guarantees.
- Security and tenant assumptions; sensitive-data behavior.
- Surprising constraints and compatibility decisions.

Do not narrate syntax, repeat the identifier, or keep obsolete history beside current code. Unexported code needs a comment when its purpose, invariant, or failure behavior is not clear from its name and structure.

## Packages

```go
// Package outbox implements the transactional outbox: repositories append event
// records in the same transaction as aggregate state, and a Relay later
// publishes them at least once.
package outbox
```

State what the package owns and what it deliberately does not. Put a long overview in `doc.go`.

## Declarations

A doc comment is a complete sentence starting with the declared name:

```go
// Place submits a draft order. It validates everything before changing state,
// so a rejected call leaves the order and its pending events untouched.
func (o *Order) Place(eventID string, now time.Time) error
```

Include only contract information that matters: preconditions, whether failure leaves state unchanged, units and formats, zero/nil behavior, ownership of returned slices, stable errors, and concurrency or blocking behavior. Do not promise implementation details (a particular query, an allocation count) unless they are tested contracts.

## Zero Values and Constructors

Say whether the zero value is usable and what construction validates:

```go
// The zero Tracker is not usable: construct it with NewForCreate or
// NewForReconstitute. A Tracker is not safe for concurrent use.
```

## Interfaces

Document from the consumer's point of view — behavior and errors, not the intended implementation:

```go
// OrderRepository loads and atomically persists Order aggregates.
type OrderRepository interface {
	// Get returns ErrNotFound when the order does not exist in the tenant.
	Get(ctx context.Context, tenantID, orderID string) (*domain.Order, error)
	// Save writes the order's pending state, its pending events, and any
	// idempotency records in one transaction. On failure the order keeps its
	// pending work so the call can be retried.
	Save(ctx context.Context, order *domain.Order, records ...IdempotencyRecord) error
}
```

## Family Contract Sentences

When several helpers share a shape (error predicates, mappers), give them one identical contract sentence and test it:

```go
// IsNotFound reports whether err classifies as not found.
// It supports wrapped errors and returns false when err is nil.
```

## Errors

Document sentinels callers are expected to match. Error *text* is lowercase without punctuation; comments are normal prose.

## Concurrency and Lifecycle

For long-lived or concurrent types, state who starts, stops, and waits for work; whether methods are safe for concurrent use; whether `Close`/`Stop` is idempotent and what it waits for; and whether callbacks can overlap or be retried. Never document concurrency safety the implementation and race tests do not support.

## Examples

Runnable `ExampleXxx` tests teach normal usage; they must be deterministic and compile against the current API. Use ordinary tests for edge cases.

## Deprecation

```go
// Deprecated: Use ParseOrderID, which rejects ambiguous legacy formats.
func ParseLegacyID(s string) (string, error)
```

Name the replacement; do not remove or contradict a published guarantee without a compatibility plan.

## Generated Code

Generated files carry the standard `// Code generated ... DO NOT EDIT.` header before the package clause, naming the source. Never paste that header into hand-written files — linters skip files that carry it.

## Review Checklist

- [ ] Package responsibility and boundary are clear.
- [ ] Exported comments read correctly in `go doc`.
- [ ] Comments explain contracts and reasons, not syntax.
- [ ] Nil/zero, ownership, units, and state-on-error are documented where material.
- [ ] Concurrency and lifecycle claims are accurate and tested.
- [ ] Persistence, retry, idempotency, and atomicity claims match runtime behavior.
