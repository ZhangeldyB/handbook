// Package outbox implements the transactional outbox: repositories append event
// records in the same transaction as aggregate state, and a Relay later
// publishes them at least once.
package outbox

import (
	"errors"
	"fmt"
	"time"

	"example.com/orders/internal/platform/memstore"
)

// Table is the storage table holding outbox records.
const Table = "outbox"

// Status is the delivery state of a record.
type Status string

// Record statuses.
const (
	StatusPending   Status = "PENDING"
	StatusClaimed   Status = "CLAIMED"
	StatusPublished Status = "PUBLISHED"
	StatusDead      Status = "DEAD"
)

// Record is one durable intent to publish an event.
type Record struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	Topic         string    `json:"topic"`
	AggregateID   string    `json:"aggregate_id"`
	Payload       []byte    `json:"payload"`
	OccurredAt    time.Time `json:"occurred_at"`
	Status        Status    `json:"status"`
	Attempts      int       `json:"attempts"`
	NextAttemptAt time.Time `json:"next_attempt_at"`
	ClaimToken    string    `json:"claim_token,omitempty"`
	ClaimedUntil  time.Time `json:"claimed_until,omitzero"`
	LastError     string    `json:"last_error,omitempty"`
	PublishedAt   time.Time `json:"published_at,omitzero"`
}

// Append stores r as a pending record inside tx. It must be called in the same
// transaction that writes the aggregate state the event describes.
func Append(tx *memstore.Tx, r Record) error {
	if r.ID == "" || r.Topic == "" || r.TenantID == "" {
		return errors.New("append outbox record: id, topic, and tenant are required")
	}
	r.Status = StatusPending
	r.NextAttemptAt = r.OccurredAt
	if err := tx.Insert(Table, recordKey(r), r); err != nil {
		return fmt.Errorf("append outbox record %q: %w", r.ID, err)
	}
	return nil
}

// recordKey orders records by occurrence time, then ID, so a key scan returns
// them oldest first.
func recordKey(r Record) string {
	return r.OccurredAt.UTC().Format("20060102T150405.000000000") + "/" + r.ID
}
