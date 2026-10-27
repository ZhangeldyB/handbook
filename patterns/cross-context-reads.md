# Cross-Context Reads

Contexts often need data another context owns. Reading is negotiable; writing is not.

**Hard rule: never write another context's data directly.** Its state changes only through the owner's API or by the owner consuming an event. A foreign write bypasses the owner's invariants, versioning, outbox, and audit trail.

## Option A: Ask the Owner (Synchronous API)

Use when the answer must be current, depends on the owner's logic or authorization, or the owner exposes no projection.

- Define a narrow read port in the consuming context (`ports.CustomerDirectory`).
- The adapter wraps the generated client, propagates identity, derives its deadline from the caller's context, and translates errors into port errors ([gRPC Clients](../layers/infrastructure/grpc-clients.md)).
- Map responses to local types at the adapter; wire types never cross inward.
- Decide what happens when the owner is unavailable: fail the operation, serve a stated-stale cache, or degrade a feature.

## Option B: Keep a Local Projection (Events)

Use for high-volume reads that tolerate stated staleness.

- Consume the owner's published events into a local read table with an [Event Consumer](event-consumer.md).
- Store only the fields you need; record the source event ID and version for deduplication and ordering.
- Document freshness, expose lag as a metric, and provide a rebuild procedure (replay or snapshot from the owner's API).
- The projection is never authoritative for a write invariant.

## Option C: Composition at the Edge

A client-facing composition layer (BFF or gateway) may call several owners and assemble a view. The result is a DTO, not an aggregate. Define fan-out limits, deadlines, partial-failure behavior, and authorization per call.

## Decision Rule

| Situation | Option |
|---|---|
| Must be current, or depends on owner logic/authorization | A |
| High volume, staleness acceptable | B |
| A UI needs several owners' data in one response | C |
| Any write | The owner's API or an event the owner consumes — never direct |

Sharing a physical database between contexts is an operational choice, not an integration contract: even then, read another context's tables only through an explicitly agreed, versioned read model, and never write them.

## Review Checklist

- [ ] No write ever targets another context's data.
- [ ] The adapter implements a locally defined port and returns local types.
- [ ] Synchronous reads carry identity and deadlines and translate errors.
- [ ] Projections state their freshness and rebuild procedure.
