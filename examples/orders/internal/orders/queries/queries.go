// Package queries holds read-only operations for the orders context. Queries
// read projections through a reader port and return DTOs; they never load
// aggregates for mutation and never write.
package queries

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"example.com/orders/internal/platform/deps"
)

// Page size bounds. Every list is bounded even when the caller omits a size.
const (
	DefaultPageSize = 50
	MaxPageSize     = 200
)

// ErrInvalidCursor means a page token is malformed or was issued for another
// tenant or filter. Tokens are opaque but not secret: they only ever select
// rows inside the caller's authenticated tenant.
var ErrInvalidCursor = errors.New("invalid page token")

// OrderView is the read model returned to callers.
type OrderView struct {
	OrderID    string
	CustomerID string
	Status     string
	TotalMinor int64
	Currency   string
	CreatedAt  time.Time
}

// Position is a stable sort position: newest first, ties broken by ID.
type Position struct {
	CreatedAt time.Time `json:"c"`
	OrderID   string    `json:"o"`
}

// ListFilter selects orders for one tenant.
type ListFilter struct {
	Status string    // optional exact match
	After  *Position // exclusive start position; nil for the first page
	Limit  int       // already clamped by the query
}

// OrderReader is the read port implemented by the persistence adapter.
type OrderReader interface {
	// GetOrder returns ports.ErrNotFound when the order does not exist.
	GetOrder(ctx context.Context, tenantID, orderID string) (OrderView, error)
	// ListOrders returns at most filter.Limit orders sorted by CreatedAt
	// descending, then OrderID ascending, strictly after filter.After.
	ListOrders(ctx context.Context, tenantID string, filter ListFilter) ([]OrderView, error)
}

// Queries groups the read operations.
type Queries struct {
	reader OrderReader
}

// New validates its dependency once.
func New(reader OrderReader) (*Queries, error) {
	if err := deps.Require("OrderReader", reader); err != nil {
		return nil, fmt.Errorf("new queries: %w", err)
	}
	return &Queries{reader: reader}, nil
}

// Get returns one order.
func (q *Queries) Get(ctx context.Context, tenantID, orderID string) (OrderView, error) {
	view, err := q.reader.GetOrder(ctx, tenantID, orderID)
	if err != nil {
		return OrderView{}, fmt.Errorf("get order %q: %w", orderID, err)
	}
	return view, nil
}

// ListRequest is the application input for listing orders.
type ListRequest struct {
	TenantID  string
	Status    string
	PageSize  int
	PageToken string
}

// ListResult is one page of orders.
type ListResult struct {
	Orders        []OrderView
	NextPageToken string // empty on the last page
}

// List returns one bounded page of orders.
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

// token binds a position to the tenant and filter it was issued for, so a
// token cannot be replayed against another tenant or a different filter.
type token struct {
	Position
	Tenant string `json:"t"`
	Status string `json:"s"`
}

func encodeToken(pos Position, tenantID, status string) string {
	raw, _ := json.Marshal(token{Position: pos, Tenant: tenantID, Status: status})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeToken(s, tenantID, status string) (Position, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Position{}, ErrInvalidCursor
	}
	var t token
	if err := json.Unmarshal(raw, &t); err != nil || t.Tenant != tenantID || t.Status != status || t.OrderID == "" {
		return Position{}, ErrInvalidCursor
	}
	return t.Position, nil
}
