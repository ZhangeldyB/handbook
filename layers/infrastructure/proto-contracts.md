# Protobuf Contracts

Protobuf files define the public API and, optionally, event payloads. They are managed with [buf](https://buf.build): one module per repository, linted and checked for breaking changes in CI.

## Layout

```text
proto/
└── acme/
    └── orders/
        └── v1/
            ├── orders.proto        # OrderService + request/response messages
            ├── order.proto         # shared resource messages and enums
            └── events.proto        # event payloads, if events are protobuf
buf.yaml
buf.gen.yaml
gen/                                 # generated Go code — never edited
```

The package path matches the directory (`package acme.orders.v1;`). Replace `acme` with your organization. Files do not declare `go_package`: buf managed mode derives it from the path (`example.com/orders/gen/acme/orders/v1`, Go package `ordersv1`).

Starter configs: [buf.yaml](../../templates/buf.yaml), [buf.gen.yaml](../../templates/buf.gen.yaml).

## Service Definition

```protobuf
syntax = "proto3";

package acme.orders.v1;

import "google/protobuf/timestamp.proto";
import "google/type/money.proto";

// OrderService manages customer orders. Every RPC is scoped to the caller's
// tenant, derived from authentication; requests never carry a tenant ID.
service OrderService {
  // CreateOrder creates a draft order.
  rpc CreateOrder(CreateOrderRequest) returns (CreateOrderResponse);
  // GetOrder returns one order.
  rpc GetOrder(GetOrderRequest) returns (GetOrderResponse);
  // ListOrders returns one page of orders, newest first.
  rpc ListOrders(ListOrdersRequest) returns (ListOrdersResponse);
  // PlaceOrder submits a draft order. FAILED_PRECONDITION when not a draft.
  rpc PlaceOrder(PlaceOrderRequest) returns (PlaceOrderResponse);
  // CancelOrder cancels a draft or placed order. Cancelling twice succeeds.
  rpc CancelOrder(CancelOrderRequest) returns (CancelOrderResponse);
}

message CreateOrderRequest {
  string customer_id = 1;
  // ISO 4217 code; every line price uses it.
  string currency_code = 2;
  repeated OrderLineInput lines = 3;
  // Client-generated key; retries with the same key return the original
  // order. Reusing a key with a different request is INVALID_ARGUMENT.
  string idempotency_key = 4;
}

message OrderLineInput {
  string sku = 1;
  // Must be positive.
  int64 quantity = 2;
  // Price of one unit in minor units of the order currency.
  int64 unit_price_minor = 3;
}

message CreateOrderResponse {
  string order_id = 1;
}

message GetOrderRequest {
  string order_id = 1;
}

message GetOrderResponse {
  Order order = 1;
}

message PlaceOrderRequest {
  string order_id = 1;
}

message PlaceOrderResponse {}

message CancelOrderRequest {
  string order_id = 1;
  // Free text shown to operators; at most 500 characters.
  string reason = 2;
}

message CancelOrderResponse {}

message ListOrdersRequest {
  // Optional filter; ORDER_STATUS_UNSPECIFIED means all statuses.
  OrderStatus status = 1;
  // Defaults to 50, maximum 200.
  int32 page_size = 2;
  string page_token = 3;
}

message ListOrdersResponse {
  repeated Order orders = 1;
  // Empty on the last page.
  string next_page_token = 2;
}

message Order {
  string order_id = 1;
  string customer_id = 2;
  OrderStatus status = 3;
  google.type.Money total = 4;
  google.protobuf.Timestamp create_time = 5;
}

enum OrderStatus {
  ORDER_STATUS_UNSPECIFIED = 0;
  ORDER_STATUS_DRAFT = 1;
  ORDER_STATUS_PLACED = 2;
  ORDER_STATUS_PAID = 3;
  ORDER_STATUS_CANCELLED = 4;
}
```

## Naming

| Element | Convention |
|---|---|
| Package | `<org>.<context>.v<N>` |
| Service | `<Resource>Service` |
| RPC | Verb + resource: `CreateOrder`, `GetOrder`, `ListOrders`, plus domain verbs `PlaceOrder`, `CancelOrder` |
| Messages | `<Rpc>Request` / `<Rpc>Response`, unique per RPC — never shared |
| Fields | `lower_snake_case`; repeated fields plural; booleans without `is_` |
| Enums | `PascalCase` type; values prefixed with the enum name; `_UNSPECIFIED = 0` carries no meaning |
| Time fields | `<name>_time` (`create_time`) as `google.protobuf.Timestamp` |

`buf lint` (STANDARD) enforces most of this.

## Field Contract Before Syntax

Decide the behavior of every field before choosing its type:

- meaning, owner, and tenant/security policy;
- required, optional, or computed; whether absent differs from zero, false, or empty;
- default and where it is applied; units and bounds;
- create-only, mutable, immutable, or server-managed;
- how old readers and writers behave.

Use `optional` when presence changes meaning. Prefer explicit update RPCs or update messages with `optional` fields over `google.protobuf.FieldMask` unless generic partial updates are needed. Status changes get their own RPCs (`PlaceOrder`), not a writable `status` field.

## Types

- Time: `google.protobuf.Timestamp`; durations: `google.protobuf.Duration`.
- Money: `google.type.Money` (units + nanos, exact) or `int64 amount_minor` plus `string currency_code` — never `double`.
- IDs: `string`. Do not encode meaning in ID formats clients must parse.
- Map domain enums to protobuf enums explicitly in the adapter, in both directions, with an error for unknown values.

## Pagination

`page_size` + opaque `page_token` → `next_page_token`. Document default and maximum sizes; reject tampered or foreign tokens with `INVALID_ARGUMENT`. See [Queries](../application/queries.md#pagination).

## Evolution

- Never renumber or reuse a field number; never change a field's type or cardinality.
- Reserve removed numbers and names: `reserved 6; reserved "legacy_note";`.
- Add enum values without renumbering; every consumer must handle values it does not know.
- New `oneof` variants need safe behavior from old consumers.
- Changing presence, a default, or meaning is a breaking contract change even if the wire format decodes — add a new field or a new package version.
- Breaking changes go into `v2`, served alongside `v1` until consumers migrate.

`buf breaking --against '.git#branch=main'` catches wire-level breaks; semantic breaks need review.

## Generation

```bash
buf lint
buf breaking --against '.git#branch=main'
buf generate
```

- Pin plugin versions in `buf.gen.yaml`; regeneration must be reproducible and a second run must produce no diff.
- Commit generated code under `gen/` (or generate in CI — choose once per repository).
- Never hand-edit generated files; change the `.proto` and regenerate.
- Use generated getters (`req.GetOrderId()`) in Go: they are nil-safe.

## Events as Protobuf

If events are protobuf, define payloads in `events.proto` with an envelope carrying `event_id`, `tenant_id`, `aggregate_id`, and `occur_time`, and version topics alongside packages (`orders.order_placed.v1`). The same evolution rules apply, plus replay compatibility for the retention window.

Rules: [proto-rules.md](../../.agents/rules/proto-rules.md). Evolution process: [API Contract Evolution](../../guides/api-contract-evolution.md).
