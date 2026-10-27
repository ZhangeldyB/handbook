# Durable Reconciliation

Reconciliation converges desired state (what we decided) and observed state (what a provider or another system actually did) when calls can fail, time out ambiguously, or outlive a request. The workflow survives retries, multiple replicas, process termination, and unknown outcomes.

## Use It When

- A provider call cannot join the local transaction and its outcome matters (payments, shipments, provisioning).
- Calls may succeed after the client deadline.
- Retry policy depends on durable attempts and classification.
- Operators need an audit trail and manual intervention.

Do not use it for an ordinary local transaction.

## Durable State Machine

```text
PENDING ──claim──► IN_FLIGHT ──► CONFIRMED_SUCCESS
                       │      ├─► CONFIRMED_FAILURE_RETRYABLE ──backoff──► PENDING
                       │      ├─► CONFIRMED_FAILURE_TERMINAL
                       │      └─► OUTCOME_UNKNOWN ──reconcile──► (success | retryable | terminal | still unknown)
                       └── lease expired ──► reclaimed
```

Names follow your domain, but **unknown is never collapsed into failed**. A timeout or broken connection means the client stopped waiting; the provider may have applied the effect.

Each operation has a stable identity scoped by tenant, kind, and target, and stores: operation/idempotency ID, desired target and version, phase, provider reference when known, attempt count and next-attempt time, lease owner and expiry, a classified safe error, and lifecycle timestamps.

## Flow

```text
request or event
  → persist desired state + operation (PENDING) in one transaction
  → worker claims a due operation with a lease
  → provider call OUTSIDE any transaction, with the operation ID as idempotency key
  → persist confirmed success/failure or OUTCOME_UNKNOWN in a new transaction
  → unknown: query provider status, then decide
  → retry only when a duplicate effect is impossible or harmless
```

- **Initiate**: the usecase changes authoritative state and creates the operation record via `repo.Save`, atomically. Return "accepted/pending" to the caller only if that is the public contract; never report provider success before it is confirmed.
- **Claim**: atomically claim a bounded batch with owner and expiry ([same technique as the outbox](outbox.md#claiming)).
- **Perform**: call the provider with a stable idempotency key and a deadline inside the attempt budget.
- **Record**: if the local write fails after provider success, the operation stays unresolved and is reconciled by its stable identity — never issue the effect again blindly.
- **Reconcile unknown**: prefer a provider status or idempotency lookup. Applied → record success. Confirmed absent → schedule a retry. Still ambiguous → stay unknown with bounded backoff. Irreconcilable or expired → terminal manual-intervention state with an alert.

## Cycle Outcomes

Every reconcile cycle returns exactly one of three outcomes:

| Outcome | Return | Effect |
|---|---|---|
| Converged, nothing to do, or target gone | `nil` | Reset attempt counters; treat not-found as converged — never requeue a tombstone |
| Transient failure | error | Per-item capped, jittered backoff |
| Permanent data problem retry cannot fix | `nil` after logging once | Wait for the next state change; persist a terminal/manual state if loss matters |

The third row is the one sanctioned exception to "never log and swallow", scoped to sweep isolation: one broken item must not stall the rest.

**Coalescing invariant:** one cycle is equivalent to processing every queued trigger for that item — it reads the current durable state and converges to it. Triggers are edges; durable state is the level. Never replay per-event deltas. This makes duplicate, missed, and coalesced triggers all safe.

## Controller Shape

```go
type Reconciler interface {
	Reconcile(ctx context.Context, tenantID, operationID string) error
}
```

One entry point loads the durable operation, checks lease and version, and picks the next action. Extract policies or adapters only when they own a real variant. Do not generate a forest of `Initiate`/`Perform`/`Finalize` packages.

## Worker Lifecycle

An in-memory queue may deduplicate and schedule keys, but it is never the source of truth:

```text
get key → reconcile latest durable state
   success / gone   → forget key (reset backoff)
   retryable        → requeue with backoff
   terminal         → persist terminal state, forget key
   unknown          → persist unknown, schedule a bounded status lookup
shutdown → stop accepting → drain the queue → wait for workers
```

- One key is never processed by two local workers at once; database leases and versions protect across replicas.
- Retry decisions come from typed, durable classification — never from error strings.
- A retry cap never silently drops business work: persist a terminal or manual state and alert.
- Startup waits for required dependencies before claiming; shutdown stops claims before draining.
- Bound worker concurrency from provider and database quotas.

## Bulk Maintenance Is Not Reconciliation

Backfills and large cleanups are reviewed, idempotent, batched maintenance jobs — not per-item reconcile loops over millions of rows.

## Observability

Backlog and oldest age by phase, lease expirations, provider latency and outcome class, unknown count and age, attempts before success, convergence latency, terminal and manual states. Alert on aged unknown and pending work. Logs carry safe operation and target IDs, attempt, phase, and class — never credentials or provider payloads.

## Testing

Use a provider fake that can apply an effect *and then* time out.

- Valid and invalid phase transitions; superseded desired versions.
- Intent and state commit atomically; failed saves retain pending work.
- Provider success followed by a failed local write converges without a duplicate effect.
- Timeout before and after the provider applied: both reconcile correctly.
- Duplicate and expired leases across replicas are safe.
- Retry caps, backoff, and terminal classification.
- Process termination at every boundary recovers.
- Shutdown stops claims and waits for owned work.

## Review Checklist

- [ ] Desired state and intent commit before the provider call.
- [ ] Stable, tenant-scoped operation identity used as the provider idempotency key.
- [ ] Provider call outside any transaction.
- [ ] Unknown is distinct from failure and reconciled before retry.
- [ ] Lease and version semantics are multi-replica safe.
- [ ] Retries are capped, jittered, and durable; nothing is silently dropped.
