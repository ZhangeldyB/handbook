# State Machine Template

One table is the single source of truth for legal transitions. Working version: [status.go](../examples/orders/internal/orders/domain/status.go).

## Status Type and Table

```go
// Status values are the stored and published representation; never rename them.
type Status string

const (
	StatusPending   Status = "PENDING"
	StatusActive    Status = "ACTIVE"
	StatusCompleted Status = "COMPLETED"
	StatusCancelled Status = "CANCELLED"
)

var allowedTransitions = map[Status][]Status{
	StatusPending: {StatusActive, StatusCancelled},
	StatusActive:  {StatusCompleted, StatusCancelled},
}

func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusActive, StatusCompleted, StatusCancelled:
		return true
	default:
		return false
	}
}

func ParseStatus(value string) (Status, error) {
	s := Status(value)
	if !s.Valid() {
		return "", fmt.Errorf("parse status %q: %w", value, ErrInvalid<Entity>)
	}
	return s, nil
}

func validateTransition(from, to Status) error {
	if slices.Contains(allowedTransitions[from], to) {
		return nil
	}
	return fmt.Errorf("%s -> %s: %w", from, to, ErrInvalidTransition)
}
```

Terminal states simply have no entry. Same-state behavior is decided per operation in the aggregate method (idempotent no-op or error), not globally in the table.

## Guard Methods

Expose a guard only when a caller genuinely needs a pre-check (UI hints, batch filtering):

```go
func (e *<Entity>) CanActivate() bool { return validateTransition(e.status, StatusActive) == nil }
```

The command method still enforces the transition for every caller.

## Several State Machines

An aggregate with two lifecycles (for example `status` and `reviewStatus`) keeps one table per lifecycle; a method that moves both validates both before mutating either.

## Tests

```go
func TestTransitions(t *testing.T) {
	tests := []struct {
		from, to Status
		allowed  bool
	}{
		{StatusPending, StatusActive, true},
		{StatusActive, StatusPending, false},
		{StatusCompleted, StatusActive, false},
		{StatusCancelled, StatusActive, false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s->%s", tt.from, tt.to), func(t *testing.T) {
			err := validateTransition(tt.from, tt.to)
			if (err == nil) != tt.allowed {
				t.Fatalf("validateTransition() error = %v, allowed %v", err, tt.allowed)
			}
		})
	}
}
```

Also test every public method's behavior for each starting state, including same-state calls.

## When to Use a Library

For workflow coordinators with many states, guards, and substates, a state-machine library may help — keep it in the application layer, not inside aggregates, and treat its transition table as immutable after construction.
