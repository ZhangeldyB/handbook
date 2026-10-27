// Package aggregate tracks the pending persistence work of one aggregate
// instance and acknowledges that work only after it has been committed.
//
// An aggregate keeps a Tracker in an unexported field, calls Changed or Record
// from its own domain methods, and exposes Begin/Persist so a repository can
// write state and events in one transaction. A failed commit leaves every
// captured change and event pending, so the same in-memory aggregate can be
// retried without losing outbox events.
//
// The zero Tracker is not usable: construct it with NewForCreate or
// NewForReconstitute. A Tracker is not safe for concurrent use; an aggregate is
// owned by one goroutine at a time.
package aggregate

import "slices"

// Tracker records whether an aggregate is new, whether it has unpersisted state
// changes, and which events are waiting to be written to the outbox.
type Tracker[E any] struct {
	initialized bool
	isNew       bool
	version     int64
	changes     uint64
	events      []E
}

// Pending is a snapshot of the work captured by Begin. Repositories treat it as
// read-only.
type Pending[E any] struct {
	// IsNew reports whether the aggregate has never been committed, which
	// selects an insert instead of a version-checked update.
	IsNew bool
	// ExpectedVersion is the committed version the aggregate was loaded at.
	// It is zero when IsNew is true.
	ExpectedVersion int64
	// NextVersion is the version to store when the commit succeeds.
	NextVersion int64
	// Events are the captured events in the order they were recorded.
	Events []E
}

// NewForCreate returns a Tracker for an aggregate that has not been persisted.
// It starts dirty so that the first save inserts the aggregate.
func NewForCreate[E any]() Tracker[E] {
	return Tracker[E]{initialized: true, isNew: true, changes: 1}
}

// NewForReconstitute returns a clean Tracker for an aggregate loaded from
// storage at the given committed version.
func NewForReconstitute[E any](version int64) Tracker[E] {
	return Tracker[E]{initialized: true, version: version}
}

// Changed marks aggregate state as modified without recording an event.
func (t *Tracker[E]) Changed() {
	t.mustBeInitialized()
	t.changes++
}

// Record appends an event and marks aggregate state as modified.
func (t *Tracker[E]) Record(event E) {
	t.mustBeInitialized()
	t.events = append(t.events, event)
	t.changes++
}

// IsNew reports whether the aggregate has never been committed.
func (t *Tracker[E]) IsNew() bool { return t.isNew }

// IsDirty reports whether there is any unacknowledged change or event.
func (t *Tracker[E]) IsDirty() bool { return t.changes > 0 }

// Version returns the last committed version known to this instance.
func (t *Tracker[E]) Version() int64 { return t.version }

// EventCount returns the number of events waiting to be persisted.
func (t *Tracker[E]) EventCount() int { return len(t.events) }

// Begin captures the pending work. ok is false when there is nothing to
// persist. Call ack only after the transaction that wrote the captured work has
// committed; ack acknowledges exactly the captured work, so changes recorded
// after Begin stay pending. Calling ack more than once, or after a newer
// commit was acknowledged, has no effect.
func (t *Tracker[E]) Begin() (pending Pending[E], ack func(), ok bool) {
	t.mustBeInitialized()
	if t.changes == 0 {
		return Pending[E]{}, func() {}, false
	}

	captured := t.changes
	n := len(t.events)
	pending = Pending[E]{
		IsNew:           t.isNew,
		ExpectedVersion: t.version,
		NextVersion:     t.version + 1,
		Events:          slices.Clone(t.events[:n:n]),
	}

	ack = func() {
		if t.isNew != pending.IsNew || t.version != pending.ExpectedVersion {
			return // stale or repeated acknowledgement
		}
		t.isNew = false
		t.version = pending.NextVersion
		t.changes -= captured
		t.events = slices.Clone(t.events[n:])
	}
	return pending, ack, true
}

// Persist runs apply with the captured work and acknowledges it only when apply
// returns nil. It returns nil without calling apply when nothing is pending.
func (t *Tracker[E]) Persist(apply func(Pending[E]) error) error {
	pending, ack, ok := t.Begin()
	if !ok {
		return nil
	}
	if err := apply(pending); err != nil {
		return err
	}
	ack()
	return nil
}

func (t *Tracker[E]) mustBeInitialized() {
	if !t.initialized {
		panic("aggregate: Tracker used before NewForCreate or NewForReconstitute")
	}
}
