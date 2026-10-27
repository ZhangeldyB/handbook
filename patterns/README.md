# Patterns

Reusable solutions for the problems every service meets once it has state, events, and external dependencies.

## Selection Guide

| Situation | Pattern |
|---|---|
| Change one aggregate | Usecase → domain → `repo.Save` ([Request Flow](../architecture/request-flow.md)) |
| Publish what happened | [Transactional Outbox](outbox.md) |
| React to another context's events | [Event Consumer](event-consumer.md) |
| Clients retry writes; duplicates must not happen | [Idempotency](idempotency.md) |
| Call a provider whose outcome matters | [Durable Reconciliation](reconciliation.md) |
| Read-heavy or differently shaped reads | [CQRS](cqrs.md) |
| Several aggregates in one operation | [Multi-Aggregate Workflows](multi-aggregate.md) |
| Need another context's data | [Cross-Context Reads](cross-context-reads.md) |
| Review signals for what not to do | [Anti-patterns](antipatterns.md) |

## The Core Guarantees

1. **One transaction is the only atomic unit.** State, outbox records, and idempotency records for one operation commit together in one database.
2. **Pending work is acknowledged after commit.** A failed save loses nothing ([ADR-004](../decisions/004-acknowledge-after-commit.md)).
3. **Delivery is at least once.** Every consumer and worker is idempotent on a stable ID.
4. **Unknown is a state.** A timeout on a non-idempotent effect is recorded as unknown and reconciled, never blindly retried.
5. **Leases, not locks, coordinate replicas.** Claims expire; idempotency covers the overlap.

Every pattern here is a combination of these five.
