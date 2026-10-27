---
description: Rules for protobuf contracts and buf configuration
paths:
  - "**/*.proto"
  - "**/buf.yaml"
  - "**/buf.gen.yaml"
---

# Protobuf Contract Rules

Protobuf files are public contracts. Every change is reviewed for old/new producer and consumer overlap.

## Layout and Tooling

- Sources live under `proto/<package path>/<version>/`, for example `proto/acme/orders/v1/orders.proto` with `package acme.orders.v1;`.
- `buf lint` (STANDARD rules) and `buf breaking --against` the main branch run in CI and must pass. Generation uses pinned plugin versions in `buf.gen.yaml`; generated code is never edited.

## Naming

- Services `OrderService`; RPCs are verbs (`CreateOrder`, `GetOrder`, `ListOrders`, `PlaceOrder`); every RPC has its own `<Rpc>Request` and `<Rpc>Response` messages, never shared.
- Fields `lower_snake_case`; repeated fields plural; booleans without `is_` prefix.
- Enums `PascalCase`; values `UPPER_SNAKE_CASE` prefixed with the enum name; zero is `<ENUM>_UNSPECIFIED` and never carries meaning.

## Field Semantics

- Use `optional` when absence differs from the zero value (updates, flags with three states). Do not overload a scalar to mean absent, disabled, and invalid.
- Prefer explicit update messages with `optional` fields over `google.protobuf.FieldMask` unless partial-update generality is genuinely needed; document whichever you choose.
- Times: `google.protobuf.Timestamp`; durations: `google.protobuf.Duration`; money: `google.type.Money` (or `int64` minor units plus `string currency_code`) — never `double`/`float`.
- Lists are paginated with `page_size` and opaque `page_token` → `next_page_token`; document default and maximum sizes.
- Document required/optional, defaults, units, mutability, and tenant ownership in a comment when the schema cannot express it.

## Evolution

- Never renumber, reuse, or change the type/cardinality of a field. Reserve removed numbers **and** names (`reserved 4; reserved "legacy_id";`).
- Add enum values without renumbering; every consumer handles unknown values. A new `oneof` variant needs safe behavior from old consumers.
- Changing meaning, presence, or a default is a contract change even when the wire format still decodes — add a new field or version instead.
- An incompatible change gets a new package version (`v2`) served alongside `v1` until consumers migrate.

## Events

- Event payloads are versioned messages (`OrderPlacedV1` or a `v1` package) carrying event ID, tenant ID, aggregate ID, and occurrence time. Topic names are contracts (`orders.order_placed.v1`).

See `.handbook/layers/infrastructure/proto-contracts.md` and `.handbook/guides/api-contract-evolution.md`.
