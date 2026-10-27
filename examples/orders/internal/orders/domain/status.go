package domain

import (
	"fmt"
	"slices"
)

// Status is the order lifecycle state. The string values are the stored and
// published representation and must never be renamed.
type Status string

// Order statuses. The zero value is invalid.
const (
	StatusDraft     Status = "DRAFT"
	StatusPlaced    Status = "PLACED"
	StatusPaid      Status = "PAID"
	StatusCancelled Status = "CANCELLED"
)

// allowedTransitions is the single source of truth for legal status changes.
var allowedTransitions = map[Status][]Status{
	StatusDraft:  {StatusPlaced, StatusCancelled},
	StatusPlaced: {StatusPaid, StatusCancelled},
}

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusDraft, StatusPlaced, StatusPaid, StatusCancelled:
		return true
	default:
		return false
	}
}

// ParseStatus converts a stored value, rejecting unknown values instead of
// guessing.
func ParseStatus(value string) (Status, error) {
	s := Status(value)
	if !s.Valid() {
		return "", fmt.Errorf("parse status %q: %w", value, ErrInvalidOrder)
	}
	return s, nil
}

func validateTransition(from, to Status) error {
	if slices.Contains(allowedTransitions[from], to) {
		return nil
	}
	return fmt.Errorf("%s -> %s: %w", from, to, ErrInvalidTransition)
}
