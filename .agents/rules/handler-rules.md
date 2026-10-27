---
description: Rules for gRPC transport handlers and interceptors
paths:
  - "**/adapters/grpc/**"
---

# gRPC Handler Rules

Handlers own transport behavior only: trusted identity, wire validation, request/response mapping, one application call, and returning errors for the central mapper.

## Boundaries

- No business transitions, workflow retries, repository or database access, provider SDK calls, or domain method calls in handlers.
- Read identity with `auth.FromContext(ctx)`; return `codes.Unauthenticated` when absent. Never trust a tenant or actor ID from the request body unless the RPC is explicitly a cross-tenant administrative API with its own authorization check.
- Pass `ctx` to the application call unchanged.
- Return application/domain errors **unchanged**. One service-wide `grpcerr.Mapper` interceptor translates them using the canonical code table, sanitizes messages, and logs unexpected failures once. Do not write per-handler error switches.
- Log only at the interceptor boundary. Validation and not-found results are not error logs.

## Shape

1. Reject malformed wire input; for multi-field requests collect every violation with `validation.Violations` and return `InvalidArgument` with `BadRequest` field paths.
2. Get the principal.
3. Map the proto request to an application request, preserving absent-versus-zero meaning (`optional` fields, `Has…`/`Get…` accessors).
4. Call one command or query.
5. Map the result to the response message.

Use the generated getters (`req.GetOrderId()`), which are nil-safe. Split handler files by RPC when it improves navigation; keep a small service in one file. No arbitrary line quotas.

## Mapping and Compatibility

- Account for every request and response field; do not zero-fill a field to silence a linter — trace whether it is absent, intentionally zero, or missing upstream.
- Map enums explicitly in both directions; reject `*_UNSPECIFIED` where a value is required and handle unknown values deliberately.
- Use `google.protobuf.Timestamp` via `timestamppb`, money via `google.type.Money` or minor units plus currency — never `double`.
- Treat field presence, status-code, and proto changes as public compatibility decisions. Never hand-edit generated code.

## Server Configuration

- Interceptor order is behavior: recovery → authentication → error mapping → (telemetry via `otelgrpc` stats handler). Test the order.
- Set explicit receive/send size limits, keepalive enforcement, and graceful shutdown with a bounded `Stop` fallback.
- Register the health service and reflect real readiness.

## Verification

- Test nil/malformed input, missing principal, tenant propagation from the principal, mapping including optional/zero/enum-unknown values, and status codes for each mapped error.
- Assert no application call happens after an authentication or validation failure.
- Integration-test interceptor order, registration, and size limits through a real server (`bufconn`).

See `.handbook/layers/infrastructure/grpc-handlers.md` and `.handbook/templates/grpc-handler.md`.
