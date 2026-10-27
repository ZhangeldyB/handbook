---
name: add-e2e-test
description: >-
  Add an end-to-end test that drives a running service through its generated
  gRPC client, with bounded per-operation contexts, cleanup, and polling for
  asynchronous results. Use only when the user wants coverage through the
  public boundary; direct usecase or repository tests are integration tests.
---

# Add an E2E Test

## Prerequisites

- A generated client and a runnable test deployment or local binary.
- An approved test endpoint, credentials, and tenant that cannot resolve to production.
- The existing `test/e2e/` suite setup, if any.

If the environment contract is missing, scaffold only what can be verified locally and report what is needed to run it. Never relabel a direct usecase test as E2E.

## Read First

1. `.handbook/testing/e2e-testing.md`.
2. Existing E2E tests and helpers in this repository.

## Steps

1. Choose a workflow that proves a consumer-visible risk lower levels cannot (authentication, mapping, wiring, a critical lifecycle, idempotency).
2. If absent, create `test/e2e/main_test.go` with `TestMain` building an immutable environment (client, credential-bearing base context without a deadline, per-RPC timeout) and `helpers_test.go` with `rpcContext`, create/cleanup helpers, and `eventually`.
3. Write the workflow as one test with dependent `t.Run` steps that stop on a failed prerequisite; register cleanup immediately after each create.
4. Assert public fields, enum values, and status codes; poll asynchronous results with a deadline.

## Guardrails

- Only the public client — no direct usecase, repository, or database calls.
- Unique, response-derived IDs; no order dependence; no shared mutable state.
- Every RPC and cleanup gets its own bounded context.
- No sleeps; no `t.Logf` instead of assertions.

## Verify

```bash
go test -c -o /tmp/e2e.test ./test/e2e   # compiles without running TestMain
E2E_ENDPOINT=... go test -count=1 ./test/e2e/...
```

## Done

- [ ] Public boundary only; risk-justified workflow.
- [ ] Bounded contexts, cleanup, unique data.
- [ ] Compiles; passes against the test environment (or what is missing is reported).
