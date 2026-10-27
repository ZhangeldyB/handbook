# Examples

Maintained Go modules that compile and pass their tests (`make verify-examples`). They are the source the templates and skills point at; when they disagree with a document, fix one of them immediately.

| Module | What it shows | Dependencies |
|---|---|---|
| [orders](orders/README.md) | A complete bounded context: aggregate with tracker, usecases with idempotency, queries with keyset pagination, repository with version checks and outbox, outbox relay with leases, event consumer, composition root | Standard library only |
| [grpctransport](grpctransport/README.md) | Transport-layer building blocks: authenticated principal in context, central error-to-status mapping interceptor, structured field-violation errors | `google.golang.org/grpc`, `google.golang.org/genproto` |
| [antipatterns](antipatterns/README.md) | Paired `Bad`/`Good` functions for recurring mistakes | Standard library only |

## Running

```bash
make verify-examples        # from the handbook root: vet + test every module
cd examples/orders && go run ./cmd/demo
```

## Why an In-Memory Store

The handbook is database-agnostic. `examples/orders` uses `internal/platform/memstore`, a small transactional key-value store with commit-failure injection, so the persistence contract (one transaction for state, outbox, and idempotency; acknowledgement after commit; version checks) is executable and tested without a database. A real adapter keeps the same contract with its own transaction API — see [Repositories](../layers/infrastructure/repositories.md#the-same-contract-in-sql).

## Maintenance Policy

- Examples show only the supported path. Superseded code is deleted, not archived.
- Every example package has tests; anti-pattern pairs compile and vet.
- Versions in `go.mod` are bumped as a reviewed change, with tests.
