---
name: add-event-consumer
description: >-
  Add a consumer for another context's event: decode and validate the foreign
  payload, call exactly one command with the event ID, deduplicate in the same
  transaction as the effect, and classify outcomes as success, retryable, or
  permanent. Use for "listen for", "subscribe to", or "react to" events.
---

# Add an Event Consumer

## Read First

1. `.handbook/patterns/event-consumer.md` and `.handbook/.agents/rules/adapter-rules.md`.
2. `.handbook/templates/event-consumer.md`.
3. Compiled example: `.handbook/examples/orders/internal/orders/adapters/eventconsumer/` and `.handbook/examples/orders/internal/orders/usecases/mark_paid.go`.
4. The producer's published payload and topic version.

## Steps

1. Trace the whole path: producer transaction → outbox → broker → handler → command → transaction (effect + inbox record) → ack/redeliver/dead-letter.
2. Create or extend the command (`implement-usecase`): it requires `EventID`, saves with `IdempotencyRecord{Scope: "consume:<topic>", Key: EventID}`, treats `ErrDuplicate` as success, and relies on idempotent domain transitions where natural.
3. Create `adapters/eventconsumer/<topic>.go`: a private foreign payload type; decode (malformed → wrap `messaging.ErrPermanent`); validate identity and tenant; call the command once; classify its errors (cannot-succeed-unchanged → permanent; everything else → retry).
4. Register the handler for the topic at startup; a subscription without a handler is a startup error.
5. Make sure the consumer runs under the lifecycle group and stops pulling on shutdown.

## Guardrails

- No business logic, repository access, or publishing in the handler.
- Never import the producer's domain package.
- Never acknowledge before the effect commits; never retry poison forever; never drop silently.
- Do not assume ordering.

## Verify

Message-to-command mapping; malformed and incomplete messages are permanent and never reach the command; permanent versus retried command errors; sequential and concurrent duplicates change state once; crash after commit before ack is safe on redelivery.

## Done

- [ ] Foreign contract translated in the adapter.
- [ ] Inbox record commits with the effect.
- [ ] Outcomes classified; handler registered; lifecycle supervised.
- [ ] Tests pass.
