# Usecases

A usecase coordinates one application operation across identity, domain behavior, persistence, and external ports. It should read like the workflow a developer is trying to understand.

## Responsibilities

- Load state through ports; enforce application authorization and tenant scope.
- Generate time and IDs once per operation and pass them to the domain.
- Use one persistence boundary (`Save`) or an explicit durable workflow.
- Propagate cancellation; return stable, contextual errors.

Usecases do not map protobuf, touch rows or transactions, call another usecase, or re-implement rules the domain owns.

## Package and Type Shape

```text
internal/orders/usecases/place.go          # small context: package usecases; type Place
internal/orders/usecases/order/place.go    # large context: package order; type Place
```

Never `usecases/place/place.go`. Name the type after the operation (`Place`, `CreateDraft`, `MarkPaid`), not `Interactor` or `Service`. A command facade struct that groups operations for wiring is optional and holds already-constructed commands, never their dependencies.

## A Single-Aggregate Command

```go
type PlaceDeps struct {
	Orders ports.OrderRepository
	Clock  ports.Clock
	IDs    ports.IDGenerator
}

type Place struct {
	orders ports.OrderRepository
	clock  ports.Clock
	ids    ports.IDGenerator
}

func NewPlace(d PlaceDeps) (*Place, error) {
	if err := errors.Join(
		deps.Require("Orders", d.Orders),
		deps.Require("Clock", d.Clock),
		deps.Require("IDs", d.IDs),
	); err != nil {
		return nil, fmt.Errorf("new place: %w", err)
	}
	return &Place{orders: d.Orders, clock: d.Clock, ids: d.IDs}, nil
}

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

After `NewPlace` succeeds, `Execute` never checks `c.orders == nil`. Domain errors are returned unwrapped when the domain already added context.

## Creation With an Idempotency Key

Client-facing creates are retried after timeouts, so they **must** be idempotent. The key record commits in the same transaction as the aggregate:

```go
func (c *CreateDraft) Execute(ctx context.Context, req CreateDraftRequest) (CreateDraftResult, error) {
	fingerprint := fingerprintOf(req)
	if req.IdempotencyKey != "" {
		if result, ok, err := c.replay(ctx, req, fingerprint); err != nil || ok {
			return result, err // original result, or ErrIdempotencyKeyReused
		}
	}
	order, err := c.buildOrder(req)
	if err != nil {
		return CreateDraftResult{}, err
	}
	var records []ports.IdempotencyRecord
	if req.IdempotencyKey != "" {
		records = append(records, ports.IdempotencyRecord{
			Scope: scopeCreateDraft, Key: req.IdempotencyKey, Fingerprint: fingerprint, Response: order.ID(),
		})
	}
	err = c.orders.Save(ctx, order, records...)
	if errors.Is(err, ports.ErrDuplicate) {
		// A concurrent request with the same key won the race; return its result.
		if result, ok, replayErr := c.replay(ctx, req, fingerprint); replayErr != nil || ok {
			return result, replayErr
		}
	}
	if err != nil {
		return CreateDraftResult{}, fmt.Errorf("save draft order %q: %w", order.ID(), err)
	}
	return CreateDraftResult{OrderID: order.ID()}, nil
}
```

Full pattern and selection rule: [Idempotency](../../patterns/idempotency.md).

## Event-Driven Commands

A command driven by a consumer receives the event ID and deduplicates on it in the same transaction:

```go
err = c.orders.Save(ctx, order, ports.IdempotencyRecord{Scope: scopePaymentCaptured, Key: req.EventID})
if errors.Is(err, ports.ErrDuplicate) {
	return nil // a concurrent delivery of the same event already committed
}
```

Combined with an idempotent domain transition (`MarkPaid` with the same payment is a no-op), redelivery is harmless.

## Concurrency Conflicts

`Save` returns `ports.ErrConcurrentModification` when another writer committed first. Choose per operation:

- **Surface it** (`Aborted`) when the client should re-read and decide.
- **Retry internally** when re-applying the command to fresh state is always correct: reload, re-run the domain method, save — a small, bounded number of attempts, honoring `ctx`.

Never retry by saving the same stale aggregate again.

## Batches

- Same aggregate type, all-or-nothing: `repo.SaveAll(ctx, orders)`.
- Independent items: process each, collect per-item results, return `errors.Join` of item-wrapped failures or an explicit result list — never a silent partial success.
- Different aggregate types with one real same-store invariant: a dedicated coordinator — see [Multi-Aggregate Workflows](../../patterns/multi-aggregate.md).

## External Effects

The unsafe sequence is "call the provider, then hope the database write succeeds". Instead:

```text
persist state + intent (outbox/operation record) atomically
  → worker claims intent
  → provider call with a stable idempotency key
  → record confirmed success, confirmed failure, or unknown outcome
  → reconcile unknown before any retry
```

See [Reconciliation](../../patterns/reconciliation.md). Never call a provider inside a transaction callback.

## Identity and Requests

- Application request types are plain Go. Handlers fill `TenantID` and actor fields from the authenticated principal.
- Fail closed: if an operation requires an actor, reject a missing one — never default to a "system" actor.
- Use pointer or explicit optional types only where absent and zero mean different things (partial updates).

## Errors and Logging

- Preserve classifiable errors with `%w`; add operation and identity context once.
- Do not import gRPC status packages; return domain and port errors for the central mapper.
- Do not log and return. The transport boundary logs unexpected failures once.

## Testing

Use the real domain and small fakes at ports ([example](../../examples/orders/internal/orders/usecases/usecases_test.go)):

- constructor rejects each missing dependency (including a typed nil);
- success, domain rejection, not-found, save failure with pending work retained;
- idempotent replay, key reuse with a different payload, race-loser recovery;
- duplicate event delivery is a no-op;
- cancellation propagates; tenant from the request reaches the port.

## Review Checklist

- [ ] One operation, one file, operation-named type.
- [ ] Only the collaborators it uses, validated once.
- [ ] Time and IDs injected and captured once.
- [ ] Tenant from the principal reaches every port call.
- [ ] Persistence through `Save`/`SaveAll`; no transactions in the usecase.
- [ ] Idempotency where the selection rule requires it.
- [ ] External effects are durable and reconcilable.
