# Idiomatic Go

Idiomatic Go optimizes for code a maintainer can read, change, and delete safely. Prefer clarity, simplicity, and maintainability over uniformity or clever abstraction.

## Make the Workflow Visible

Keep request data local and pass explicit values. Avoid short-lived "workflow" objects whose methods secretly communicate through mutable fields.

```go
func (c *Place) Execute(ctx context.Context, req PlaceRequest) error {
	order, err := c.orders.Get(ctx, req.TenantID, req.OrderID)
	if err != nil {
		return fmt.Errorf("get order %q: %w", req.OrderID, err)
	}
	if err := order.Place(c.ids.NewID(), c.clock.Now()); err != nil {
		return err
	}
	if err := c.orders.Save(ctx, order); err != nil {
		return fmt.Errorf("save order %q: %w", req.OrderID, err)
	}
	return nil
}
```

The sequence, ownership, and error boundary are obvious. Extract a helper when it names a cohesive operation or enables focused testing — not merely to shorten a function.

## API Shape

- Keep the exported surface small.
- Accept interfaces where the consumer needs a seam; return concrete types by default.
- Use a short positional constructor while arguments are distinct; use a `Deps` struct when labels prevent swaps or make wiring auditable.
- Functional options only for genuinely optional, extensible behavior with safe defaults.
- Make zero values useful when practical; when they are not, make construction impossible to miss (the aggregate tracker panics if used uninitialized).
- Do not add a package, interface, facade, builder, or generic helper until it pays for a real variant, policy, or boundary.

## State and Ownership

- Prefer local variables to hidden mutable fields.
- Return copies of internal slices and maps; copy incoming ones you retain.
- Choose value or pointer receivers by identity, mutability, nil semantics, and copy cost — consistently per type.
- `new(expr)` is fine for an optional scalar when clearer than a temporary variable.

## Errors and Logging

- Return errors; never convert failure into success to keep a loop moving unless the contract defines partial results.
- Wrap with operation and identity context; classify with `errors.Is`/`errors.As`.
- Log once at the boundary that handles the outcome.

## Control Flow and Collections

- Early returns; keep the success path visually direct.
- A nil slice is a valid empty result; return a non-nil empty slice only when the wire contract distinguishes them.
- Preallocate when the size is known (`make([]T, 0, len(in))`).
- Do not mutate a collection while iterating it unless the semantics are explicit (`slices.DeleteFunc` is clearer).
- Use `for i := range n`, `min`/`max`, `slices.Contains`, `maps.Keys` with `slices.Sorted` — when they read better.

## Concurrency

Prefer synchronous code. When concurrency is justified: bound it, propagate cancellation, wait for every goroutine, preserve promised ordering, and make ownership testable. Never start a goroutine in a constructor. See [Context and Concurrency](../layers/shared/context-concurrency.md).

## Generics

Use type parameters to remove real duplication across types with the same behavior (`aggregate.Tracker[E]`, collection helpers). Do not use them to build frameworks or to avoid writing two clear functions.

## Verification

Format, run focused tests, the full affected module, lint, and `-race` where concurrency changed. Benchmark only to support a performance claim.

See [Go Quality Rules](../.agents/rules/go-quality.md) and [Engineering Sources](../reference/go-engineering-sources.md).
