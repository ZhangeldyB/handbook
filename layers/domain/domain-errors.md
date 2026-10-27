# Domain Errors

Errors communicate failures across package boundaries. Design them around decisions callers make, not around a taxonomy of layers.

## Stable Versus Incidental Errors

Create a sentinel or typed error only when a caller branches on it:

```go
var ErrInvalidTransition = errors.New("invalid order status transition")

type LimitExceededError struct {
	Limit, Requested Money
}

func (e *LimitExceededError) Error() string {
	return fmt.Sprintf("requested %s exceeds limit %s", e.Requested, e.Limit)
}
```

Everything else is wrapped with context. Do not create a sentinel per operation just to label it.

When many callers check one condition, expose a predicate whose doc comment states the shared contract — "It supports wrapped errors and returns false when err is nil." — and implement it with `errors.Is`/`errors.As`, never string matching.

## Wrapping

```go
order, err := c.orders.Get(ctx, req.TenantID, req.OrderID)
if err != nil {
	return fmt.Errorf("get order %q: %w", req.OrderID, err)
}
```

A clean chain reads like a sentence: `place order "o-1": save order "o-1": commit: connection refused`. Wrap with `%w` when the cause is part of your contract; translate infrastructure errors at the adapter boundary when callers must not depend on them (`sql.ErrNoRows` → `ports.ErrNotFound`). Avoid `failed to` prefixes at every layer.

`errors.Join` preserves several independent failures (primary plus cleanup, per-item batch failures wrapped with item identity). It is not a way to attach context.

## State on Error

Document and test what a failure leaves behind:

- Domain methods validate before mutating — state unchanged.
- A failed `Save` leaves pending work retained for a retry.
- An external timeout may leave the outcome *unknown*, not failed. Keep that state distinct and reconcile before retrying a non-idempotent effect.

## Cancellation

Preserve `context.Canceled` and `context.DeadlineExceeded` through wrapping. A deadline means the caller stopped waiting; it does not prove a remote effect did not happen.

## Retry Classification

Whether to retry depends on more than the error: confirmed versus ambiguous failure, the operation's idempotency, attempt and age budgets, transaction semantics, cancellation, and the product contract. Adapters translate provider codes into a small internal classification (retryable, permanent, unknown), keeping the original as a cause only when safe.

## Logging

Inner layers return errors; the operational boundary (gRPC interceptor, consumer runner, job runner, `main`) logs once with safe structured fields. Expected outcomes (validation failures, not-found) are not error logs. Error text can reach logs, traces, and alerts — never put credentials, tokens, or sensitive payloads in it.

## Transport Mapping

Handlers return errors unchanged; one `grpcerr.Mapper` per service translates them ([ADR-010](../../decisions/010-central-error-mapping.md), [example](../../examples/grpctransport/grpcerr/grpcerr.go)). Every mapper is a thin rule table against this canonical mapping — the only place it is defined:

| Condition | gRPC code |
|---|---|
| Malformed or invalid request fields | `InvalidArgument` (with `BadRequest` field violations) |
| Not authenticated | `Unauthenticated` |
| Authenticated but not allowed | `PermissionDenied` (or `NotFound` when existence must be hidden) |
| Resource not found in the caller's tenant | `NotFound` |
| Uniqueness conflict on create | `AlreadyExists` |
| Optimistic-concurrency conflict | `Aborted` |
| Invalid state transition / business precondition | `FailedPrecondition` |
| Quota or rate limit | `ResourceExhausted` |
| Dependency outage (retryable) | `Unavailable` |
| `context.Canceled` | `Canceled` |
| `context.DeadlineExceeded` | `DeadlineExceeded` |
| Anything unmapped | `Internal` with a generic message, logged once |

```go
mapper := grpcerr.NewMapper(
	grpcerr.Rule{Target: ports.ErrNotFound, Code: codes.NotFound, Message: "order not found"},
	grpcerr.Rule{Target: ports.ErrConcurrentModification, Code: codes.Aborted, Message: "order was modified concurrently; retry"},
	grpcerr.Rule{Target: domain.ErrInvalidTransition, Code: codes.FailedPrecondition, Message: "order is not in a valid state for this operation"},
	grpcerr.Rule{Target: domain.ErrEmptyOrder, Code: codes.FailedPrecondition, Message: "order has no lines"},
	grpcerr.Rule{Target: usecases.ErrIdempotencyKeyReused, Code: codes.InvalidArgument, Message: "idempotency key reused with a different request"},
)
```

Public messages are static text from the rule — never `err.Error()`. Changing a code is an API change.

## Partial Results

If an operation can legitimately partially succeed, make it explicit with per-item results or a durable job status. Never skip failures silently and report the input count as success. For batch writes decide up front: atomic all-or-nothing, independent per-item, or asynchronous.

## Testing

- `errors.Is`/`errors.As` work through wrapping.
- Infrastructure errors are translated, not leaked.
- State unchanged or retained as documented.
- Cancellation propagates.
- Every mapper rule produces its code; unmapped errors become sanitized `Internal`.
