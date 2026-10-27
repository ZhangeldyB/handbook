# Testing and CI

## Persistence and Lifecycle Tests

For every repository, prove ([example](../examples/orders/internal/orders/adapters/persistence/repository_test.go)):

- complete round-trip mapping, with a fixture that sets every snapshot field (checked by reflection);
- state and outbox commit together;
- a failed commit retains pending work and the retry writes the outbox exactly once; a further save is a no-op;
- a stale writer gets `ErrConcurrentModification`;
- a duplicate idempotency key rolls back the whole save;
- tenant isolation;
- `SaveAll` acknowledges nothing when the batch fails.

The example runs these against an in-memory transactional store. In a service, run the same tests against the real database too.

## Integration Tests With the Real Database

- Start the database in a container from the test (`TestMain` or a shared helper), or use a dedicated test instance; apply all migrations to an empty schema first.
- Give each test its own tenant ID or schema so tests can run in parallel safely.
- Cover constraints, isolation behavior, query plans for important queries, and the outbox claim query under concurrent claimers.
- Gate them behind an environment variable or build tag (`//go:build integration`) so `go test ./...` runs without infrastructure, and skip with a message naming the enabling command.
- Never make a test pass with hand-written rows that production code is not allowed to produce.

## Contract and Serialization Tests

Treat protobuf, JSON, event, and database mappings as compatibility code. For every material field cover: absent versus explicit zero, false, empty, and null; create defaults, update omission, explicit override, and re-read; required and optional validation with stable path, reason, and code; unknown enum values and fields; old-writer/new-reader and new-writer/old-reader; complete mapping and round trips. Keep checked-in fixtures for important historical encodings.

## Concurrency

- Use `t.Parallel()` only when data, globals, ports, and cleanup are isolated.
- Never call `t.Fatal` from a goroutine; report back to the test goroutine.
- `testing/synctest` for self-contained concurrent code driven by timers, channels, and contexts — not for real network or process I/O.
- Run `go test -race` on changed concurrent packages and representative integration paths. A green race run only covers the paths it executed. (The race detector needs cgo; CI images must provide it.)
- For packages that own goroutines, test start, stop, and wait; consider `go.uber.org/goleak` where it is stable.

## Fuzzing

Fuzz parsers, decoders, page tokens, and other untrusted structured input when invariants exist: never panic, bounded resource use, round trip where defined, rejection without partial mutation. Seed with valid and invalid inputs and keep every failure as a regression case.

## Time and Asynchrony

Inject clocks. For asynchronous integration and E2E behavior, poll an observable condition with a deadline and report the last observed state. Derive a fresh bounded context per operation; never share one expiring context across a suite.

## Coverage

Coverage is a discovery signal. Investigate untested high-risk branches; never write assertion-free tests to hit a number. Authorization, money, transactions, retries, parsers, and shutdown deserve explicit behavior tests regardless of line coverage.

## CI Pipeline

Fast, deterministic gates first; expensive environment gates later with retained diagnostics:

```text
1. golangci-lint run                    (formatters + linters, incl. depguard)
2. buf lint && buf breaking             (contract compatibility)
3. regenerate + git diff --exit-code    (generated code is current)
4. go test ./...                        (unit, usecase, transport)
5. go test -race ./...                  (where cgo is available)
6. migrations on an empty DB + integration tests
7. govulncheck ./...
8. build image + image scan
9. E2E against a deployed test environment
```

Never disable or broadly suppress a failing test, linter, or scanner to get a green build. A narrow exception needs evidence, an owner, and a review or expiry condition.

## Flaky Tests

Treat flakiness as a defect. Record the seed, timing, and environment; preserve logs; fix the ownership, isolation, or eventual-condition handling. Retries may gather evidence temporarily but must not hide the original failure or become permanent.

## Final Checklist

- [ ] Tests target observable contracts and meaningful failures.
- [ ] Deterministic time, IDs, and data.
- [ ] Rejected commands leave state unchanged.
- [ ] Failed saves retain pending work; retries are safe.
- [ ] Real mappings and transactions are integration-tested.
- [ ] Tenant, authorization, idempotency, cancellation, and shutdown are covered.
- [ ] Concurrent paths are bounded, owned, and race-tested.
- [ ] E2E coverage is focused.
