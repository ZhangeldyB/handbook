# ADR-009: gRPC APIs With buf-Managed Protobuf Contracts

**Status:** Accepted
**Date:** 2026-10-28

## Context

Services need typed, evolvable contracts with generated clients, deadlines, cancellation, and structured errors. Hand-maintained protoc invocations and ad hoc plugin versions produce irreproducible generated code and silent breaking changes.

## Decision

- Public service APIs are gRPC. Event payloads are protobuf or JSON with a versioned schema; pick one per system and record it.
- Protobuf sources live in `proto/` as a buf module. `buf lint` with the STANDARD rule set and `buf breaking` against the main branch run in CI and must pass.
- Generation uses `buf generate` with pinned remote or local plugin versions; generated code is committed under `gen/` (or produced in CI — choose once) and never edited.
- Naming follows buf STANDARD: `<Rpc>Request`/`<Rpc>Response` per RPC, enum values prefixed and zero `*_UNSPECIFIED`, versioned packages (`acme.orders.v1`).
- Well-known types for time and duration; `google.type.Money` or minor units plus currency for money.
- An HTTP/JSON surface, if needed later, is generated from the same contract (Connect or grpc-gateway) rather than hand-written.

## Consequences

- **Easier:** machine-checked compatibility, reproducible generation, consistent naming.
- **Harder:** contributors need the buf CLI; breaking changes require a new package version.

## Enforcement

`buf lint` and `buf breaking` in CI; [proto rules](../.agents/rules/proto-rules.md) for review.
