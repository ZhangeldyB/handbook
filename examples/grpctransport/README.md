# gRPC Transport Building Blocks

Small, tested packages to copy into `internal/platform/` of a service.

```bash
go test ./...
```

| Package | Purpose | Docs |
|---|---|---|
| `auth` | Carry the authenticated `Principal` (tenant, subject, scopes) in the request context | [gRPC Handlers](../../layers/infrastructure/grpc-handlers.md) |
| `grpcerr` | One ordered rule table maps application errors to gRPC codes with static public messages; the interceptor logs unmapped errors once and returns a sanitized `Internal` | [ADR-010](../../decisions/010-central-error-mapping.md), [Domain Errors](../../layers/domain/domain-errors.md#transport-mapping) |
| `validation` | Collect every field violation and return `InvalidArgument` with `google.rpc.BadRequest` details | [Structured Validation](../../layers/infrastructure/grpc-handlers.md#structured-validation) |

The authentication interceptor itself (verifying tokens and calling `auth.WithPrincipal`) depends on your identity provider and is not included.
