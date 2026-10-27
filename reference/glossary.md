# Glossary

**Acknowledge (ack)** — Mark an aggregate's captured pending work as persisted. Happens only after the commit succeeds ([ADR-004](../decisions/004-acknowledge-after-commit.md)).

**Adapter** — Infrastructure code that implements a port or drives the application: persistence, clients, gRPC handlers, consumers, jobs.

**Aggregate** — A consistency boundary that owns invariants and state transitions; loaded and saved as a unit.

**At-least-once delivery** — A message may arrive more than once and out of order; never zero times once committed to the outbox.

**Bounded context** — The boundary within which a model and its language are authoritative.

**Claim / lease** — A time-limited, token-identified ownership of a work item (outbox record, operation, job item) that lets several replicas share work safely.

**Composition root** — `cmd/<binary>/main.go`: the only place that constructs concrete types and wires them together.

**Dead-letter** — Terminal state for a message or record that cannot succeed unchanged; kept for investigation and replay.

**Domain event** — A past-tense fact recorded by a successful transition (`OrderPlaced`).

**Expand / migrate / contract** — The three-phase sequence for changing a schema or contract while old and new versions overlap.

**Fingerprint** — A hash of a request's meaning, stored with an idempotency key to detect key reuse with a different payload.

**Idempotency record** — A row inserted in the same transaction as a write, keyed by `(tenant, scope, key)`, that makes the write commit at most once.

**Inbox record** — An idempotency record whose key is a consumed event ID.

**Optimistic concurrency** — Detecting conflicting writes with a version compare-and-set at commit instead of holding locks.

**Outbox** — A table written in the same transaction as state, holding events to be published later by a relay.

**Pending work** — An aggregate's uncommitted state change and recorded events, captured by `Begin` and cleared by `ack`.

**Port** — An interface owned by the application layer describing a capability it needs.

**Principal** — The authenticated caller (tenant, subject, scopes) carried in the request context.

**Projection** — Derived, non-authoritative read data built from events.

**Reconciliation** — Converging desired and observed state by reading durable state, comparing it with the external system, and acting idempotently.

**Relay** — The worker that claims outbox records, publishes them, and records the outcome.

**Snapshot** — The complete persisted state of an aggregate; the single mapping surface for repositories.

**Tenant** — The customer or organization that owns data; every key and query is scoped by it.

**Unknown outcome** — The state of a non-idempotent external call whose result was not observed (timeout, connection reset). Resolved by reconciliation, never by blind retry.

**Usecase** — An application command: one operation, one file, one type with `Execute`.
