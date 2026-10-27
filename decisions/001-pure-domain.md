# ADR-001: Keep the Domain Pure

**Status:** Accepted
**Date:** 2026-10-28

## Context

Business rules are the most valuable and most frequently changed code in a service. When they depend on databases, transports, loggers, or request contexts, they become hard to test, persistence mechanics leak into their API, and rules get duplicated at the edges.

## Decision

- Domain packages perform no I/O and do not accept or store `context.Context`.
- Domain state contains no database rows, transactions, protobuf messages, SDK types, loggers, metrics, or configuration.
- Time, IDs, and randomness are parameters supplied by the application layer.
- Repository and other port interfaces live in the application layer (`ports/`), not in the domain.
- The domain may import other domain packages and pure platform primitives (the aggregate tracker).
- Domain enums are domain-owned types; protobuf and storage representations are mapped in adapters.

## Alternatives Considered

- **Repository interfaces in the domain package** (common in DDD literature): rejected because the interfaces need `context.Context`, which would then be legal in the domain and invite I/O.
- **Aliasing protobuf enums in the domain**: rejected because it couples business rules to a wire contract and its generator.

## Consequences

- **Easier:** exhaustive, fast, deterministic domain tests with no fakes; rules have one obvious home.
- **Harder:** adapters must map explicitly between domain, storage, and wire types, and those mappings need round-trip tests.

## Enforcement

`depguard` rule `domain` in the [lint config](../templates/golangci.yml) denies `context`, `database/sql`, `net`, `os`, `log`, gRPC, protobuf, and adapter imports under `internal/*/domain`. Review covers the rest.
