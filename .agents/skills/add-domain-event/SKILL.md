---
name: add-domain-event
description: >-
  Add a domain event: record it in the aggregate's transition method, convert
  it to a versioned published payload in the repository, write it through the
  outbox in the same transaction, and verify producer and consumer
  compatibility. Use when publishing a new business event or changing an
  existing event contract.
---

# Add a Domain Event

## Read First

1. `.handbook/layers/domain/domain-events.md` and `.handbook/patterns/outbox.md`.
2. `.handbook/guides/api-contract-evolution.md` when changing an existing event.
3. Compiled example: `.handbook/examples/orders/internal/orders/domain/events.go` and `.handbook/examples/orders/internal/orders/adapters/persistence/events.go`.
4. Every existing consumer of related topics.

## Steps

1. Confirm it is a stable past-tense business fact, not an internal notification.
2. Add the event struct to the domain's closed `Event` set with event ID, tenant, aggregate ID, occurrence time, and the payload consumers need — no secrets.
3. Record it with `tracker.Record` in the transition method, after validation and mutation; the event ID and time arrive as parameters from the usecase.
4. Add a topic constant (`<context>.<event>.v1`) and a versioned payload type in the persistence adapter; extend `toOutboxRecord`. The unknown-type branch must keep returning an error.
5. For a change to an existing event: add fields compatibly within the version, or publish a new version alongside the old one until consumers migrate.
6. Update or notify every consumer; add a contract fixture for the payload.

## Guardrails

- Never publish from the domain or the usecase; never publish before commit.
- Never ignore encoding errors.
- Do not promise ordering unless the chain enforces it.

## Verify

The transition records exactly one event with the right contents; a rejected transition records none; the converter produces the documented payload; a failed save retains the event and the retry writes it once; consumers decode the fixture.

## Done

- [ ] Past-tense fact with stable identity and time.
- [ ] Recorded in the transition method; converted in the adapter.
- [ ] Versioned topic and payload; consumers checked.
- [ ] Tests pass.
