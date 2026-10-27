# Go Engineering Handbook

Architecture guidance, conventions, compiled examples, and an AI-agent bundle for building Go microservices: gRPC APIs, a pure domain model, repository-owned transactions with a transactional outbox, and at-least-once event processing that is safe by construction.

It is database- and broker-agnostic. The patterns are proven by runnable, tested Go code in [`examples/`](examples/README.md).

## Install Into a Service

```bash
git clone https://github.com/ZhangeldyB/handbook.git
./handbook/scripts/install.sh /path/to/your-service
```

This installs the handbook to `.handbook/`, the root agent instructions to `AGENTS.md` and `CLAUDE.md`, and the rules, skills, subagents, and hooks to `.claude/`. See [Distribution](reference/distribution.md).

## Start Here

1. [Getting Started](guides/getting-started.md)
2. [Architecture Overview](architecture/overview.md) and [Request Flow](architecture/request-flow.md)
3. [Service Layout](architecture/service-layout.md)
4. [Quick Reference](guides/quick-reference.md)
5. Building from scratch? [New Service](guides/new-service.md)

## Contents

| Section | What is in it |
|---|---|
| [Architecture](architecture/README.md) | Principles, request flow, layout, bounded contexts |
| [Decisions](decisions/README.md) | ADRs for the load-bearing choices |
| [Layers](layers/README.md) | Domain, application, infrastructure, and cross-cutting concerns |
| [Patterns](patterns/README.md) | Outbox, event consumers, idempotency, reconciliation, CQRS, multi-aggregate, anti-patterns |
| [Implementation](implementation/README.md) | Naming, style, doc comments, enums, logging, tracing, scheduled jobs |
| [Testing](testing/README.md) | Unit, integration, contract, E2E, and CI |
| [Guides](guides/README.md) | Workflows: new service, new feature, contract evolution, debugging, dependencies |
| [Templates](templates/README.md) | Copyable scaffolds plus lint, buf, Makefile, and Dockerfile configs |
| [Examples](examples/README.md) | Compiled and tested reference code |
| [Reference](reference/README.md) | PR checklist, glossary, sources, distribution, validation |
| [Agent bundle](.agents/README.md) | Path-scoped rules, skills, subagents, and hooks for coding agents |

## Common Tasks

| Task | Go to |
|---|---|
| Add a write operation | [Usecases](layers/application/usecases.md) · [template](templates/usecase.md) |
| Model an aggregate | [Aggregates](layers/domain/aggregates.md) · [template](templates/domain.md) |
| Persist an aggregate | [Repositories](layers/infrastructure/repositories.md) · [template](templates/repository.md) |
| Add an RPC | [gRPC Handlers](layers/infrastructure/grpc-handlers.md) · [Protobuf Contracts](layers/infrastructure/proto-contracts.md) |
| Publish events | [Transactional Outbox](patterns/outbox.md) |
| Consume events | [Event Consumer](patterns/event-consumer.md) |
| Make a write retry-safe | [Idempotency](patterns/idempotency.md) |
| Call a provider whose outcome matters | [Reconciliation](patterns/reconciliation.md) |
| Change an API, event, or schema | [API Contract Evolution](guides/api-contract-evolution.md) · [Migrations](layers/infrastructure/migrations.md) |
| Investigate production behavior | [Evidence-Driven Debugging](guides/evidence-driven-debugging.md) |

## The Core Rules in One Screen

- The **domain** owns invariants. It is pure: no context, no I/O, time and IDs as parameters.
- **Usecases** orchestrate one operation through ports; they never touch transactions or rows.
- **Repositories** `Save` state (version-checked), outbox records, and idempotency records in **one** transaction and acknowledge pending work only after commit.
- **Handlers** validate the wire, take identity from the authenticated principal, call one operation, and return errors to **one** central mapper.
- Delivery is **at least once**; every consumer deduplicates on the event ID in its own transaction.
- A timeout is an **unknown** outcome, reconciled before any retry.
- Money is **exact**. Every goroutine is **owned**. Every contract change is **compatible**.

## Contributing

Read [CLAUDE.md](CLAUDE.md) for authoring standards, then run `make verify` before opening a change.
