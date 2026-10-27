# CQRS

CQRS here means separating write behavior from read models when their responsibilities differ ([ADR-003](../decisions/003-cqrs-lite.md)). It does not require separate databases, event sourcing, or a framework.

## Use It When

- Writes enforce non-trivial invariants through aggregates.
- Reads need joins, projections, filters, or shapes that differ from aggregate state.
- Read and write scaling or consistency requirements differ.

For a simple package, ordinary repository reads are enough. Do not create parallel command and query types with no behavioral benefit.

## Command Side

Commands go through the aggregate and `repo.Save` ([Usecases](../layers/application/usecases.md)). A command returns only what the caller needs (a created ID, an explicit operation state) — not the full entity because it happens to be available.

## Query Side

Queries read through a reader port, return DTOs, and never write ([Queries](../layers/application/queries.md)). They are tenant-scoped, bounded, deterministically ordered, and close their resources. Loading an aggregate for a read is fine only when its domain computation *is* the required behavior.

## Projections

A projection is derived data. Before adding one, define:

- the source events and their versions;
- the deduplication key (event ID) and how the checkpoint commits with the projection update;
- ordering and gap handling;
- rebuild and replay procedure;
- the freshness promise exposed to clients;
- schema migration and backfill;
- tenant and access control.

A projection consumer is an [Event Consumer](event-consumer.md): effect plus processed-event record in one transaction. A projection is never authoritative for a write invariant unless ownership is explicitly transferred.

## Consistency Promises

- Reads from the same database see committed writes.
- Event-fed projections and caches are eventually consistent; say how stale they can be.
- For read-your-writes, return the command's result or version, read the authoritative store, or wait for an observable projection condition with a bound — never a hidden sleep.

## Testing

- Commands: transitions, retained work on failed save, tenant authorization, idempotent replay.
- Queries: complete mapping, tenant isolation, filters, ordering, pagination boundaries, foreign tokens, cancellation.
- Projections: duplicate, out-of-order, and replayed events; rebuild; lag visibility.
