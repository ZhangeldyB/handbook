---
description: Rules for outbound clients, event consumers, and other adapters
paths:
  - "**/adapters/**"
  - "**/platform/**"
---

# Adapter Rules

## Ports

- Define the interface in the consuming package (`ports/` for application ports, the consuming package for narrow local seams). Name it by capability (`PaymentGateway`, `FileStore`), never by technology (`StripeClient`).
- Keep it as small as the consumer needs; never mirror a whole SDK.
- Port methods use application types; no SDK, protobuf, or `any` values cross inward.

## Adapters

- Adapters translate protocols, types, and errors and own dependency-specific lifecycle, deadlines, and retries. They make no business decisions.
- Add `var _ ports.X = (*Adapter)(nil)` for important or non-obvious contracts.
- Construct from focused `Deps` validated once; never start goroutines in a constructor.
- Every outbound call has a deadline derived from the caller's context. Background callers (jobs, consumers) set an explicit timeout.
- Translate stable provider conditions into port errors and classify retryability from typed/protocol evidence (status codes, error types), never from message text. Keep an explicit *unknown outcome* for timeouts on non-idempotent calls.
- Send a stable idempotency key on every non-idempotent provider call that supports one.
- Never log secrets, tokens, or raw provider payloads; never put them in returned error text.
- Only `cmd/` (the composition root) and adapters know concrete infrastructure types.

## Event Consumers

- One handler per topic in `adapters/eventconsumer/`. It decodes and validates the foreign contract, calls exactly one application command, and classifies the outcome — no business logic, storage access, or publishing.
- Pass the message's stable event ID to the command so the authoritative write is deduplicated in the same transaction (`IdempotencyRecord{Scope: "consume:<topic>", Key: eventID}`).
- Return nil for success and for an already-applied duplicate; wrap `messaging.ErrPermanent` for malformed payloads and commands that cannot succeed unchanged (dead-letter, never infinite retry, never silent drop); return other errors for redelivery.
- Never acknowledge a message before its effect commits.

## Verification

- Test mapping, error classification (retryable, permanent, unknown), cancellation and deadlines, cleanup, and any idempotency claim.
- Consumers: duplicate and concurrent delivery change state once; malformed and unknown inputs dead-letter.

See `.handbook/patterns/event-consumer.md` and `.handbook/templates/adapters.md`.
