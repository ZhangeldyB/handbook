package queries_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"example.com/orders/internal/orders/adapters/persistence"
	"example.com/orders/internal/orders/domain"
	"example.com/orders/internal/orders/queries"
	"example.com/orders/internal/platform/memstore"
)

func seed(t *testing.T, tenantID string, n int) *queries.Queries {
	t.Helper()
	repo, err := persistence.New(memstore.New())
	if err != nil {
		t.Fatalf("persistence.New() error = %v", err)
	}
	base := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)
	for i := range n {
		// Pairs of orders share a creation time to exercise the ID tie-breaker.
		created := base.Add(time.Duration(i/2) * time.Minute)
		order, err := domain.NewOrder(domain.NewOrderParams{
			TenantID: tenantID, OrderID: fmt.Sprintf("order-%02d", i), CustomerID: "c", Currency: "EUR", Now: created,
		})
		if err != nil {
			t.Fatalf("NewOrder() error = %v", err)
		}
		if err := repo.Save(t.Context(), order); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}
	q, err := queries.New(repo)
	if err != nil {
		t.Fatalf("queries.New() error = %v", err)
	}
	return q
}

func TestListPaginatesWithoutDuplicatesOrGaps(t *testing.T) {
	q := seed(t, "tenant-1", 7)

	seen := map[string]bool{}
	token := ""
	pages := 0
	for {
		page, err := q.List(t.Context(), queries.ListRequest{TenantID: "tenant-1", PageSize: 3, PageToken: token})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		pages++
		for _, order := range page.Orders {
			if seen[order.OrderID] {
				t.Fatalf("order %s returned twice", order.OrderID)
			}
			seen[order.OrderID] = true
		}
		if page.NextPageToken == "" {
			break
		}
		token = page.NextPageToken
	}

	if len(seen) != 7 || pages != 3 {
		t.Fatalf("saw %d orders in %d pages, want 7 in 3", len(seen), pages)
	}
}

func TestListRejectsTokenFromAnotherTenant(t *testing.T) {
	q := seed(t, "tenant-1", 3)
	page, err := q.List(t.Context(), queries.ListRequest{TenantID: "tenant-1", PageSize: 1})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	_, err = q.List(t.Context(), queries.ListRequest{TenantID: "tenant-2", PageSize: 1, PageToken: page.NextPageToken})

	if !errors.Is(err, queries.ErrInvalidCursor) {
		t.Fatalf("List() error = %v, want %v", err, queries.ErrInvalidCursor)
	}
}

func TestListClampsPageSize(t *testing.T) {
	q := seed(t, "tenant-1", 3)

	page, err := q.List(t.Context(), queries.ListRequest{TenantID: "tenant-1", PageSize: queries.MaxPageSize + 1000})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(page.Orders) != 3 || page.NextPageToken != "" {
		t.Fatalf("got %d orders, token %q; want all 3 and no next page", len(page.Orders), page.NextPageToken)
	}
}
