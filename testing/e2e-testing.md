# End-to-End Testing

An E2E test calls a running, production-equivalent service through its public boundary — for gRPC, the generated client over the network:

```text
generated client → network + serialization → interceptors (auth, errors)
  → handler → usecase → domain → repository → real database
```

A test that calls a usecase directly, even with a real database, is a **component integration test**. It is valuable, but it does not prove transport wiring, interceptors, mapping, or status codes.

## What E2E Tests Are For

A small, risk-selected set:

- authentication, authorization, and tenant isolation at the public boundary;
- request validation, mapping, and stable status codes;
- production wiring (registration, interceptor order, configuration);
- a few critical business workflows end to end;
- consumer-visible idempotency (same idempotency key → same result);
- asynchronous convergence that is part of the public contract (an event-driven status change).

Do not enumerate domain branches or every field here; lower-level tests do that faster with better diagnostics. A universal "E2E every field" rule produces a slow, duplicated suite.

## Environment

- Start the real binary (or a production-equivalent deployment) with a controlled database.
- The test endpoint, credentials, and tenant come from explicit configuration; the suite refuses to run against anything that could be production.
- Use least-privilege test credentials and a unique run identity; never commit secrets.
- Retain service logs and diagnostics when CI fails.
- Skip only when a declared capability is unavailable; an unexpected RPC or assertion failure fails the test.

## Layout

```text
test/e2e/
  main_test.go          # TestMain: environment, client, immutable base context
  helpers_test.go       # rpcContext, create/delete helpers, eventually
  orders_lifecycle_test.go
  orders_idempotency_test.go
```

## Suite Setup

```go
var suite *environment // assigned once in TestMain, read-only afterwards

type environment struct {
	client  ordersv1.OrderServiceClient
	baseCtx context.Context // carries test credentials metadata; no deadline
	timeout time.Duration
	close   func() error
}

func TestMain(m *testing.M) {
	env, err := newEnvironment()
	if err != nil {
		fmt.Fprintf(os.Stderr, "set up e2e environment: %v\n", err)
		os.Exit(1)
	}
	suite = env
	code := m.Run()
	if err := env.close(); err != nil && code == 0 {
		fmt.Fprintf(os.Stderr, "close e2e environment: %v\n", err)
		code = 1
	}
	os.Exit(code) // deferred calls would not run after this
}

// rpcContext gives every RPC and cleanup its own deadline.
func rpcContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(suite.baseCtx, suite.timeout)
}
```

Share only immutable, concurrency-safe state. Never create one expiring context for the whole suite.

## Workflow Pattern

```go
func TestOrderLifecycle(t *testing.T) {
	orderID := createOrder(t)
	t.Cleanup(func() { cancelOrder(t, orderID) })

	if !t.Run("place", func(t *testing.T) {
		ctx, cancel := rpcContext(t)
		defer cancel()
		if _, err := suite.client.PlaceOrder(ctx, &ordersv1.PlaceOrderRequest{OrderId: orderID}); err != nil {
			t.Fatalf("PlaceOrder() error = %v", err)
		}
	}) {
		return // later steps depend on this one
	}

	t.Run("observe paid", func(t *testing.T) {
		eventually(t, 30*time.Second, func(ctx context.Context) (bool, error) {
			resp, err := suite.client.GetOrder(ctx, &ordersv1.GetOrderRequest{OrderId: orderID})
			if err != nil {
				return false, err
			}
			return resp.GetOrder().GetStatus() == ordersv1.OrderStatus_ORDER_STATUS_PAID, nil
		})
	})
}
```

- Register cleanup immediately after creating each resource. Cleanup uses a fresh bounded context and reports unexpected errors; it tolerates `NotFound` only when deletion is idempotent by contract.
- Dependent steps live in one test and stop after a failed prerequisite (`if !t.Run(...) { return }`).
- Independent tests never depend on execution order.
- IDs come from API responses, never hard-coded; names include a unique run suffix.

## Assertions

Assert public behavior: response fields, stable enum values, status codes (`status.Code(err)`), validation details by (field, reason), tenant isolation, and the absence of duplicate effects. Use `t.Fatalf` when later checks depend on the result and `t.Errorf` for independent field mismatches. Poll asynchronous results with a deadline and report the last observed state — never sleep. Do not reach into the database for ordinary setup or verification.

## Running

```bash
go test -c -o /tmp/orders-e2e.test ./test/e2e   # compile check; does not run TestMain
E2E_ENDPOINT=localhost:8080 go test -count=1 ./test/e2e/...
```

Wrap them in `make test-e2e`. CI enforces an overall timeout, keeps diagnostics, and makes production targeting impossible.

## Review Checklist

- [ ] Enters through the public boundary.
- [ ] Proves a consumer-visible risk lower levels cannot cover.
- [ ] Environment and tenant are approved for test data; production is unreachable.
- [ ] Every operation and cleanup has its own deadline.
- [ ] Resources are cleaned up; cleanup errors are checked.
- [ ] No order dependence, no hard-coded IDs, no sleeps.
- [ ] Assertions use public fields and codes.
