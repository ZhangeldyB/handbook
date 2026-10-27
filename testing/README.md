# Testing

Tests provide evidence that contracts hold and failures are handled. Optimize for defect detection, determinism, diagnostics, and maintainability — not for a coverage percentage or a pyramid ratio.

| Document | Covers |
|---|---|
| [Unit Testing](unit-testing.md) | Domain and usecase tests, fakes, fixtures, error assertions |
| [Testing and CI](testing-ci.md) | Choosing test levels, integration tests, contract tests, concurrency, fuzzing, CI gates, flakiness |
| [E2E Testing](e2e-testing.md) | Public-boundary tests against a running service |

## Choose the Lowest Level That Proves the Behavior

| Level | Entry point | Proves |
|---|---|---|
| Unit | Aggregate, value object, pure function | Rules, transitions, edge cases |
| Usecase | `Execute` with fakes at ports | Orchestration, authorization, idempotency, failure handling |
| Adapter integration | Repository or client against the real dependency | Mapping, transactions, constraints, query behavior, protocol handling |
| Transport | Handler over `bufconn` | Identity, validation, mapping, status codes, interceptor order |
| E2E | Generated client against the running binary | Critical public workflows and production wiring |
| Fuzz | Parsers and decoders | Robustness against untrusted input |
| Race | Executed concurrent paths | Data races |

Most branches are cheap to test below E2E. Reserve E2E for composition risks a lower level cannot see.

## Rules That Apply Everywhere

- Test observable behavior and contracts, not private call order.
- A regression test fails before the fix and passes after it.
- Deterministic time, IDs, and randomness; isolated data; no sleeps for synchronization.
- Compare errors with `errors.Is`/`errors.As`/`status.Code`, never message substrings.
- Prefer the standard library plus `github.com/google/go-cmp/cmp` for diffs. Small hand-written fakes beat mocking frameworks.
- The default `go test ./...` passes with no infrastructure; tests that need a database or a running service skip with a message naming how to enable them.

The complete ruleset is the Testing section of [go-quality.md](../.agents/rules/go-quality.md).
