# Domain Aggregate Template

Use only for a context whose behavior and consistency rules justify an aggregate. Working version: [order.go](../examples/orders/internal/orders/domain/order.go).

## Files

```text
internal/<context>/domain/
├── <entity>.go      # state, constructors, snapshot, getters, behavior
├── status.go        # Status type, Parse, transition table
├── events.go        # Event interface and event structs
├── errors.go        # stable domain errors
└── <value>.go       # value objects with substance (money.go, ...)
```

## Aggregate

```go
package domain

import (
	"fmt"
	"slices"
	"time"

	"<module>/internal/platform/aggregate"
)

type <Entity> struct {
	tenantID  string
	id        string
	status    Status
	createdAt time.Time
	// ...every business field, unexported

	tracker aggregate.Tracker[Event]
}

type New<Entity>Params struct {
	TenantID string
	ID       string
	Now      time.Time
	// ...creation inputs
}

func New<Entity>(p New<Entity>Params) (*<Entity>, error) {
	if p.TenantID == "" || p.ID == "" || p.Now.IsZero() {
		return nil, fmt.Errorf("new <entity>: tenant, id, and time are required: %w", ErrInvalid<Entity>)
	}
	e := &<Entity>{
		tenantID:  p.TenantID,
		id:        p.ID,
		status:    StatusPending,
		createdAt: p.Now,
		tracker:   aggregate.NewForCreate[Event](),
	}
	// Record a creation event here if consumers need one, so no caller can forget it.
	return e, nil
}

// Snapshot is the complete persisted state. Adding a field here without mapping
// it fails the repository round-trip test.
type Snapshot struct {
	TenantID  string
	ID        string
	Status    Status
	CreatedAt time.Time
	Version   int64
}

func Reconstitute(s Snapshot) (*<Entity>, error) {
	if s.TenantID == "" || s.ID == "" || !s.Status.Valid() || s.Version <= 0 {
		return nil, fmt.Errorf("reconstitute <entity> %q: %w", s.ID, ErrInvalid<Entity>)
	}
	return &<Entity>{
		tenantID:  s.TenantID,
		id:        s.ID,
		status:    s.Status,
		createdAt: s.CreatedAt,
		tracker:   aggregate.NewForReconstitute[Event](s.Version),
	}, nil
}

func (e *<Entity>) Snapshot() Snapshot {
	return Snapshot{TenantID: e.tenantID, ID: e.id, Status: e.status, CreatedAt: e.createdAt, Version: e.tracker.Version()}
}
```

## Behavior

```go
// Activate moves a pending <entity> to active. A rejected call changes nothing.
func (e *<Entity>) Activate(eventID string, now time.Time) error {
	if e.status == StatusActive {
		return nil // same-state behavior decided by this operation's contract
	}
	if err := validateTransition(e.status, StatusActive); err != nil {
		return fmt.Errorf("activate <entity> %q: %w", e.id, err)
	}
	// ...validate everything else before the first assignment

	e.status = StatusActive
	e.tracker.Record(<Entity>Activated{EventID: eventID, TenantID: e.tenantID, <Entity>ID: e.id, OccurredAt: now})
	return nil
}
```

## Getters and Persistence Surface

```go
func (e *<Entity>) ID() string       { return e.id }
func (e *<Entity>) TenantID() string { return e.tenantID }
func (e *<Entity>) Status() Status   { return e.status }
func (e *<Entity>) Items() []Item    { return slices.Clone(e.items) } // never return internal slices

func (e *<Entity>) BeginPersist() (aggregate.Pending[Event], func(), bool) { return e.tracker.Begin() }
func (e *<Entity>) IsDirty() bool                                         { return e.tracker.IsDirty() }
func (e *<Entity>) PendingEvents() int                                    { return e.tracker.EventCount() }
```

## Events and Errors

```go
type Event interface{ is<Entity>Event() }

type <Entity>Activated struct {
	EventID    string
	TenantID   string
	<Entity>ID string
	OccurredAt time.Time
}

func (<Entity>Activated) is<Entity>Event() {}

var (
	ErrInvalidTransition = errors.New("invalid <entity> status transition")
	ErrInvalid<Entity>   = errors.New("invalid <entity>")
)
```

## Checklist

- [ ] Owns a real invariant; fields private; collections copied.
- [ ] No `context.Context`, I/O, logging, or infrastructure imports.
- [ ] `NewForCreate` in the factory, `NewForReconstitute(version)` in `Reconstitute`.
- [ ] Each method validates, then mutates, then records.
- [ ] `Reconstitute` maps every field and rejects impossible state.
- [ ] Tests: transitions, unchanged state on rejection, events, round trip.
