# Event Consumer

Consumers turn another context's events into local commands. Delivery is at least once, so the consumer's correctness rests on one rule: **the business effect and the record that the event was processed commit in the same transaction.**

Reference implementation: [`adapters/eventconsumer`](../examples/orders/internal/orders/adapters/eventconsumer/payment_captured.go) and the [`MarkPaid` command](../examples/orders/internal/orders/usecases/mark_paid.go).

## Flow

```text
broker delivery (at least once, maybe duplicated or reordered)
  → handler: decode + validate the foreign contract
  → one application command, given the event ID
       → load aggregate → domain transition (idempotent where natural)
       → repo.Save(aggregate, IdempotencyRecord{Scope: "consume:<topic>", Key: eventID})
           one transaction: state + outbox + inbox record (insert, never upsert)
  → nil → ack | ErrPermanent → dead-letter | other error → redeliver
```

## Handler

```go
func (h *PaymentCaptured) Handle(ctx context.Context, msg messaging.Message) error {
	var payload paymentCapturedV1
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode %s %q: %w: %w", msg.Topic, msg.ID, messaging.ErrPermanent, err)
	}
	if msg.ID == "" || msg.TenantID == "" || payload.OrderID == "" || payload.PaymentID == "" {
		return fmt.Errorf("validate %s %q: missing identity: %w", msg.Topic, msg.ID, messaging.ErrPermanent)
	}

	err := h.markPaid.Execute(ctx, usecases.MarkPaidRequest{
		TenantID: msg.TenantID, OrderID: payload.OrderID,
		PaymentID: payload.PaymentID, EventID: msg.ID,
	})
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ports.ErrNotFound), errors.Is(err, domain.ErrInvalidTransition):
		return fmt.Errorf("handle %s %q: %w: %w", msg.Topic, msg.ID, messaging.ErrPermanent, err)
	default:
		return fmt.Errorf("handle %s %q: %w", msg.Topic, msg.ID, err)
	}
}
```

- The foreign payload type (`paymentCapturedV1`) stays in the adapter; never import another context's domain package.
- The handler maps, calls one command, and classifies. No business rules, no repository access, no publishing.
- Validate tenant and identity like any other untrusted input.

## Durable Idempotency

```go
err = c.orders.Save(ctx, order, ports.IdempotencyRecord{Scope: scopePaymentCaptured, Key: req.EventID})
if errors.Is(err, ports.ErrDuplicate) {
	return nil // a concurrent delivery of the same event already committed
}
```

- The inbox record is **inserted** in the same transaction as the effect. A concurrent duplicate loses the commit with `ErrDuplicate` and treats it as success.
- A separate "have I seen this?" check before the write races and is not sufficient on its own.
- Natural idempotency in the domain (`MarkPaid` with the same payment is a no-op) is a second line of defense and makes the common duplicate cheap.
- If the consumer triggers an external effect, it persists its own durable intent and follows [Reconciliation](reconciliation.md).

## Outcome Classification

| Outcome | Handler returns | Delivery |
|---|---|---|
| Success, or already applied | `nil` | Ack |
| Transient (storage outage, version conflict, dependency unavailable) | error | Redeliver with bounded backoff |
| Permanent (malformed payload, unknown aggregate, impossible transition) | error wrapping `messaging.ErrPermanent` | Dead-letter with diagnostics, ack the transport |
| External outcome unknown | per the reconciliation workflow | Never blindly repeat the effect |

Never retry a poison message forever and never drop it silently. Classify "not found" deliberately: here a payment for an unknown order cannot succeed on redelivery, so it dead-letters for investigation; a consumer that can legitimately see events before the data they refer to should retry with bounded backoff instead.

Pair per-message retry with a global brake (bounded worker concurrency) so an outage produces a slow trickle, not a retry storm.

## Ordering

Do not assume global ordering. Make handlers safe under reordering: rely on the aggregate's state machine (an out-of-order `OrderPaid` before `OrderPlaced` is rejected or deferred deliberately), or carry a per-aggregate version and ignore stale events.

## Lifecycle

- Register every handler during startup; a subscription without a handler is a startup error, not a silently acknowledged topic.
- Run the consumer under the lifecycle group; an unexpected return is a service failure.
- Shutdown stops pulling first, lets in-flight handlers finish within the budget, and leaves unfinished messages for redelivery.

## Observability

Processed, duplicate, retried, and dead-lettered counts by topic; handler latency; consumer lag and oldest unprocessed age; dead-letter backlog. No payloads or tenant IDs in metric labels. Dead-letter views redact sensitive data and support replay.

## Testing

- Mapping from message to command ([example](../examples/orders/internal/orders/adapters/eventconsumer/payment_captured_test.go)).
- Malformed and incomplete messages are permanent and never reach the command.
- Duplicate (sequential and concurrent) deliveries change state once.
- Crash after the business commit but before the ack: the redelivery is a no-op.
- Transient failures are retried; permanent ones dead-letter.
- Cross-tenant event IDs cannot collide (the inbox key includes the tenant).

## Review Checklist

- [ ] Foreign contract translated in the adapter.
- [ ] Event ID reaches the command; inbox record commits with the effect.
- [ ] Permanent, transient, and unknown outcomes are distinct.
- [ ] Handlers registered at startup; lifecycle supervised.
- [ ] Reordering and duplicates are safe.
