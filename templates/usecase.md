# Usecase Template

Working versions: [place.go](../examples/orders/internal/orders/usecases/place.go), [create_draft.go](../examples/orders/internal/orders/usecases/create_draft.go), [mark_paid.go](../examples/orders/internal/orders/usecases/mark_paid.go).

## Command

`internal/<context>/usecases/<operation>.go`:

```go
package usecases

import (
	"context"
	"errors"
	"fmt"

	"<module>/internal/<context>/ports"
	"<module>/internal/platform/deps"
)

// <Operation>Deps are the required collaborators of <Operation>.
type <Operation>Deps struct {
	<Entities> ports.<Entity>Repository
	Clock      ports.Clock
	IDs        ports.IDGenerator
}

// <Operation> <does what, in business language>.
type <Operation> struct {
	<entities> ports.<Entity>Repository
	clock      ports.Clock
	ids        ports.IDGenerator
}

// New<Operation> validates its dependencies once.
func New<Operation>(d <Operation>Deps) (*<Operation>, error) {
	if err := errors.Join(
		deps.Require("<Entities>", d.<Entities>),
		deps.Require("Clock", d.Clock),
		deps.Require("IDs", d.IDs),
	); err != nil {
		return nil, fmt.Errorf("new <operation>: %w", err)
	}
	return &<Operation>{<entities>: d.<Entities>, clock: d.Clock, ids: d.IDs}, nil
}

// <Operation>Request is the application input. TenantID comes from the
// authenticated principal, never from the request body.
type <Operation>Request struct {
	TenantID string
	<Entity>ID string
}

func (c *<Operation>) Execute(ctx context.Context, req <Operation>Request) error {
	<entity>, err := c.<entities>.Get(ctx, req.TenantID, req.<Entity>ID)
	if err != nil {
		return fmt.Errorf("get <entity> %q: %w", req.<Entity>ID, err)
	}
	if err := <entity>.<DomainMethod>(c.ids.NewID(), c.clock.Now()); err != nil {
		return err
	}
	if err := c.<entities>.Save(ctx, <entity>); err != nil {
		return fmt.Errorf("save <entity> %q: %w", req.<Entity>ID, err)
	}
	return nil
}
```

## Variants

| Need | Change |
|---|---|
| Client-retried create | Look up the idempotency key first; pass an `IdempotencyRecord` to `Save`; on `ErrDuplicate` look up again and return the winner ([create_draft.go](../examples/orders/internal/orders/usecases/create_draft.go)) |
| Driven by an event consumer | Require `EventID`; pass `IdempotencyRecord{Scope: "consume:<topic>", Key: EventID}`; treat `ErrDuplicate` as success ([mark_paid.go](../examples/orders/internal/orders/usecases/mark_paid.go)) |
| Same-type atomic batch | Load all, apply the domain method to each, `SaveAll`; verify every requested ID was found before mutating any |
| Partial update | Pointer or `optional` fields mean "present"; `nil` never means "clear" |
| External effect | Save state plus intent; a worker performs the call ([Reconciliation](../patterns/reconciliation.md)) |
| Conflict retry | On `ErrConcurrentModification`, reload and re-apply a bounded number of times, honoring `ctx` |

## Checklist

- [ ] One operation per file; operation-named type; `Execute(ctx, req)`.
- [ ] Only the collaborators it calls, validated once.
- [ ] Time and IDs injected.
- [ ] Tenant from the request (filled by the handler from the principal).
- [ ] Persistence only through `Save`/`SaveAll`.
- [ ] Constructor and behavior tests ([usecase-test.md](usecase-test.md)).
