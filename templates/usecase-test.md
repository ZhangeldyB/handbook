# Usecase Test Template

Working version: [usecases_test.go](../examples/orders/internal/orders/usecases/usecases_test.go).

```go
package usecases

var now = time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return now }

type sequentialIDs struct{ n int }

func (g *sequentialIDs) NewID() string { g.n++; return fmt.Sprintf("id-%d", g.n) }

// fakeStore implements the repository and idempotency ports all-or-nothing,
// like the real adapter.
type fakeStore struct {
	items   map[string]*domain.<Entity>
	records map[string]ports.IdempotencyRecord
	saveErr error
	saves   int
}

func (s *fakeStore) Save(_ context.Context, e *domain.<Entity>, records ...ports.IdempotencyRecord) error {
	if s.saveErr != nil {
		return s.saveErr // nothing acknowledged: pending work stays on the aggregate
	}
	for _, rec := range records {
		if _, exists := s.records[e.TenantID()+"/"+rec.Scope+"/"+rec.Key]; exists {
			return ports.ErrDuplicate
		}
	}
	for _, rec := range records {
		s.records[e.TenantID()+"/"+rec.Scope+"/"+rec.Key] = rec
	}
	if _, ack, ok := e.BeginPersist(); ok {
		ack()
	}
	s.items[e.TenantID()+"/"+e.ID()] = e
	s.saves++
	return nil
}

func TestNew<Operation>RejectsMissingDependencies(t *testing.T) {
	var ids *sequentialIDs // typed nil must be rejected too
	if _, err := New<Operation>(<Operation>Deps{<Entities>: newFakeStore(), Clock: fixedClock{}, IDs: ids}); err == nil {
		t.Fatal("New<Operation>() error = nil, want missing dependency error")
	}
}

func Test<Operation>SaveFailureKeepsPendingEvent(t *testing.T) {
	store := newFakeStore()
	store.items["tenant-1/id-1"] = pending<Entity>(t)
	errUnavailable := errors.New("unavailable")
	store.saveErr = errUnavailable
	cmd, _ := New<Operation>(<Operation>Deps{<Entities>: store, Clock: fixedClock{}, IDs: &sequentialIDs{}})

	err := cmd.Execute(t.Context(), <Operation>Request{TenantID: "tenant-1", <Entity>ID: "id-1"})

	if !errors.Is(err, errUnavailable) {
		t.Fatalf("Execute() error = %v, want %v", err, errUnavailable)
	}
	if store.items["tenant-1/id-1"].PendingEvents() != 1 {
		t.Fatal("failed save lost the pending event")
	}
}
```

Also cover: success; domain rejection; `ports.ErrNotFound`; idempotent replay, key reuse, and race-loser recovery (for creates); duplicate delivery (for consumers); cancellation.
