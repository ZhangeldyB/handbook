# Queries

A query is a read-only application operation. It returns DTOs and performs no writes — not even "harmless" ones.

## Responsibilities

- Authorization and disclosure policy, tenant scope.
- Read orchestration and the consistency it promises.
- Bounded pagination, explicit filters, deterministic ordering.
- Row-to-DTO mapping and stable read errors.

Queries usually read a projection through a reader port instead of reconstituting aggregates. If a result needs live business rules rather than stored data, reconsider the model instead of duplicating rules in SQL.

## Reader Port and DTOs

The query package owns its port and DTOs; the persistence adapter implements them:

```go
type OrderView struct {
	OrderID, CustomerID, Status, Currency string
	TotalMinor                            int64
	CreatedAt                             time.Time
}

type OrderReader interface {
	GetOrder(ctx context.Context, tenantID, orderID string) (OrderView, error)
	ListOrders(ctx context.Context, tenantID string, filter ListFilter) ([]OrderView, error)
}
```

Never expose rows, nullable driver types, or aggregates to handlers.

## Pagination

Use **keyset (cursor) pagination** with a stable total order and a unique tie-breaker, here `(created_at DESC, order_id ASC)`:

```go
func (q *Queries) List(ctx context.Context, req ListRequest) (ListResult, error) {
	limit := req.PageSize
	if limit <= 0 {
		limit = DefaultPageSize
	}
	limit = min(limit, MaxPageSize)

	var after *Position
	if req.PageToken != "" {
		pos, err := decodeToken(req.PageToken, req.TenantID, req.Status)
		if err != nil {
			return ListResult{}, err
		}
		after = &pos
	}

	// Fetch one extra row to learn whether another page exists.
	views, err := q.reader.ListOrders(ctx, req.TenantID, ListFilter{Status: req.Status, After: after, Limit: limit + 1})
	if err != nil {
		return ListResult{}, fmt.Errorf("list orders: %w", err)
	}
	result := ListResult{Orders: views}
	if len(views) > limit {
		result.Orders = views[:limit]
		last := result.Orders[limit-1]
		result.NextPageToken = encodeToken(Position{CreatedAt: last.CreatedAt, OrderID: last.OrderID}, req.TenantID, req.Status)
	}
	return result, nil
}
```

- Every list has a default and a maximum page size, even when the caller omits one.
- Page tokens are opaque, bound to the tenant and filter they were issued for, and rejected (`InvalidArgument`) otherwise.
- The SQL equivalent is an index-backed keyset predicate: `WHERE tenant_id = $1 AND (created_at, order_id) < ($2, $3) ORDER BY created_at DESC, order_id ASC LIMIT $4` (adjust the comparison to your sort directions).
- Offset pagination is acceptable only for small, stable sets; it gets slow and skips or repeats rows under concurrent writes.
- Total counts are expensive and observe a different snapshot from the page; provide one only when the contract needs it, computed from the same predicate *without* the page limit.

## Filtering and Sorting

Map published filter and sort enums to an explicit allowlist of columns and operators. Never interpolate request strings into SQL; parameterize every value and validate its type. Inspect query plans for important high-volume queries rather than assuming an index is used.

## Consistency

Tell callers what they can rely on: same-database reads see committed writes; event-fed projections and caches are eventually consistent. If an API needs read-your-writes, return the command's result or version, or read the authoritative store.

## Caching

Cache at the read boundary only with an explicit key (tenant plus every result-shaping input), TTL or invalidation strategy, negative-cache behavior, stampede control, and bounded metrics. Never authorize a write or enforce an invariant from a cache.

## Testing

- tenant isolation and authorization;
- complete DTO mapping, including nullable and unknown values;
- filter and sort allowlists, invalid operators and values;
- pagination without duplicates or gaps, including ties ([example](../../examples/orders/internal/orders/queries/queries_test.go));
- page-size clamping and foreign or tampered tokens;
- not-found, empty, and cancellation behavior;
- integration tests with the real database for important query plans.
