# gRPC Handlers

A gRPC handler is a transport adapter: it validates wire input, reads the authenticated principal, maps to an application request, calls one operation, and maps the result. Errors go back unchanged to a central mapper.

## Responsibilities

Handlers own protobuf request/response mapping, syntactic validation, principal → request identity, and passing `ctx` through. They do not own invariants, transitions, storage, provider orchestration, retries that could duplicate a write, or logging of returned errors.

"Thin" means a cohesive responsibility, not a line count.

## Command Handler

```go
type OrderServer struct {
	ordersv1.UnimplementedOrderServiceServer
	place placeCommand
}

type placeCommand interface {
	Execute(ctx context.Context, req usecases.PlaceRequest) error
}

func (s *OrderServer) PlaceOrder(ctx context.Context, req *ordersv1.PlaceOrderRequest) (*ordersv1.PlaceOrderResponse, error) {
	if req.GetOrderId() == "" {
		return nil, validation.Violations{{Field: "order_id", Reason: validation.Required, Description: "order_id is required"}}.Err()
	}
	principal, ok := auth.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	if err := s.place.Execute(ctx, usecases.PlaceRequest{
		TenantID: principal.TenantID,
		OrderID:  req.GetOrderId(),
	}); err != nil {
		return nil, err // the grpcerr interceptor maps it
	}
	return &ordersv1.PlaceOrderResponse{}, nil
}
```

The tenant comes from the principal, never from the request. A cross-tenant administrative RPC makes that explicit with its own authorization check and audit.

## Query Handler

```go
func (s *OrderServer) ListOrders(ctx context.Context, req *ordersv1.ListOrdersRequest) (*ordersv1.ListOrdersResponse, error) {
	principal, ok := auth.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	statusFilter, err := toStatusFilter(req.GetStatus()) // explicit enum mapping
	if err != nil {
		return nil, err
	}
	page, err := s.queries.List(ctx, queries.ListRequest{
		TenantID: principal.TenantID, Status: statusFilter,
		PageSize: int(req.GetPageSize()), PageToken: req.GetPageToken(),
	})
	if err != nil {
		return nil, err
	}
	resp := &ordersv1.ListOrdersResponse{
		Orders:        make([]*ordersv1.Order, 0, len(page.Orders)),
		NextPageToken: page.NextPageToken,
	}
	for _, view := range page.Orders {
		resp.Orders = append(resp.Orders, toProtoOrder(view))
	}
	return resp, nil
}
```

Keep mapping deterministic: timestamps via `timestamppb.New`, money as `google.type.Money` or minor units plus currency, enums mapped in both directions with an explicit unknown case.

## Structured Validation

For multi-field requests, walk every field and return all violations at once with `google.rpc.BadRequest` details ([example](../../examples/grpctransport/validation/validation.go)):

```go
func validateCreateOrder(req *ordersv1.CreateOrderRequest) error {
	var v validation.Violations
	if req.GetCustomerId() == "" {
		v.Add("customer_id", validation.Required, "customer_id is required")
	}
	if len(req.GetLines()) == 0 {
		v.Add("lines", validation.Required, "at least one line is required")
	}
	for i, line := range req.GetLines() {
		if line.GetQuantity() <= 0 {
			v.Add(validation.Index("lines", i)+".quantity", validation.OutOfRange, "quantity must be positive")
		}
	}
	return v.Err()
}
```

- Paths are stable wire paths (`lines[2].quantity`); reasons are a closed set that clients and tests branch on; descriptions are human remediation text and never echo secrets.
- Stop checks that depend on a failed prerequisite.
- Business rules ("cannot place an empty order") belong to the domain and come back as `FailedPrecondition`, not as handler validation. Do not duplicate them here.
- Do not build a tag-driven validation framework; an explicit walk is clearer. A schema-level validator such as `protovalidate` is an acceptable choice if adopted repository-wide.

## Error Mapping

One `grpcerr.Mapper` per service, installed as an interceptor, maps errors with the [canonical table](../domain/domain-errors.md#transport-mapping). Handlers never contain `switch errors.Is(...)` blocks. Public messages are static; unmapped errors become `Internal` and are logged once, with the full error, by the interceptor ([ADR-010](../../decisions/010-central-error-mapping.md)).

## Server Construction

```go
server := grpc.NewServer(
	grpc.StatsHandler(otelgrpc.NewServerHandler()),
	grpc.ChainUnaryInterceptor(
		recovery.UnaryServerInterceptor(),        // contain panics first
		authn.UnaryServerInterceptor(verifier),    // Principal into ctx
		mapper.UnaryServerInterceptor(logger),     // errors → status, log once
	),
	grpc.MaxRecvMsgSize(4<<20),
	grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{MinTime: 30 * time.Second}),
)
ordersv1.RegisterOrderServiceServer(server, orderServer)
healthpb.RegisterHealthServer(server, healthServer)
```

- Interceptor order is behavior: recovery outermost, authentication before anything that needs identity, error mapping inside so it sees handler errors.
- Choose message size limits and keepalive settings deliberately; do not copy large limits without a protocol need.
- Streaming handlers handle send/receive cancellation, backpressure, and partial progress explicitly.
- Register every service and the health service in the composition root; test that the final binary includes them.

## Compatibility

Protobuf changes are public API changes: never reuse field numbers, reserve removed fields, add fields compatibly, tolerate unknown enum values, and do not change status-code semantics by accident. See [Protobuf Contracts](proto-contracts.md).

## Tests

Use a real server over `bufconn` for registration and interceptor order, and focused tests for mapping:

- missing principal → `Unauthenticated`, and no application call;
- invalid input → `InvalidArgument` with the expected (field, reason) pairs, and no application call;
- tenant passed to the command comes from the principal;
- request and response mapping, including optional, zero, and unknown enum values;
- each mapped error produces its code; cancellation propagates.

Assert codes with `status.Code(err)` and violations by (field, reason) — never by message text.

## Review Checklist

- [ ] Only transport work; one application call.
- [ ] Identity from the principal.
- [ ] Every field mapped deliberately; presence preserved.
- [ ] Errors returned unchanged for the central mapper.
- [ ] No storage, provider, or domain-method calls.
- [ ] Tests cover identity, validation, mapping, and codes.
