# ADR-006: Publish Events Through a Transactional Outbox

**Status:** Accepted
**Date:** 2026-10-28

## Context

Publishing to a broker after committing loses events when the process dies in between; publishing before committing announces changes that may roll back. Distributed transactions across a database and a broker are not available or not worth their cost.

## Decision

- Repositories append outbox records in the same transaction as the state change.
- A relay claims due records with a lease (owner token + expiry), publishes them outside any transaction, and records the outcome only while it still owns the lease.
- Transient failures back off exponentially with jitter and a cap; permanent failures and exhausted attempts move to a dead-letter status and alert.
- Delivery is at least once. Every consumer deduplicates on the stable event ID inside its own transaction.
- The broker is an adapter detail behind `messaging.Publisher`. Topic names and payloads are versioned contracts (`orders.order_placed.v1`).
- Retention never deletes unfinished (pending, claimed, dead) records; only confirmed-published records are archived or deleted on a monitored schedule.

## Consequences

- **Easier:** no lost or phantom events; broker outages delay rather than break writes; brokers can be swapped.
- **Harder:** an extra table and a supervised relay; consumers must be idempotent; no global ordering (only per-key ordering when the broker and relay preserve it).

## Enforcement

Outbox and consumer tests per [patterns/outbox.md](../patterns/outbox.md) and [patterns/event-consumer.md](../patterns/event-consumer.md); the reference relay is [compiled and tested](../examples/orders/internal/platform/outbox/relay_test.go).
