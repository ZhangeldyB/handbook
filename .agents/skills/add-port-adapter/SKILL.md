---
name: add-port-adapter
description: >-
  Add a consumer-owned port and an outbound adapter for an external capability
  (storage, email, rendering, a provider API, another service). Use for any
  new outbound dependency. For aggregate persistence use add-repository
  instead.
---

# Add a Port and Adapter

## Read First

1. `.handbook/.agents/rules/adapter-rules.md` and `.handbook/layers/application/ports.md`.
2. `.handbook/layers/infrastructure/grpc-clients.md` and `.handbook/templates/adapters.md`.
3. The dependency's real API: error contract, idempotency support, timeouts, concurrency safety, cleanup.
4. Confirm an interface is useful — a concrete dependency is simpler when there is no seam.

## Steps

1. Define the port in `ports/` (or the consuming package for a narrow seam), named by capability, with application types only and `ctx` first. Include tenant and idempotency key parameters when the effect needs them. Document retry behavior and which errors mean "outcome unknown".
2. Add port errors the adapter translates into (`ErrRejected`, `ErrTemporarilyUnavailable`, `ErrOutcomeUnknown`, ...).
3. Implement the adapter in `adapters/clients/`: map types, set deadlines from the caller's context, send idempotency keys, classify from status codes and typed errors, bound response sizes, never log secrets.
4. Construct it from a validated `Deps` value at the composition root; the shared client or connection is created there and passed in.
5. Assert `var _ ports.<Capability> = (*Adapter)(nil)`.

## Guardrails

- Never mirror an SDK in the interface; no SDK types cross inward.
- No goroutines in constructors.
- Retries only for idempotent operations, configured once (service config for gRPC).
- Correctness-critical effects go through durable intent (`add-reconciler`), not a direct call from a usecase.

## Verify

With `httptest.Server` or `bufconn`: success mapping; each error class; slow responses past the deadline → unknown; idempotency key sent; secrets absent from errors and logs; cleanup.

## Done

- [ ] Port contains only what the consumer calls.
- [ ] No infrastructure type crosses inward.
- [ ] Errors classified from typed evidence.
- [ ] Wired at the composition root; tests pass.
