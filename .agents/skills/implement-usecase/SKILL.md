---
name: implement-usecase
description: >-
  Implement an application command (usecase) with validated dependencies,
  domain orchestration, persistence through the repository, optional
  idempotency, and tests with fakes. Use when adding a write operation,
  command, or application workflow.
---

# Implement a Usecase

## Inputs

- Bounded context, operation name, aggregate(s) involved
- Whether clients retry it or it is event-driven (decides idempotency)
- Whether it triggers an external effect (decides durable intent + reconciliation)

## Read First

1. `.handbook/.agents/rules/usecase-rules.md`.
2. `.handbook/layers/application/usecases.md` and `.handbook/templates/usecase.md`.
3. `.handbook/patterns/idempotency.md` when the selection rule applies.
4. Compiled examples: `.handbook/examples/orders/internal/orders/usecases/` (`place.go`, `create_draft.go`, `mark_paid.go`) and `usecases_test.go`.
5. Callers that will invoke it (handler, consumer, job) and the existing usecases in this context.

## Steps

1. Trace the entry point, the domain method(s), the repository, and every side effect; identify the riskiest failure path.
2. Create `usecases/<operation>.go` (flat) or `usecases/<entity>/<operation>.go` (large contexts) — never a package per operation.
3. Define `<Operation>Deps` with only the collaborators it calls; `New<Operation>` validates with `errors.Join(deps.Require(...)...)`.
4. Define `<Operation>Request` (tenant from the principal, filled by the handler) and a `<Operation>Result` only if the caller needs data back.
5. `Execute(ctx, req)`: load through the port → call the domain with injected time and IDs → `Save` (or `SaveAll`).
6. Add idempotency per the selection rule: client-retried creates use key + fingerprint + replay + race-loser recovery; consumer-driven commands pass `IdempotencyRecord{Scope: "consume:<topic>", Key: EventID}` and treat `ErrDuplicate` as success.
7. External effect: persist the intent with the state change; leave the provider call to a worker (`add-reconciler`).
8. Wire it in the composition root and the caller.

## Guardrails

- No adapter, driver, or protobuf imports (`depguard`).
- No transactions, outbox writes, or tracker calls in the usecase.
- No request-path nil checks for required dependencies.
- Do not import another usecase; do not log and return.

## Verify

Constructor rejects each missing dependency (including a typed nil); success; domain rejection; `ports.ErrNotFound`; save failure keeps pending work; idempotent replay, key reuse, and race-loser recovery where applicable; duplicate events are no-ops; cancellation.

```bash
go test ./internal/<context>/usecases/...
make lint
```

## Done

- [ ] One operation, one file, operation-named type.
- [ ] Focused `Deps` validated once; time and IDs injected.
- [ ] Persistence only through `Save`/`SaveAll`.
- [ ] Idempotency and external-effect handling match the selection rules.
- [ ] Wired into its caller and the composition root.
- [ ] Tests and lint pass.
