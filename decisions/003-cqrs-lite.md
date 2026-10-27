# ADR-003: Commands Through the Domain, Queries Through Projections

**Status:** Accepted
**Date:** 2026-10-28

## Context

Writes enforce invariants, track versions, and record events, which justifies loading an aggregate. Reads need flexible shapes, filters, and pagination and gain nothing from reconstituting aggregates.

## Decision

- Commands follow handler → usecase → aggregate → `repo.Save`.
- Queries live in `queries/`, read through a reader port with explicit filters and keyset pagination, and return DTOs. They never mutate state.
- Both sides use the same database by default. Separate read stores or event-fed projections are introduced only for a measured need and document their freshness.
- A context that uses an aggregate for an invariant routes *every* entry point for that invariant (RPC, consumer, job) through the aggregate.

## Consequences

- **Easier:** fast, simple read paths; clear separation of what can change state.
- **Harder:** two code paths touch the same data; developers must resist putting "just a little" business logic in queries.

## Enforcement

Review, plus the [usecase rules](../.agents/rules/usecase-rules.md). Queries have no access to `Save` because the reader port does not offer it.
