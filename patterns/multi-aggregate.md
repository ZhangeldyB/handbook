# Multi-Aggregate Workflows

The default is one aggregate per transaction. Use a multi-aggregate transaction only when a concrete invariant spans those aggregates and every write lives in the same database.

## Decide the Boundary

1. Which invalid state becomes observable if these writes commit separately?
2. Which context owns that invariant?
3. Are all rows in the same database and transaction?
4. How many records can one operation touch at production scale?
5. Can an event-driven workflow satisfy the requirement instead?
6. What happens on retry, partial validation failure, and process termination?

Do not merge aggregates into a "god aggregate" to get one transaction. Model a smaller root, enforce the invariant with a database constraint, or coordinate a durable workflow.

## Same Type, All-or-Nothing: SaveAll

A batch of one aggregate type uses the repository's `SaveAll`, which begins every aggregate, writes all of them in one transaction, and acknowledges all only after the commit ([implementation and test](../examples/orders/internal/orders/adapters/persistence/repository_test.go)). Calling `Save` in a loop is wrong for an atomic batch: earlier aggregates would be committed and acknowledged even if a later one fails.

## Different Types, One Store: A Coordinator

When one real invariant spans different aggregate types in the same database, define one semantic port for that operation:

```go
// ports/ports.go
type ReservationStore interface {
	// SaveOrderWithReservation commits both aggregates in one transaction or neither.
	SaveOrderWithReservation(ctx context.Context, order *domain.Order, stock *inventory.Reservation) error
}
```

The adapter implements it with the same lifecycle as `SaveAll`:

```text
begin every aggregate (capture pending state and events, no acknowledgement)
  → map and validate everything
  → one transaction: every state write (version-checked) + every outbox record
  → commit → acknowledge every aggregate
  → any failure → acknowledge none
```

The usecase never sees a transaction, rows, or acks. A heterogeneous coordinator is exceptional; prefer independent operations when no shared invariant exists.

## Across Stores or Contexts

Nothing is atomic just because writes are in one Go function. When aggregates live in different databases or contexts:

1. choose one authoritative transition;
2. commit it plus a durable intent (outbox event or operation record);
3. deliver to the other side at least once;
4. apply idempotently on the stable event or operation ID;
5. record confirmed, failed, and unknown outcomes;
6. reconcile until convergence or a terminal, alerted state.

Avoid synchronous usecase-to-usecase calls across contexts; depend on a published event or the owner's API.

## Contention

Large transactions increase conflicts, lock time, retry work, and blast radius. Bound batch sizes from measured limits. Partition independent work and return per-item results when full atomicity is unnecessary. Version checks prevent stale overwrites but do not replace domain validation; a retried command reloads and revalidates.

## Failure Semantics to Define

- validation failure before any write;
- conflict and retry behavior;
- known rollback versus unknown external outcome;
- atomic versus per-item results;
- idempotency key scope (tenant + operation + target);
- compensation or reconciliation owner;
- terminal and manual-intervention states.

"Best effort" is not a specification.

## Testing

Same store: all states and outbox records commit together; any failure acknowledges nothing; a retry resubmits every event once; batch limits are enforced. Across stores: duplicates and reordering are safe; a downstream outage leaves durable pending work; termination at every boundary converges after restart; unknown outcomes reconcile before retry.
