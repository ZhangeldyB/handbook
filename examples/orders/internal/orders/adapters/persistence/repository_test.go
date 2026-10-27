package persistence

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"example.com/orders/internal/orders/domain"
	"example.com/orders/internal/orders/ports"
	"example.com/orders/internal/orders/queries"
	"example.com/orders/internal/platform/memstore"
	"example.com/orders/internal/platform/outbox"
)

var now = time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)

func newRepo(t *testing.T) (*Repository, *memstore.DB) {
	t.Helper()
	db := memstore.New()
	repo, err := New(db)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return repo, db
}

func newDraft(t *testing.T, tenantID, orderID string) *domain.Order {
	t.Helper()
	order, err := domain.NewOrder(domain.NewOrderParams{TenantID: tenantID, OrderID: orderID, CustomerID: "customer-1", Currency: "EUR", Now: now})
	if err != nil {
		t.Fatalf("NewOrder() error = %v", err)
	}
	price, _ := domain.NewMoney(1000, "EUR")
	line, _ := domain.NewLine("sku-1", 3, price)
	if err := order.AddLine(line); err != nil {
		t.Fatalf("AddLine() error = %v", err)
	}
	return order
}

func outboxCount(t *testing.T, db *memstore.DB) int {
	t.Helper()
	n := 0
	_ = db.InTx(t.Context(), func(tx *memstore.Tx) error {
		n = len(tx.Keys(outbox.Table))
		return nil
	})
	return n
}

// TestRowMappingIsComplete fails when a Snapshot field is added without being
// mapped: the fixture must set every field, and the round trip must keep it.
func TestRowMappingIsComplete(t *testing.T) {
	price, _ := domain.NewMoney(250, "EUR")
	line, _ := domain.NewLine("sku-9", 4, price)
	full := domain.Snapshot{
		TenantID: "tenant-1", OrderID: "order-1", CustomerID: "customer-1", Currency: "EUR",
		Status: domain.StatusCancelled, Lines: []domain.Line{line}, CreatedAt: now, PlacedAt: now.Add(time.Minute),
		PaymentID: "pay-1", CancelReason: "duplicate", Version: 7,
	}
	v := reflect.ValueOf(full)
	for i := range v.NumField() {
		if v.Field(i).IsZero() {
			t.Fatalf("fixture leaves Snapshot.%s zero; set it so the round trip covers it", v.Type().Field(i).Name)
		}
	}

	got, err := toSnapshot(toRow(full, domain.Zero("EUR"), full.Version))
	if err != nil {
		t.Fatalf("toSnapshot() error = %v", err)
	}
	if !reflect.DeepEqual(got, full) {
		t.Fatalf("round trip = %+v, want %+v", got, full)
	}
}

func TestSaveWritesStateAndOutboxTogether(t *testing.T) {
	repo, db := newRepo(t)
	order := newDraft(t, "tenant-1", "order-1")
	if err := order.Place("event-1", now); err != nil {
		t.Fatalf("Place() error = %v", err)
	}

	if err := repo.Save(t.Context(), order); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := repo.Get(t.Context(), "tenant-1", "order-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loaded.Status() != domain.StatusPlaced {
		t.Errorf("Status() = %s, want %s", loaded.Status(), domain.StatusPlaced)
	}
	if got := outboxCount(t, db); got != 1 {
		t.Errorf("outbox records = %d, want 1", got)
	}
	if order.IsDirty() {
		t.Error("successful save did not acknowledge pending work")
	}
}

func TestFailedCommitRetainsPendingWorkAndRetryWritesOnce(t *testing.T) {
	repo, db := newRepo(t)
	order := newDraft(t, "tenant-1", "order-1")
	_ = order.Place("event-1", now)
	errUnavailable := errors.New("unavailable")
	db.FailNextCommit(errUnavailable)

	if err := repo.Save(t.Context(), order); !errors.Is(err, errUnavailable) {
		t.Fatalf("Save() error = %v, want %v", err, errUnavailable)
	}
	if !order.IsDirty() || order.PendingEvents() != 1 || outboxCount(t, db) != 0 {
		t.Fatalf("after failure: dirty=%v events=%d outbox=%d, want retained work and empty outbox",
			order.IsDirty(), order.PendingEvents(), outboxCount(t, db))
	}

	if err := repo.Save(t.Context(), order); err != nil {
		t.Fatalf("retry Save() error = %v", err)
	}
	if err := repo.Save(t.Context(), order); err != nil {
		t.Fatalf("no-op Save() error = %v", err)
	}
	if got := outboxCount(t, db); got != 1 {
		t.Fatalf("outbox records = %d, want exactly 1", got)
	}
}

func TestStaleWriterGetsConcurrentModification(t *testing.T) {
	repo, _ := newRepo(t)
	if err := repo.Save(t.Context(), newDraft(t, "tenant-1", "order-1")); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	first, _ := repo.Get(t.Context(), "tenant-1", "order-1")
	second, _ := repo.Get(t.Context(), "tenant-1", "order-1")

	_ = first.Place("event-1", now)
	if err := repo.Save(t.Context(), first); err != nil {
		t.Fatalf("first Save() error = %v", err)
	}
	_ = second.Cancel("event-2", "changed mind", now)
	err := repo.Save(t.Context(), second)

	if !errors.Is(err, ports.ErrConcurrentModification) {
		t.Fatalf("second Save() error = %v, want %v", err, ports.ErrConcurrentModification)
	}
}

func TestDuplicateIdempotencyRecordRollsBackWholeSave(t *testing.T) {
	repo, _ := newRepo(t)
	record := ports.IdempotencyRecord{Scope: "CreateDraft", Key: "key-1", Response: "order-1"}
	if err := repo.Save(t.Context(), newDraft(t, "tenant-1", "order-1"), record); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	err := repo.Save(t.Context(), newDraft(t, "tenant-1", "order-2"), record)

	if !errors.Is(err, ports.ErrDuplicate) {
		t.Fatalf("Save() error = %v, want %v", err, ports.ErrDuplicate)
	}
	if _, err := repo.Get(t.Context(), "tenant-1", "order-2"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get(order-2) error = %v, want rolled back (%v)", err, ports.ErrNotFound)
	}
}

func TestSaveAllAcknowledgesNothingWhenTheBatchFails(t *testing.T) {
	repo, _ := newRepo(t)
	existing := newDraft(t, "tenant-1", "order-2")
	if err := repo.Save(t.Context(), existing); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	fresh := newDraft(t, "tenant-1", "order-1")
	_ = fresh.Place("event-1", now)
	duplicate := newDraft(t, "tenant-1", "order-2") // new aggregate whose ID already exists

	err := repo.SaveAll(t.Context(), []*domain.Order{fresh, duplicate})

	if !errors.Is(err, ports.ErrConcurrentModification) {
		t.Fatalf("SaveAll() error = %v, want %v", err, ports.ErrConcurrentModification)
	}
	if !fresh.IsDirty() || fresh.PendingEvents() != 1 {
		t.Fatal("SaveAll acknowledged an order although the batch rolled back")
	}
	if _, err := repo.Get(t.Context(), "tenant-1", "order-1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get(order-1) error = %v, want rolled back (%v)", err, ports.ErrNotFound)
	}
}

func TestSaveAllCommitsEveryOrder(t *testing.T) {
	repo, db := newRepo(t)
	first, second := newDraft(t, "tenant-1", "order-1"), newDraft(t, "tenant-1", "order-2")
	_ = first.Place("event-1", now)
	_ = second.Place("event-2", now)

	if err := repo.SaveAll(t.Context(), []*domain.Order{first, second}); err != nil {
		t.Fatalf("SaveAll() error = %v", err)
	}
	if first.IsDirty() || second.IsDirty() || outboxCount(t, db) != 2 {
		t.Fatalf("dirty=%v,%v outbox=%d, want both acknowledged and 2 records", first.IsDirty(), second.IsDirty(), outboxCount(t, db))
	}
}

func TestTenantIsolation(t *testing.T) {
	repo, _ := newRepo(t)
	if err := repo.Save(t.Context(), newDraft(t, "tenant-1", "order-1")); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if _, err := repo.Get(t.Context(), "tenant-2", "order-1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get() from another tenant error = %v, want %v", err, ports.ErrNotFound)
	}
	views, err := repo.ListOrders(t.Context(), "tenant-2", queries.ListFilter{Limit: 10})
	if err != nil || len(views) != 0 {
		t.Fatalf("ListOrders() = %v, %v, want no rows for another tenant", views, err)
	}
}
