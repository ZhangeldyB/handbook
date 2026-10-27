// Package domain contrasts domain-layer mistakes with their fixes. Functions
// ending in Bad are intentionally wrong; never copy them.
package domain

import (
	"context"
	"errors"
	"slices"
	"time"
)

var ErrInvalidTransition = errors.New("invalid transition")

type Order struct {
	status string
	lines  []string
}

// ActivateBad accepts a context and reads the wall clock, so the domain depends
// on request lifetime and cannot be tested deterministically.
func (o *Order) ActivateBad(ctx context.Context) (time.Time, error) {
	if err := ctx.Err(); err != nil {
		return time.Time{}, err
	}
	o.status = "ACTIVE"
	return time.Now(), nil
}

// ActivateGood is pure: the application passes the decision time in.
func (o *Order) ActivateGood(now time.Time) (time.Time, error) {
	if o.status != "PENDING" {
		return time.Time{}, ErrInvalidTransition
	}
	o.status = "ACTIVE"
	return now, nil
}

// LinesBad leaks the internal slice; callers can mutate the aggregate without
// going through a method that enforces its invariants.
func (o *Order) LinesBad() []string { return o.lines }

// LinesGood returns a copy.
func (o *Order) LinesGood() []string { return slices.Clone(o.lines) }

// CancelBad mutates before validating, so a rejected call leaves the order
// half-changed.
func (o *Order) CancelBad(reason string) error {
	o.status = "CANCELLED"
	if reason == "" {
		return errors.New("reason required")
	}
	return nil
}

// CancelGood validates everything first; on error the state is unchanged.
func (o *Order) CancelGood(reason string) error {
	if reason == "" {
		return errors.New("reason required")
	}
	if o.status == "PAID" {
		return ErrInvalidTransition
	}
	o.status = "CANCELLED"
	return nil
}
