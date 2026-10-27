# Getting Started

This handbook is a decision aid for Go services. Start from the problem's real complexity; do not add layers because a template exists.

## Install Into a Repository

From a checkout of this handbook:

```bash
./scripts/install.sh /path/to/your-service
```

This copies the handbook to `.handbook/` in the target, projects the root agent instructions to `AGENTS.md` and `CLAUDE.md`, and installs the agent rules, skills, subagents, and hook settings under `.claude/`. Details: [Distribution](../reference/distribution.md).

## Read in This Order

1. [Architecture Overview](../architecture/overview.md) — principles and when to use the layered model.
2. [Request Flow](../architecture/request-flow.md) — one command traced end to end, with every failure path.
3. [Service Layout](../architecture/service-layout.md) — where code goes.
4. [Quick Reference](quick-reference.md) — the cheat sheet.
5. The compiled example: [`examples/orders`](../examples/orders/README.md). Run it:

```bash
cd examples/orders
go test ./...
go run ./cmd/demo
```

Then read the layer you are about to change ([domain](../layers/domain/README.md), [application](../layers/application/README.md), [infrastructure](../layers/infrastructure/README.md)) and its template.

## Establish Repository Truth

Before changing an existing service, inspect `go.mod` and the toolchain (`go version`), the Makefile targets, CI workflows, ADRs, generated-code rules, the package layout, and representative tests. The repository's checked-in decisions override generic advice here.

## Choose Proportional Architecture

| The change is... | Use |
|---|---|
| Several fields and events changing under one business rule | An aggregate |
| Validation, authorization, and persistence only | A usecase calling a repository; no aggregate ceremony |
| An optimized read | A query over a reader port |
| A library or CLI | Plain packages |

Do not manufacture an aggregate with no invariant, an interface used only by its implementation, a facade with no wiring benefit, an event for an implementation detail, or a "transaction" spanning separate stores on paper.

## Map Before You Edit

Trace the entry point through usecase or query, domain owner, repository or external port, transaction and outbox, and downstream consumers. Also check contracts and generated code, configuration and wiring, tenancy and security, retries and idempotency, observability, and tests. Trace one success path and the riskiest failure path.

## Verify

```bash
make lint
go test ./...
go test -race ./path/to/changed/concurrent/...
```

Add integration, E2E, and `govulncheck` in proportion to risk. Re-read the complete diff before finishing: wired, failure-safe, observable, compatible — not merely compiling.

## Next

- [New Service](new-service.md) or [Add a Feature](add-a-feature.md)
- [Testing](../testing/README.md)
- [Patterns](../patterns/README.md)
