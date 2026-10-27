// Package persistence implements the orders ports on top of a transactional
// store. It owns row mapping, version checks, outbox writes, and translation of
// storage errors into port errors.
package persistence

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"example.com/orders/internal/orders/domain"
	"example.com/orders/internal/orders/ports"
	"example.com/orders/internal/orders/queries"
	"example.com/orders/internal/platform/aggregate"
	"example.com/orders/internal/platform/deps"
	"example.com/orders/internal/platform/memstore"
	"example.com/orders/internal/platform/outbox"
)

const (
	ordersTable      = "orders"
	idempotencyTable = "idempotency"
)

// Repository stores orders. It implements ports.OrderRepository,
// ports.IdempotencyStore, and queries.OrderReader.
type Repository struct {
	db *memstore.DB
}

var (
	_ ports.OrderRepository  = (*Repository)(nil)
	_ ports.IdempotencyStore = (*Repository)(nil)
	_ queries.OrderReader    = (*Repository)(nil)
)

// New validates its dependency once.
func New(db *memstore.DB) (*Repository, error) {
	if err := deps.Require("DB", db); err != nil {
		return nil, fmt.Errorf("new order repository: %w", err)
	}
	return &Repository{db: db}, nil
}

// Get loads an order. It returns ports.ErrNotFound when the tenant has no such
// order; another tenant's order with the same ID is never visible.
func (r *Repository) Get(ctx context.Context, tenantID, orderID string) (*domain.Order, error) {
	var row orderRow
	err := r.db.InTx(ctx, func(tx *memstore.Tx) error {
		return tx.Get(ordersTable, orderKey(tenantID, orderID), &row)
	})
	if errors.Is(err, memstore.ErrNotFound) {
		return nil, ports.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load order row: %w", err)
	}
	snapshot, err := toSnapshot(row)
	if err != nil {
		return nil, err
	}
	return domain.Reconstitute(snapshot)
}

// Save implements ports.OrderRepository. State, outbox records, and
// idempotency records commit in one transaction; the order's pending work is
// acknowledged only after that commit succeeds.
func (r *Repository) Save(ctx context.Context, order *domain.Order, records ...ports.IdempotencyRecord) error {
	pending, ack, dirty := order.BeginPersist()
	if !dirty && len(records) == 0 {
		return nil
	}

	var row orderRow
	var outboxRecords []outbox.Record
	if dirty {
		// Map everything before opening the transaction so a mapping bug fails
		// without touching storage.
		total, err := order.Total()
		if err != nil {
			return err
		}
		row = toRow(order.Snapshot(), total, pending.NextVersion)
		for _, event := range pending.Events {
			rec, err := toOutboxRecord(event)
			if err != nil {
				return err
			}
			outboxRecords = append(outboxRecords, rec)
		}
	}

	err := r.db.InTx(ctx, func(tx *memstore.Tx) error {
		if dirty {
			if err := writeOrder(tx, row, pending.IsNew, pending.ExpectedVersion); err != nil {
				return err
			}
			for _, rec := range outboxRecords {
				if err := outbox.Append(tx, rec); err != nil {
					return err
				}
			}
		}
		for _, rec := range records {
			if err := insertIdempotency(tx, order.TenantID(), rec); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err // pending work is retained for a retry
	}
	if dirty {
		ack()
	}
	return nil
}

// SaveAll writes a batch of orders atomically: every order's state and events
// commit in one transaction, and no order is acknowledged unless all of them
// committed. Calling Save in a loop would acknowledge earlier orders even if a
// later one failed.
func (r *Repository) SaveAll(ctx context.Context, orders []*domain.Order) error {
	type unit struct {
		row     orderRow
		pending aggregate.Pending[domain.Event]
		ack     func()
		events  []outbox.Record
	}
	units := make([]unit, 0, len(orders))
	for _, order := range orders {
		pending, ack, dirty := order.BeginPersist()
		if !dirty {
			continue
		}
		total, err := order.Total()
		if err != nil {
			return err
		}
		u := unit{row: toRow(order.Snapshot(), total, pending.NextVersion), pending: pending, ack: ack}
		for _, event := range pending.Events {
			rec, err := toOutboxRecord(event)
			if err != nil {
				return err
			}
			u.events = append(u.events, rec)
		}
		units = append(units, u)
	}
	if len(units) == 0 {
		return nil
	}

	err := r.db.InTx(ctx, func(tx *memstore.Tx) error {
		for _, u := range units {
			if err := writeOrder(tx, u.row, u.pending.IsNew, u.pending.ExpectedVersion); err != nil {
				return err
			}
			for _, rec := range u.events {
				if err := outbox.Append(tx, rec); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err // no order is acknowledged
	}
	for _, u := range units {
		u.ack()
	}
	return nil
}

// writeOrder inserts a new order or performs a compare-and-set update on the
// stored version, so a stale writer cannot silently overwrite a newer commit.
func writeOrder(tx *memstore.Tx, row orderRow, isNew bool, expectedVersion int64) error {
	key := orderKey(row.TenantID, row.OrderID)
	if isNew {
		if err := tx.Insert(ordersTable, key, row); err != nil {
			if errors.Is(err, memstore.ErrAlreadyExists) {
				return fmt.Errorf("insert order %q: %w", row.OrderID, ports.ErrConcurrentModification)
			}
			return fmt.Errorf("insert order %q: %w", row.OrderID, err)
		}
		return nil
	}

	var current orderRow
	if err := tx.Get(ordersTable, key, &current); err != nil {
		if errors.Is(err, memstore.ErrNotFound) {
			return fmt.Errorf("update order %q: %w", row.OrderID, ports.ErrNotFound)
		}
		return fmt.Errorf("update order %q: %w", row.OrderID, err)
	}
	if current.Version != expectedVersion {
		return fmt.Errorf("update order %q at version %d (stored %d): %w",
			row.OrderID, expectedVersion, current.Version, ports.ErrConcurrentModification)
	}
	return tx.Put(ordersTable, key, row)
}

type idempotencyRow struct {
	Fingerprint string `json:"fingerprint,omitempty"`
	Response    string `json:"response,omitempty"`
}

func insertIdempotency(tx *memstore.Tx, tenantID string, rec ports.IdempotencyRecord) error {
	if rec.Scope == "" || rec.Key == "" {
		return errors.New("insert idempotency record: scope and key are required")
	}
	err := tx.Insert(idempotencyTable, idempotencyKey(tenantID, rec.Scope, rec.Key),
		idempotencyRow{Fingerprint: rec.Fingerprint, Response: rec.Response})
	if errors.Is(err, memstore.ErrAlreadyExists) {
		return fmt.Errorf("insert idempotency record %s: %w", rec.Scope, ports.ErrDuplicate)
	}
	return err
}

// Lookup implements ports.IdempotencyStore.
func (r *Repository) Lookup(ctx context.Context, tenantID, scope, key string) (ports.IdempotencyRecord, bool, error) {
	var row idempotencyRow
	err := r.db.InTx(ctx, func(tx *memstore.Tx) error {
		return tx.Get(idempotencyTable, idempotencyKey(tenantID, scope, key), &row)
	})
	if errors.Is(err, memstore.ErrNotFound) {
		return ports.IdempotencyRecord{}, false, nil
	}
	if err != nil {
		return ports.IdempotencyRecord{}, false, fmt.Errorf("look up idempotency record: %w", err)
	}
	return ports.IdempotencyRecord{Scope: scope, Key: key, Fingerprint: row.Fingerprint, Response: row.Response}, true, nil
}

// GetOrder implements queries.OrderReader.
func (r *Repository) GetOrder(ctx context.Context, tenantID, orderID string) (queries.OrderView, error) {
	var row orderRow
	err := r.db.InTx(ctx, func(tx *memstore.Tx) error {
		return tx.Get(ordersTable, orderKey(tenantID, orderID), &row)
	})
	if errors.Is(err, memstore.ErrNotFound) {
		return queries.OrderView{}, ports.ErrNotFound
	}
	if err != nil {
		return queries.OrderView{}, fmt.Errorf("load order row: %w", err)
	}
	return toView(row), nil
}

// ListOrders implements queries.OrderReader. A database adapter would express
// the same contract as an index-backed query with a keyset predicate; this
// in-memory version scans the tenant's rows.
func (r *Repository) ListOrders(ctx context.Context, tenantID string, filter queries.ListFilter) ([]queries.OrderView, error) {
	var views []queries.OrderView
	err := r.db.InTx(ctx, func(tx *memstore.Tx) error {
		views = views[:0]
		prefix := tenantID + "/"
		for _, key := range tx.Keys(ordersTable) {
			if !strings.HasPrefix(key, prefix) {
				continue
			}
			var row orderRow
			if err := tx.Get(ordersTable, key, &row); err != nil {
				return err
			}
			if filter.Status != "" && row.Status != filter.Status {
				continue
			}
			views = append(views, toView(row))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan order rows: %w", err)
	}

	slices.SortFunc(views, func(a, b queries.OrderView) int {
		if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
			return c // newest first
		}
		return cmp.Compare(a.OrderID, b.OrderID)
	})
	if after := filter.After; after != nil {
		views = slices.DeleteFunc(views, func(v queries.OrderView) bool {
			return !isAfter(v, *after)
		})
	}
	return views[:min(filter.Limit, len(views))], nil
}

// isAfter reports whether v sorts strictly after p in (CreatedAt desc, OrderID
// asc) order — the keyset predicate a database query would use.
func isAfter(v queries.OrderView, p queries.Position) bool {
	if !v.CreatedAt.Equal(p.CreatedAt) {
		return v.CreatedAt.Before(p.CreatedAt)
	}
	return v.OrderID > p.OrderID
}

func toView(row orderRow) queries.OrderView {
	return queries.OrderView{
		OrderID:    row.OrderID,
		CustomerID: row.CustomerID,
		Status:     row.Status,
		TotalMinor: row.TotalMinor,
		Currency:   row.Currency,
		CreatedAt:  row.CreatedAt,
	}
}

func orderKey(tenantID, orderID string) string { return tenantID + "/" + orderID }

func idempotencyKey(tenantID, scope, key string) string {
	return tenantID + "/" + scope + "/" + key
}
