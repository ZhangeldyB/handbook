# Decision Trees

## Write Workflow

```text
Does the operation need an aggregate (several values changing under a rule)?
  no  → simplest cohesive usecase + repository flow that preserves policy
  yes → one aggregate in one store?
          yes → domain method + repo.Save
          no  → same aggregate type, all-or-nothing?
                  yes → repo.SaveAll
                  no  → one real same-store invariant across types?
                          yes → dedicated coordinator port (begin all → one tx → ack all)
                          no  → independent saves with explicit per-item results,
                                or a durable workflow

Does it touch another store, broker, cache, or provider?
  yes → state + durable intent in one tx → idempotent worker → reconcile

Can a client retry it, or does a remote effect follow the commit?
  yes → idempotency record in the same transaction (patterns/idempotency.md)

Can a timeout leave the outcome uncertain?
  yes → record UNKNOWN; reconcile before any retry
```

Before coding, answer: who owns the invariant; what commits atomically and where; what stays pending if the save fails; what the idempotency scope is; where tenant and actor identity come from; whether the outcome can be unknown; and who owns every goroutine, lease, and shutdown path.

## Read Workflow

```text
Read-only?
  no  → it is a command
  yes → does it need live business rules evaluated on aggregate state?
          yes → load the aggregate in an application operation; return a DTO
          no  → query over a reader port

Potentially large result?      → keyset pagination with default and maximum page size
Dynamic filters or sorting?    → explicit allowlist, parameterized values
Needs another context's data?  → its API or an event-fed projection — never its tables
```

## Background and Async Work

```text
Triggered by another context's event?
  → event consumer: decode, one command with the event ID (patterns/event-consumer.md)
Inbound webhook or RPC?
  → thin transport adapter calling one usecase; accept asynchronously only if the
    product contract says so
Periodic?
  → converges owned state with an external system?
       yes → reconciler run by a scheduled job (patterns/reconciliation.md)
       no  → scheduled job calling one command (implementation/scheduled-jobs.md)
Needs an independent lifecycle or durable replay?
  → record a domain event with the state change; consume it
Otherwise
  → keep it synchronous
```

Jobs call commands directly — no intermediate "syncer" layer. A global lock is optional and never replaces per-item claims and idempotency.

## Error Handling

```text
A stable condition callers branch on?
  yes → only the category matters?  → documented sentinel, errors.Is
        structured data needed?      → typed error, errors.As
  no  → crossing an abstraction boundary from infrastructure?
          callers may depend on the cause?  → wrap with context and %w
          they must not?                    → translate to a port error
        else → add operation context if it helps; otherwise return unchanged

Several independent failures (work + cleanup, batch items)?  → errors.Join
This layer handles the failure (recovers, degrades, terminates)?  → log once here
Otherwise                                                    → return it; do not log
Crossing gRPC?  → return unchanged; the central mapper translates (ADR-010)
```

## Constructor Inputs

```text
Several required collaborators?
  → <Name>Deps value, every field required, validated once with deps.Require
One self-evident collaborator?
  → positional parameter, still validated
Optional settings with safe defaults?
  → separate Config value with semantic validation
An open-ended optional surface in a public library API?
  → functional options, documented defaults
The Deps list keeps growing?
  → the component has too many responsibilities; split it
```
