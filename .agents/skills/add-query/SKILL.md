---
name: add-query
description: >-
  Add a read-only query with a reader port, DTOs, tenant isolation, bounded
  keyset pagination, allowlisted filters, and tests. Use for get, list, search,
  or other read endpoints. Not for operations that change state.
---

# Add a Query

## Read First

1. `.handbook/layers/application/queries.md` and `.handbook/templates/query.md`.
2. Compiled example: `.handbook/examples/orders/internal/orders/queries/` and the reader methods in `.handbook/examples/orders/internal/orders/adapters/persistence/repository.go`.
3. The table schema and indexes, the proto contract, and existing queries.

## Design First

Write down: the trusted tenant source and authorization rule; exact DTO fields and their presence semantics; the index or access path and expected cardinality; the sort order with a unique tie-breaker; default and maximum page size; filter and sort allowlists; consistency expectations; not-found behavior.

## Steps

1. Define the DTO and extend the reader port in `queries/` — no rows, aggregates, or wire types.
2. Implement the query: clamp page size, decode the tenant- and filter-bound page token, fetch `limit+1`, trim, encode the next token.
3. Implement the reader method in the persistence adapter as an index-backed keyset query with the tenant first in the predicate; close rows and check `rows.Err()`; translate "no rows" to `ports.ErrNotFound`.
4. Map published filter and sort values through explicit allowlists; parameterize every value.
5. Expose it through a handler (`add-grpc-handler`).

## Guardrails

- Read-only: no `Save`, no hidden writes, no cache writes that change business state.
- Never an unbounded list; never interpolated SQL.
- Do not reconstitute aggregates unless the result needs live domain rules.

## Verify

Tenant isolation; complete DTO mapping; pagination without gaps or duplicates including ties; page-size clamping; foreign or malformed tokens → `ErrInvalidCursor`; empty and not-found results; integration test of the query plan for high-volume paths.

## Done

- [ ] DTO and reader port in `queries/`.
- [ ] Bounded keyset pagination with tenant-bound tokens.
- [ ] Allowlisted filters; parameterized SQL.
- [ ] Tests pass.
