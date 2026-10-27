// Package persistence contrasts persistence and side-effect mistakes with their
// fixes. Functions ending in Bad are intentionally wrong; never copy them.
package persistence

import (
	"context"
	"errors"
)

type Event struct{ ID string }

type Aggregate struct{ events []Event }

// DrainEventsBad clears pending events before the commit is known to succeed.
func (a *Aggregate) DrainEventsBad() []Event {
	events := a.events
	a.events = nil
	return events
}

type Tx interface {
	WriteState(a *Aggregate) error
	AppendOutbox(events []Event) error
}

type DB interface {
	InTx(ctx context.Context, fn func(tx Tx) error) error
}

// SaveBad drains first: if the transaction fails, the events are gone and a
// retry commits state without them.
func SaveBad(ctx context.Context, db DB, a *Aggregate) error {
	events := a.DrainEventsBad()
	return db.InTx(ctx, func(tx Tx) error {
		if err := tx.WriteState(a); err != nil {
			return err
		}
		return tx.AppendOutbox(events)
	})
}

// SaveGood snapshots pending events and acknowledges them only after commit.
func SaveGood(ctx context.Context, db DB, a *Aggregate) error {
	captured := len(a.events)
	snapshot := append([]Event(nil), a.events...)
	err := db.InTx(ctx, func(tx Tx) error {
		if err := tx.WriteState(a); err != nil {
			return err
		}
		return tx.AppendOutbox(snapshot)
	})
	if err != nil {
		return err // events stay pending for a retry
	}
	a.events = a.events[captured:]
	return nil
}

type PaymentProvider interface {
	Charge(ctx context.Context, idempotencyKey string, amountMinor int64) error
}

// ChargeInsideTxBad calls the provider inside a transaction callback. The
// database may retry the callback (charging twice), and a rollback after a
// successful charge leaves money taken with no local record.
func ChargeInsideTxBad(ctx context.Context, db DB, provider PaymentProvider, a *Aggregate) error {
	return db.InTx(ctx, func(tx Tx) error {
		if err := provider.Charge(ctx, "", 1000); err != nil {
			return err
		}
		return tx.WriteState(a)
	})
}

// ChargeIntentGood commits state plus a durable intent; a separate worker
// performs the charge with a stable idempotency key and reconciles unknown
// outcomes.
func ChargeIntentGood(ctx context.Context, db DB, a *Aggregate, intent Event) error {
	return db.InTx(ctx, func(tx Tx) error {
		if err := tx.WriteState(a); err != nil {
			return err
		}
		return tx.AppendOutbox([]Event{intent})
	})
}

// ErrNotFound is the port-level sentinel callers match.
var ErrNotFound = errors.New("not found")

type rowStore interface {
	Load(ctx context.Context, key string) ([]byte, error)
}

var errStoreNoRows = errors.New("store: no rows")

// LoadBad leaks the storage library's error, so every caller must import and
// match an infrastructure type.
func LoadBad(ctx context.Context, store rowStore, key string) ([]byte, error) {
	return store.Load(ctx, key)
}

// LoadGood translates the expected condition at the adapter boundary.
func LoadGood(ctx context.Context, store rowStore, key string) ([]byte, error) {
	row, err := store.Load(ctx, key)
	if errors.Is(err, errStoreNoRows) {
		return nil, ErrNotFound
	}
	return row, err
}
