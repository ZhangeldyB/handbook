# Policies

Most business rules are clearest as an aggregate method, a private predicate, or a value-object method. Introduce a named policy (sometimes called a specification) only when a pure decision:

- has a stable name in the business language;
- is reused by more than one real caller, or has meaningful variants;
- needs explicit inputs such as a threshold or an effective time; and
- performs no I/O and exposes no storage or wire types.

It is a tool for a demonstrated responsibility, not a layer every aggregate needs.

## Keep the Aggregate in Charge

A policy may answer a question; it must not let callers bypass the aggregate that owns the transition:

```go
// The usecase may consult a policy first...
if !c.approval.Allows(order.Snapshot()) {
	return domain.ErrApprovalRequired
}
// ...but Approve still enforces the order's own transition rules.
if err := order.Approve(c.clock.Now()); err != nil {
	return err
}
```

## Smallest Useful Shape

```go
type HighValuePolicy struct {
	threshold Money
}

func NewHighValuePolicy(threshold Money) (HighValuePolicy, error) {
	if threshold.Minor() < 0 {
		return HighValuePolicy{}, errors.New("threshold must not be negative")
	}
	return HighValuePolicy{threshold: threshold}, nil
}

func (p HighValuePolicy) IsSatisfiedBy(total Money) bool {
	return total.Currency() == p.threshold.Currency() && total.Minor() >= p.threshold.Minor()
}
```

Add an interface (`type Specification[T any] interface{ IsSatisfiedBy(T) bool }`) only when a consumer needs several implementations or composition. Do not pre-build generic `And`/`Or`/`Not` combinators, builders, and a separate package.

Return a stable error or typed result instead of `bool` when callers must know *why* the policy failed.

## Policies Are Not Queries

A domain policy is not a query language. Query adapters translate typed application filters into SQL with allowlisted columns. Do not load an unbounded table to reuse an in-memory predicate that should have been a `WHERE` clause.

## Testing

Values below, at, and above thresholds; currency and timezone boundaries; constructor validation; and that a rejected policy leaves the aggregate unchanged. Delete a policy abstraction with one trivial caller — clear ownership beats pattern consistency.
