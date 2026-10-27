# Transactional Outbox

The outbox guarantees that a committed state change is never separated from the durable intent to publish its event. It does not provide exactly-once delivery, global ordering, or atomicity with anything outside the database ([ADR-006](../decisions/006-transactional-outbox.md)).

Reference implementation: [`platform/outbox`](../examples/orders/internal/platform/outbox/relay.go) with [tests](../examples/orders/internal/platform/outbox/relay_test.go).

## Guarantees

When implemented correctly:

- aggregate state and outbox records commit or roll back together;
- a committed record survives process termination;
- the relay retries publication until it succeeds or dead-letters;
- consumers can recognize duplicates by the stable event ID.

Not guaranteed: exactly-once publication or consumption, global ordering, atomic writes across databases, or atomicity with a provider API.

## Producer Side

```text
domain transition → tracker.Record(event)
  → repo.Save: map state + convert events (before the transaction)
  → one transaction: write state (version-checked) + outbox.Append(record) for each event
  → commit → ack pending work
```

```go
for _, rec := range outboxRecords {
	if err := outbox.Append(tx, rec); err != nil {
		return err
	}
}
```

Never publish before or inside the transaction, ignore an encoding error, or clear events before commit.

## Record Schema

```sql
CREATE TABLE outbox (
    id              TEXT PRIMARY KEY,        -- stable event ID
    tenant_id       TEXT NOT NULL,
    topic           TEXT NOT NULL,           -- orders.order_placed.v1
    aggregate_id    TEXT NOT NULL,           -- ordering / partition key
    payload         BYTEA NOT NULL,
    occurred_at     TIMESTAMPTZ NOT NULL,
    status          TEXT NOT NULL,           -- PENDING | CLAIMED | PUBLISHED | DEAD
    attempts        INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL,
    claim_token     TEXT,
    claimed_until   TIMESTAMPTZ,
    last_error      TEXT,                    -- classified, sanitized
    published_at    TIMESTAMPTZ
);
CREATE INDEX outbox_due ON outbox (status, next_attempt_at);
```

(Shown in PostgreSQL syntax; adapt the types to your database.) Never store secrets or unnecessary personal data in payloads or errors.

**Retention** never deletes unfinished records (pending, claimed, dead) merely because they are old. Archive or delete confirmed-published rows on an explicit, monitored schedule.

## Claiming

Several relay replicas may run. Each cycle atomically claims a bounded batch of due records with a unique **claim token** and a **lease expiry**:

```go
func isDue(rec Record, now time.Time) bool {
	switch rec.Status {
	case StatusPending:
		return !rec.NextAttemptAt.After(now)
	case StatusClaimed:
		return !rec.ClaimedUntil.After(now) // lease expired: the claimer died or stalled
	default:
		return false
	}
}
```

In SQL, claim with a single statement that skips rows other relays hold, for example `UPDATE outbox SET status='CLAIMED', claim_token=$1, claimed_until=$2 WHERE id IN (SELECT id FROM outbox WHERE <due> ORDER BY occurred_at LIMIT $3 FOR UPDATE SKIP LOCKED)` on databases that support it.

- Bound the batch size and publish concurrency from broker quotas.
- The lease must exceed the publish timeout so a slow attempt cannot overlap a reclaim (validated by `Config.Validate`).
- Lease expiry still allows duplicate delivery when a publisher is slow or its outcome unknown — consumer idempotency remains mandatory.

## Recording Outcomes

The relay writes the outcome only while it still owns the claim token; a record reclaimed after lease expiry belongs to its new owner:

```go
if rec.Status != StatusClaimed || rec.ClaimToken != token {
	return nil // lost the lease; the new owner records its own outcome
}
```

| Outcome | Durable action |
|---|---|
| Confirmed publish | `PUBLISHED`, `published_at` |
| Transient failure | `PENDING`, `attempts+1`, `next_attempt_at = now + jittered capped backoff` |
| Permanent failure (`messaging.ErrPermanent`) or attempts exhausted | `DEAD`, logged at error level, alerted |
| Timeout (outcome unknown) | Treated as transient: republish; consumers deduplicate |

`last_error` stores a bounded class (`transient`, `permanent`, `timeout_outcome_unknown`), not raw error text, which may contain payload fragments or credentials. Write the outcome with a context that survives shutdown cancellation but is still bounded (`context.WithoutCancel` + timeout).

## Ordering

Do not promise ordering unless the whole chain enforces it. For per-aggregate order: publish with the aggregate ID as the partition key, have the relay publish a key's records sequentially (a later record never overtakes an unconfirmed earlier one), or make consumers version-aware and tolerant of reordering. Define how gaps and replays are handled.

## Relay Lifecycle

- One owned `Run(ctx)` per process, started by the composition root and supervised by the lifecycle group; a returned error is a service failure.
- Shutdown stops claiming first, finishes or abandons the in-flight batch within its budget, and waits for its goroutines. Abandoned claims expire and are recovered by another replica.
- Expose progress: if the relay loop dies while the process stays alive, readiness or an alert must fire.
- Optional fast path: after commit, signal the relay to run a cycle immediately. The polling scan remains the durable guarantee.

## Observability

Bounded metrics: pending/claimed/dead counts, **age of the oldest pending record**, claim batch size, lease expirations, publish latency and outcome class, attempts before success. Alert on backlog age and dead records, not on one transient failure.

## Testing

- Producer: domain rejection writes nothing; state and outbox commit together; a failed commit retains the event and the retry writes it once.
- Relay: publishes and marks; transient failure backs off; permanent failure dead-letters; max attempts dead-letters; an expired lease is reclaimed and the stale outcome ignored; config rejects a lease shorter than the publish timeout.
- Consumers: duplicates are safe ([Event Consumer](event-consumer.md)).

## Review Checklist

- [ ] Stable event ID and tenant on every record.
- [ ] State and outbox in one real transaction; ack after commit.
- [ ] Claims use a token and a lease; outcomes are guarded by the token.
- [ ] Retries are capped, jittered, and classified; poison records dead-letter.
- [ ] Retention never deletes unfinished records.
- [ ] Ordering promises match what the chain enforces.
- [ ] Relay lifecycle is supervised; backlog age is alerted.
