package usecases

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"example.com/orders/internal/orders/domain"
	"example.com/orders/internal/orders/ports"
)

var now = time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return now }

type sequentialIDs struct{ n int }

func (g *sequentialIDs) NewID() string { g.n++; return fmt.Sprintf("id-%d", g.n) }

// fakeStore is a small in-memory implementation of the repository and
// idempotency ports. It commits all-or-nothing like the real adapter.
type fakeStore struct {
	orders  map[string]*domain.Order
	records map[string]ports.IdempotencyRecord
	saveErr error
	saves   int
}

func newFakeStore() *fakeStore {
	return &fakeStore{orders: map[string]*domain.Order{}, records: map[string]ports.IdempotencyRecord{}}
}

func (s *fakeStore) Get(_ context.Context, tenantID, orderID string) (*domain.Order, error) {
	order, ok := s.orders[tenantID+"/"+orderID]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return order, nil
}

func (s *fakeStore) Save(_ context.Context, order *domain.Order, records ...ports.IdempotencyRecord) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	for _, rec := range records {
		if _, exists := s.records[order.TenantID()+"/"+rec.Scope+"/"+rec.Key]; exists {
			return ports.ErrDuplicate
		}
	}
	for _, rec := range records {
		s.records[order.TenantID()+"/"+rec.Scope+"/"+rec.Key] = rec
	}
	if _, ack, ok := order.BeginPersist(); ok {
		ack()
	}
	s.orders[order.TenantID()+"/"+order.ID()] = order
	s.saves++
	return nil
}

func (s *fakeStore) Lookup(_ context.Context, tenantID, scope, key string) (ports.IdempotencyRecord, bool, error) {
	rec, ok := s.records[tenantID+"/"+scope+"/"+key]
	return rec, ok, nil
}

func newCreateDraft(t *testing.T, store *fakeStore) *CreateDraft {
	t.Helper()
	cmd, err := NewCreateDraft(CreateDraftDeps{Orders: store, Idempotency: store, Clock: fixedClock{}, IDs: &sequentialIDs{}})
	if err != nil {
		t.Fatalf("NewCreateDraft() error = %v", err)
	}
	return cmd
}

func draftRequest(key string) CreateDraftRequest {
	return CreateDraftRequest{
		TenantID: "tenant-1", CustomerID: "customer-1", Currency: "EUR",
		Lines:          []LineInput{{SKU: "sku-1", Quantity: 2, UnitPrice: 500}},
		IdempotencyKey: key,
	}
}

func TestConstructorsRejectMissingDependencies(t *testing.T) {
	store := newFakeStore()
	tests := []struct {
		name string
		new  func() error
	}{
		{name: "create draft without clock", new: func() error {
			_, err := NewCreateDraft(CreateDraftDeps{Orders: store, Idempotency: store, IDs: &sequentialIDs{}})
			return err
		}},
		{name: "place without orders", new: func() error {
			_, err := NewPlace(PlaceDeps{Clock: fixedClock{}, IDs: &sequentialIDs{}})
			return err
		}},
		{name: "mark paid with typed nil IDs", new: func() error {
			var ids *sequentialIDs
			_, err := NewMarkPaid(MarkPaidDeps{Orders: store, Clock: fixedClock{}, IDs: ids})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.new(); err == nil {
				t.Fatal("constructor error = nil, want missing dependency error")
			}
		})
	}
}

func TestCreateDraftReplayReturnsOriginalResult(t *testing.T) {
	store := newFakeStore()
	cmd := newCreateDraft(t, store)

	first, err := cmd.Execute(t.Context(), draftRequest("key-1"))
	if err != nil {
		t.Fatalf("first Execute() error = %v", err)
	}
	second, err := cmd.Execute(t.Context(), draftRequest("key-1"))
	if err != nil {
		t.Fatalf("replayed Execute() error = %v", err)
	}

	if second.OrderID != first.OrderID {
		t.Fatalf("replay returned %q, want original %q", second.OrderID, first.OrderID)
	}
	if store.saves != 1 {
		t.Fatalf("saves = %d, want 1", store.saves)
	}
}

func TestCreateDraftRejectsKeyReuseWithDifferentPayload(t *testing.T) {
	store := newFakeStore()
	cmd := newCreateDraft(t, store)
	if _, err := cmd.Execute(t.Context(), draftRequest("key-1")); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	changed := draftRequest("key-1")
	changed.Lines[0].Quantity = 3
	_, err := cmd.Execute(t.Context(), changed)

	if !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Fatalf("Execute() error = %v, want %v", err, ErrIdempotencyKeyReused)
	}
}

func TestCreateDraftRaceLoserReturnsWinnerResult(t *testing.T) {
	store := newFakeStore()
	cmd := newCreateDraft(t, store)
	// The winner committed after the loser's initial lookup missed.
	winner := ports.IdempotencyRecord{Scope: scopeCreateDraft, Key: "key-1", Fingerprint: fingerprintOf(draftRequest("key-1")), Response: "winner-order"}
	racing := &racingStore{fakeStore: store, commitBeforeSave: winner}
	cmd.orders, cmd.idempotency = racing, racing

	got, err := cmd.Execute(t.Context(), draftRequest("key-1"))

	if err != nil || got.OrderID != "winner-order" {
		t.Fatalf("Execute() = %+v, %v, want winner-order, nil", got, err)
	}
}

// racingStore commits a competing record just before the first Save.
type racingStore struct {
	*fakeStore
	commitBeforeSave ports.IdempotencyRecord
	raced            bool
}

func (s *racingStore) Save(ctx context.Context, order *domain.Order, records ...ports.IdempotencyRecord) error {
	if !s.raced {
		s.raced = true
		s.records[order.TenantID()+"/"+s.commitBeforeSave.Scope+"/"+s.commitBeforeSave.Key] = s.commitBeforeSave
	}
	return s.fakeStore.Save(ctx, order, records...)
}

func TestPlaceReturnsSaveFailureAndKeepsPendingEvent(t *testing.T) {
	store := newFakeStore()
	order, _ := domain.NewOrder(domain.NewOrderParams{TenantID: "tenant-1", OrderID: "order-1", CustomerID: "c", Currency: "EUR", Now: now})
	price, _ := domain.NewMoney(100, "EUR")
	line, _ := domain.NewLine("sku", 1, price)
	_ = order.AddLine(line)
	store.orders["tenant-1/order-1"] = order
	errUnavailable := errors.New("unavailable")
	store.saveErr = errUnavailable
	cmd, _ := NewPlace(PlaceDeps{Orders: store, Clock: fixedClock{}, IDs: &sequentialIDs{}})

	err := cmd.Execute(t.Context(), PlaceRequest{TenantID: "tenant-1", OrderID: "order-1"})

	if !errors.Is(err, errUnavailable) {
		t.Fatalf("Execute() error = %v, want %v", err, errUnavailable)
	}
	if order.PendingEvents() != 1 {
		t.Fatalf("pending events = %d, want 1 retained for retry", order.PendingEvents())
	}
}

func TestPlaceUnknownOrderReturnsNotFound(t *testing.T) {
	cmd, _ := NewPlace(PlaceDeps{Orders: newFakeStore(), Clock: fixedClock{}, IDs: &sequentialIDs{}})

	err := cmd.Execute(t.Context(), PlaceRequest{TenantID: "tenant-1", OrderID: "missing"})

	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Execute() error = %v, want %v", err, ports.ErrNotFound)
	}
}

func TestMarkPaidDuplicateDeliveryIsANoOp(t *testing.T) {
	store := newFakeStore()
	order, _ := domain.Reconstitute(domain.Snapshot{
		TenantID: "tenant-1", OrderID: "order-1", CustomerID: "c", Currency: "EUR",
		Status: domain.StatusPlaced, CreatedAt: now, PlacedAt: now, Version: 2,
	})
	store.orders["tenant-1/order-1"] = order
	cmd, _ := NewMarkPaid(MarkPaidDeps{Orders: store, Clock: fixedClock{}, IDs: &sequentialIDs{}})
	req := MarkPaidRequest{TenantID: "tenant-1", OrderID: "order-1", PaymentID: "pay-1", EventID: "event-1"}

	for range 2 {
		if err := cmd.Execute(t.Context(), req); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	}

	if store.saves != 1 {
		t.Fatalf("saves = %d, want 1 for a redelivered event", store.saves)
	}
}
