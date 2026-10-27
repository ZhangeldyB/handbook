# Enums Across Boundaries

An enum crosses three representations: the domain type, the stored value, and the wire value. Each boundary maps explicitly; none of them leaks into another layer.

## Domain Enum

```go
// Status is the order lifecycle state. The string values are the stored and
// published representation and must never be renamed.
type Status string

const (
	StatusDraft     Status = "DRAFT"
	StatusPlaced    Status = "PLACED"
	StatusPaid      Status = "PAID"
	StatusCancelled Status = "CANCELLED"
)

func (s Status) Valid() bool {
	switch s {
	case StatusDraft, StatusPlaced, StatusPaid, StatusCancelled:
		return true
	default:
		return false
	}
}

// ParseStatus rejects unknown values instead of guessing.
func ParseStatus(value string) (Status, error) {
	s := Status(value)
	if !s.Valid() {
		return "", fmt.Errorf("parse status %q: %w", value, ErrInvalidOrder)
	}
	return s, nil
}
```

- The zero value (`""`) is invalid.
- String values are stable identifiers: readable in the database and logs, safe to reorder in code. Renaming one is a data migration.
- Integer-based enums (`iota + 1`, so zero is invalid) are fine for purely in-memory states that are never stored.
- The `exhaustive` linter requires switches over the type to handle every value (or a `default`).

## Storage

Store the domain string value. Read through `ParseStatus` so corrupt or future values fail loudly instead of becoming a default.

## Protobuf

The proto enum is a separate contract with its own zero value:

```protobuf
enum OrderStatus {
  ORDER_STATUS_UNSPECIFIED = 0;
  ORDER_STATUS_DRAFT = 1;
  ORDER_STATUS_PLACED = 2;
  ORDER_STATUS_PAID = 3;
  ORDER_STATUS_CANCELLED = 4;
}
```

Map in the transport adapter, in both directions, with an explicit unknown case:

```go
func toProtoStatus(s domain.Status) ordersv1.OrderStatus {
	switch s {
	case domain.StatusDraft:
		return ordersv1.OrderStatus_ORDER_STATUS_DRAFT
	case domain.StatusPlaced:
		return ordersv1.OrderStatus_ORDER_STATUS_PLACED
	case domain.StatusPaid:
		return ordersv1.OrderStatus_ORDER_STATUS_PAID
	case domain.StatusCancelled:
		return ordersv1.OrderStatus_ORDER_STATUS_CANCELLED
	default:
		return ordersv1.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}
}

func fromProtoStatus(s ordersv1.OrderStatus) (domain.Status, error) {
	switch s {
	case ordersv1.OrderStatus_ORDER_STATUS_DRAFT:
		return domain.StatusDraft, nil
	case ordersv1.OrderStatus_ORDER_STATUS_PLACED:
		return domain.StatusPlaced, nil
	case ordersv1.OrderStatus_ORDER_STATUS_PAID:
		return domain.StatusPaid, nil
	case ordersv1.OrderStatus_ORDER_STATUS_CANCELLED:
		return domain.StatusCancelled, nil
	default:
		return "", status.Errorf(codes.InvalidArgument, "unsupported order status %d", s)
	}
}
```

- Never alias the generated enum into the domain; the domain must not depend on a wire contract or its generator.
- Never use the generated `String()` output as a storage value.
- Test that every domain value maps to a distinct proto value and back; a test iterating all `OrderStatus_value` entries catches additions on either side.

## Events

Published payloads use the domain's stable string values (JSON) or the proto enum (protobuf). Consumers handle values they do not know without crashing — usually by treating the event as permanent-for-this-version or ignoring the field, as their contract defines.

## Transition Tables

Validate transitions against one table keyed by the domain type — see the [State Machine template](../templates/state-machine.md). Never compare enum strings in handlers or SQL to decide a transition.
