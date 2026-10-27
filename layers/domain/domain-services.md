# Domain Services

A domain service holds business logic that does not naturally belong to one aggregate or value object. Use one sparingly; it is not a home for I/O or general helpers.

## Where Logic Goes

| Logic | Home |
|---|---|
| Protects one aggregate's invariant | Method on the aggregate |
| Pure calculation combining several domain values | Domain service (function or small type) |
| Loads data, authorizes, calls providers, manages transactions | Usecase |

## Shape

Prefer a function when there is no configuration:

```go
// Allocate splits total across weights; the parts always sum to total.
func Allocate(total Money, weights []int64) ([]Money, error)
```

Use a small value type when the logic carries immutable policy:

```go
type ShippingPolicy struct {
	freeAbove Money
	flatFee   Money
}

func (p ShippingPolicy) Fee(subtotal Money, destination Region) (Money, error)
```

Name it by the business capability (`ShippingPolicy`, `Allocate`), never `OrderService` or `Helper`. Export an interface only when a consumer needs several implementations.

## Purity

Domain services take domain values and return domain values. They are deterministic, take no `context.Context`, load nothing, log nothing, read no configuration or clock, and touch no storage or wire types. Pass versioned rates and the decision time explicitly.

## Cross-Aggregate Rules

A domain service may evaluate a rule using several aggregates' data, but it does not make them transactionally consistent:

```go
func (c *Quote) Execute(ctx context.Context, req QuoteRequest) (domain.Money, error) {
	order, err := c.orders.Get(ctx, req.TenantID, req.OrderID)
	if err != nil {
		return domain.Money{}, fmt.Errorf("get order %q: %w", req.OrderID, err)
	}
	region, err := c.addresses.Region(ctx, req.TenantID, order.CustomerID())
	if err != nil {
		return domain.Money{}, fmt.Errorf("resolve region: %w", err)
	}
	subtotal, err := order.Total()
	if err != nil {
		return domain.Money{}, err
	}
	return c.shipping.Fee(subtotal, region)
}
```

Return values instead of mutating several aggregates invisibly; let the usecase call each aggregate's own methods so each one still owns its invariant. Atomic writes across aggregates need a [coordinator](../../patterns/multi-aggregate.md).

## Testing

Pure services need no fakes. Use table tests and, where algebraic invariants exist, property or fuzz tests: allocated parts sum to the total, fees are never negative, boundaries at and around thresholds, rounding remainders, determinism, and immutability of inputs.

## Review Checklist

- [ ] No natural aggregate owner exists for this logic.
- [ ] Pure, deterministic, domain-typed inputs and outputs.
- [ ] I/O and transactions stay in the usecase.
- [ ] Named in the ubiquitous language.
