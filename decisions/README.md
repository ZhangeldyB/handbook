# Architecture Decision Records

Each ADR records one decision, its context, and its consequences. ADRs are immutable once accepted: to change a decision, add a new ADR that supersedes the old one and update the old one's status line.

| ADR | Decision |
|---|---|
| [001](001-pure-domain.md) | The domain is pure: no I/O, no context, no infrastructure types |
| [002](002-repository-owns-persistence.md) | Repositories own mapping and the transaction; ports expose only `Get`/`Save` |
| [003](003-cqrs-lite.md) | Commands go through the domain; queries read projections |
| [004](004-acknowledge-after-commit.md) | Pending work is acknowledged only after the commit succeeds |
| [005](005-optimistic-concurrency.md) | Aggregates carry a version; saves are compare-and-set full-state writes |
| [006](006-transactional-outbox.md) | Events are published through a transactional outbox, at least once |
| [007](007-exact-money.md) | Money is exact: integer minor units plus currency |
| [008](008-constructor-dependencies.md) | Dependencies are injected through validated `Deps` values |
| [009](009-grpc-and-buf.md) | Public APIs are gRPC with buf-managed protobuf contracts |
| [010](010-central-error-mapping.md) | One interceptor maps errors to gRPC status codes |

Write a new ADR with [000-template.md](000-template.md) when a decision is hard to reverse, affects several services, or will be questioned later.
