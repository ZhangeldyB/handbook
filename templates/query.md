# Query Template

Working version: [queries.go](../examples/orders/internal/orders/queries/queries.go).

```go
package queries

const (
	DefaultPageSize = 50
	MaxPageSize     = 200
)

var ErrInvalidCursor = errors.New("invalid page token")

// <Entity>View is the read model. No rows, nullable driver types, or aggregates.
type <Entity>View struct {
	ID        string
	Status    string
	CreatedAt time.Time
}

type Position struct {
	CreatedAt time.Time `json:"c"`
	ID        string    `json:"i"`
}

type ListFilter struct {
	Status string
	After  *Position
	Limit  int
}

// <Entity>Reader is implemented by the persistence adapter.
type <Entity>Reader interface {
	Get<Entity>(ctx context.Context, tenantID, id string) (<Entity>View, error)
	List<Entities>(ctx context.Context, tenantID string, filter ListFilter) ([]<Entity>View, error)
}

type Queries struct{ reader <Entity>Reader }

func New(reader <Entity>Reader) (*Queries, error) {
	if err := deps.Require("<Entity>Reader", reader); err != nil {
		return nil, fmt.Errorf("new queries: %w", err)
	}
	return &Queries{reader: reader}, nil
}

func (q *Queries) List(ctx context.Context, req ListRequest) (ListResult, error) {
	limit := req.PageSize
	if limit <= 0 {
		limit = DefaultPageSize
	}
	limit = min(limit, MaxPageSize)
	// decode req.PageToken (bound to tenant and filter) → after
	// fetch limit+1 rows; if more than limit, trim and encode NextPageToken from the last row
}
```

The adapter implements `List<Entities>` as an index-backed keyset query ordered by `(created_at DESC, id ASC)` with the tenant first in the predicate.

Checklist: read-only; tenant-scoped; default and maximum page size; unique tie-breaker; tokens bound to tenant and filter; allowlisted filters and sorts; complete DTO mapping; tests for pagination without gaps or duplicates.
