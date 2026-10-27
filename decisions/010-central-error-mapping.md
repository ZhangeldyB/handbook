# ADR-010: Map Errors to gRPC Status in One Interceptor

**Status:** Accepted
**Date:** 2026-10-28

## Context

Per-handler error switches drift: the same domain error becomes different codes in different RPCs, internal messages leak to clients, and failures get logged at several layers.

## Decision

- Handlers return application and domain errors unchanged (or a deliberate `status` error for wire validation and authentication).
- Each service builds one `grpcerr.Mapper` from an ordered rule table (`Target` error → `Code` + static public message) and installs its interceptor. The canonical table is in [Domain Errors](../layers/domain/domain-errors.md#transport-mapping).
- `context.Canceled` and `context.DeadlineExceeded` map to `Canceled` and `DeadlineExceeded`. Unmatched errors become `Internal` with a generic message and are logged once, with the full error, at the interceptor.
- Public messages are static text from the rule, never `err.Error()`.

## Consequences

- **Easier:** consistent codes across RPCs; one place to audit what clients see; one log line per failure.
- **Harder:** every new domain error that callers should see needs a rule; a missing rule shows up as `Internal`, which tests and alerts catch.

## Enforcement

Mapper tests per rule ([example](../examples/grpctransport/grpcerr/grpcerr_test.go)); [handler rules](../.agents/rules/handler-rules.md) forbid per-handler switches.
