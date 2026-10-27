# gRPC and HTTP Clients

Outbound calls are adapters behind consumer-owned ports. The composition root owns connections; adapters own protocol translation, deadlines, and error classification.

## Connection Ownership

```go
conn, err := grpc.NewClient(cfg.CustomersAddr,
	grpc.WithTransportCredentials(creds),
	grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	grpc.WithChainUnaryInterceptor(auth.PropagatePrincipal()),
	grpc.WithDefaultServiceConfig(customersServiceConfig),
)
if err != nil {
	return fmt.Errorf("dial customers: %w", err)
}
defer conn.Close() // in run(); closed after every component that uses it has stopped
```

- Create one `*grpc.ClientConn` per target per process at the composition root and reuse it; connections are concurrency-safe and expensive to create.
- Use `grpc.NewClient` (not the deprecated `Dial` family).
- Credentials, interceptors, keepalive, and service config are set here, once — never inside individual adapters.
- The adapter receives only the generated client it uses (`customersv1.CustomerServiceClient`), wrapped behind a port.

## Deadlines

- On request paths, derive the call's deadline from the inbound context; add a tighter per-call timeout when the dependency's SLO is shorter than the caller's remaining budget.
- Never use `context.Background()` on a request path — it discards cancellation and the remaining budget.
- Background callers (jobs, consumers, relays) set an explicit timeout per call.

## Identity Propagation

Every inter-service call carries the caller's identity. A client interceptor copies the verified principal (or a service credential) from the context to outgoing metadata. Background work with no inbound request attaches an explicit system principal scoped to the tenant it is acting for. A missing identity fails at the callee with `Unauthenticated` — test new call sites.

## Retries

Retries are configured in the gRPC service config, not hand-written loops:

```go
const customersServiceConfig = `{
  "methodConfig": [{
    "name": [{"service": "acme.customers.v1.CustomerService", "method": "GetCustomer"}],
    "timeout": "2s",
    "retryPolicy": {
      "maxAttempts": 3,
      "initialBackoff": "0.1s",
      "maxBackoff": "1s",
      "backoffMultiplier": 2,
      "retryableStatusCodes": ["UNAVAILABLE"]
    }
  }]
}`
```

- Retry only idempotent methods (reads, or writes carrying an idempotency key the server honors).
- Never retry a non-idempotent command after an ambiguous failure (`DeadlineExceeded`, connection reset): the server may have applied it. Record the outcome as unknown and reconcile.

## Error Classification

The adapter translates protocol outcomes into port errors and a retry class:

```go
resp, err := c.client.GetCustomer(ctx, &customersv1.GetCustomerRequest{CustomerId: id})
switch status.Code(err) {
case codes.OK:
	return toCustomer(resp.GetCustomer())
case codes.NotFound:
	return Customer{}, ports.ErrCustomerNotFound
case codes.Unavailable, codes.ResourceExhausted:
	return Customer{}, fmt.Errorf("get customer %q: %w: %w", id, ports.ErrTemporarilyUnavailable, err)
default:
	return Customer{}, fmt.Errorf("get customer %q: %w", id, err)
}
```

Classify from status codes and typed errors, never from message text. Do not pass provider error text to your own clients.

## HTTP Providers

The same rules apply to HTTP APIs:

- One shared `*http.Client` per provider with explicit `Timeout` and transport settings; reuse connections.
- Requests built with `http.NewRequestWithContext`; always close response bodies (drain before closing when reusing connections).
- Map status codes explicitly: 4xx usually permanent (except 408/429), 5xx and network errors retryable, timeouts *unknown* for non-idempotent calls. Honor `Retry-After` for 429.
- Send the provider's idempotency key header on non-idempotent requests.
- Bound response size (`io.LimitReader`) before decoding.
- Never log credentials, tokens, authorization headers, or full response bodies.

## Tests

- Mapping of every relevant status code to port errors and retry classes.
- Deadlines propagate; cancellation stops the call.
- Identity metadata is attached.
- For providers: a fake server (`httptest.Server` or `bufconn`) that can return 429, 5xx, slow responses, and "applied but timed out".

See [Reconciliation](../../patterns/reconciliation.md) for effects whose outcome matters.
