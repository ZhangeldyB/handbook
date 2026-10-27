---
name: add-grpc-handler
description: >-
  Add a gRPC RPC end to end on the transport side: proto definition, buf lint
  and generate, handler with structured validation, principal-derived identity,
  mapping, one application call, error-mapper rules, registration, and tests.
  Use when adding an API endpoint, implementing an RPC, or exposing a usecase
  or query over gRPC.
---

# Add a gRPC Handler

## Inputs

- Service and RPC name
- The usecase or query it calls (create it first with `implement-usecase` or `add-query`)

## Read First

1. `.handbook/.agents/rules/handler-rules.md` and `.handbook/.agents/rules/proto-rules.md`.
2. `.handbook/layers/infrastructure/grpc-handlers.md` and `.handbook/layers/infrastructure/proto-contracts.md`.
3. `.handbook/templates/grpc-handler.md`.
4. Compiled support packages: `.handbook/examples/grpctransport/{auth,grpcerr,validation}`.
5. Two existing handlers in this repository.

## Steps

1. Add the RPC and its own `<Rpc>Request`/`<Rpc>Response` messages to the `.proto`; document presence, defaults, units, and pagination limits in comments. No tenant field.
2. `buf lint`, `buf breaking --against '.git#branch=main'`, `make generate`.
3. Implement the handler: collect `validation.Violations` for wire input → `auth.FromContext` (else `Unauthenticated`) → map to the application request with the principal's tenant → call one operation → map the result. Return errors unchanged.
4. Add `mapping.go` functions for enums (both directions, explicit unknown) and timestamps/money.
5. Add mapper rules for any new client-visible domain or port error to the context's rule table, using the canonical table in `.handbook/layers/domain/domain-errors.md`.
6. Register the server in the composition root if it is new.

## Guardrails

- No repository, storage, provider, or domain-method calls in the handler.
- No per-handler error switches; no `err.Error()` in client messages.
- Never trust a tenant or actor ID from the request body.
- Never hand-edit generated code.

## Verify

Over `bufconn` with the real interceptor chain: missing principal → `Unauthenticated` with no application call; invalid input → `InvalidArgument` with the expected (field, reason) pairs and no application call; tenant comes from the principal; mapping including optional, zero, and unknown enum values; each new mapper rule's code.

```bash
go test ./internal/<context>/adapters/grpc/...
make lint
```

## Done

- [ ] Proto passes `buf lint` and `buf breaking`; generated code current.
- [ ] Handler does transport work only.
- [ ] Mapper rules added; registration done.
- [ ] Tests and lint pass.
