# Application Layer

The application layer runs one system operation at a time. It owns workflow sequencing and application authorization, leaving business invariants to the domain and I/O mechanics to adapters.

## Commands

A typical single-aggregate command:

1. receives trusted tenant and actor identity plus validated input;
2. loads the aggregate through a port;
3. calls domain methods with time and IDs it generated;
4. saves through the repository, which commits state and events atomically;
5. returns a result or a classifiable error — no transport mapping.

Commands return `error`, plus a small result when the caller needs a created ID or computed value. See [Usecases](usecases.md).

## Queries

Queries read projections without reconstituting aggregates. They still own authorization, tenant scope, bounded pagination, deterministic ordering, and stable errors, and they never write. See [Queries](queries.md).

## Ports

Interfaces the application consumes, plus the errors adapters translate into. Small, semantic, and free of storage or wire types. See [Ports](ports.md).

## Review Checklist

- [ ] The workflow and authorization decision are visible in one place.
- [ ] Business transitions stay in the domain.
- [ ] Persistence goes through `Save`; pending work survives a failed save.
- [ ] Cross-store and provider effects are durable and idempotent or reconcilable.
- [ ] Context is passed to I/O and never stored.
- [ ] Errors stay classifiable and are not logged at every layer.
- [ ] Concurrency is bounded, cancellable, and justified.
