package outbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"example.com/orders/internal/platform/memstore"
	"example.com/orders/internal/platform/messaging"
)

var t0 = time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time          { return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

type recordingPublisher struct {
	errs      []error // returned in order; nil once exhausted
	published []messaging.Message
}

func (p *recordingPublisher) Publish(_ context.Context, msg messaging.Message) error {
	if len(p.errs) > 0 {
		err := p.errs[0]
		p.errs = p.errs[1:]
		if err != nil {
			return err
		}
	}
	p.published = append(p.published, msg)
	return nil
}

func testConfig() Config {
	return Config{
		BatchSize:      10,
		PublishTimeout: time.Second,
		Lease:          5 * time.Second,
		MaxAttempts:    3,
		BaseBackoff:    time.Second,
		MaxBackoff:     time.Minute,
		PollInterval:   time.Second,
	}
}

func newTestRelay(t *testing.T, db *memstore.DB, pub messaging.Publisher, clock *fakeClock) *Relay {
	t.Helper()
	n := 0
	relay, err := NewRelay(RelayDeps{
		DB:        db,
		Publisher: pub,
		Logger:    slog.New(slog.DiscardHandler),
		Now:       clock.Now,
		NewToken:  func() string { n++; return fmt.Sprintf("token-%d", n) },
	}, testConfig())
	if err != nil {
		t.Fatalf("NewRelay() error = %v", err)
	}
	relay.jitter = func(d time.Duration) time.Duration { return d }
	return relay
}

func appendRecord(t *testing.T, db *memstore.DB, id string) {
	t.Helper()
	err := db.InTx(t.Context(), func(tx *memstore.Tx) error {
		return Append(tx, Record{ID: id, TenantID: "tenant-1", Topic: "orders.order_placed.v1", AggregateID: "order-1", OccurredAt: t0})
	})
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
}

func loadRecord(t *testing.T, db *memstore.DB, id string) Record {
	t.Helper()
	var rec Record
	_ = db.InTx(t.Context(), func(tx *memstore.Tx) error {
		return tx.Get(Table, recordKey(Record{ID: id, OccurredAt: t0}), &rec)
	})
	return rec
}

func TestRunOncePublishesAndMarksPublished(t *testing.T) {
	db := memstore.New()
	clock := &fakeClock{now: t0}
	pub := &recordingPublisher{}
	appendRecord(t, db, "event-1")

	n, err := newTestRelay(t, db, pub, clock).RunOnce(t.Context())
	if err != nil || n != 1 {
		t.Fatalf("RunOnce() = %d, %v, want 1, nil", n, err)
	}
	if got := loadRecord(t, db, "event-1").Status; got != StatusPublished {
		t.Fatalf("status = %s, want %s", got, StatusPublished)
	}
	if len(pub.published) != 1 || pub.published[0].ID != "event-1" {
		t.Fatalf("published = %+v, want event-1 once", pub.published)
	}
}

func TestTransientFailureSchedulesBackoffRetry(t *testing.T) {
	db := memstore.New()
	clock := &fakeClock{now: t0}
	pub := &recordingPublisher{errs: []error{errors.New("broker unavailable")}}
	relay := newTestRelay(t, db, pub, clock)
	appendRecord(t, db, "event-1")

	if _, err := relay.RunOnce(t.Context()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	rec := loadRecord(t, db, "event-1")
	if rec.Status != StatusPending || rec.Attempts != 1 || !rec.NextAttemptAt.Equal(t0.Add(time.Second)) {
		t.Fatalf("record = %+v, want pending retry after 1s", rec)
	}

	if n, _ := relay.RunOnce(t.Context()); n != 0 {
		t.Fatalf("published %d before backoff elapsed, want 0", n)
	}
	clock.Advance(time.Second)
	if n, _ := relay.RunOnce(t.Context()); n != 1 {
		t.Fatalf("published %d after backoff, want 1", n)
	}
}

func TestPermanentFailureDeadLettersImmediately(t *testing.T) {
	db := memstore.New()
	clock := &fakeClock{now: t0}
	pub := &recordingPublisher{errs: []error{fmt.Errorf("unknown topic: %w", messaging.ErrPermanent)}}
	appendRecord(t, db, "event-1")

	_, _ = newTestRelay(t, db, pub, clock).RunOnce(t.Context())

	rec := loadRecord(t, db, "event-1")
	if rec.Status != StatusDead || rec.LastError != "permanent" {
		t.Fatalf("record = %+v, want dead-lettered with permanent class", rec)
	}
}

func TestMaxAttemptsDeadLetters(t *testing.T) {
	db := memstore.New()
	clock := &fakeClock{now: t0}
	errUnavailable := errors.New("unavailable")
	pub := &recordingPublisher{errs: []error{errUnavailable, errUnavailable, errUnavailable}}
	relay := newTestRelay(t, db, pub, clock)
	appendRecord(t, db, "event-1")

	for range 3 {
		_, _ = relay.RunOnce(t.Context())
		clock.Advance(time.Hour)
	}

	if got := loadRecord(t, db, "event-1").Status; got != StatusDead {
		t.Fatalf("status = %s, want %s", got, StatusDead)
	}
}

func TestExpiredLeaseIsReclaimedAndStaleOutcomeIgnored(t *testing.T) {
	db := memstore.New()
	clock := &fakeClock{now: t0}
	pub := &recordingPublisher{}
	relay := newTestRelay(t, db, pub, clock)
	appendRecord(t, db, "event-1")

	// A first relay claims the record and stalls past its lease.
	stale, err := relay.claim(t.Context(), "stale-token")
	if err != nil || len(stale) != 1 {
		t.Fatalf("claim() = %v, %v, want one record", stale, err)
	}
	clock.Advance(testConfig().Lease)

	// Another cycle reclaims and publishes it.
	if n, err := relay.RunOnce(t.Context()); err != nil || n != 1 {
		t.Fatalf("RunOnce() = %d, %v, want 1, nil", n, err)
	}
	// The stalled attempt finally reports a failure; it must not overwrite the new outcome.
	if err := relay.recordOutcome(t.Context(), stale[0], "stale-token", errors.New("late failure")); err != nil {
		t.Fatalf("recordOutcome() error = %v", err)
	}
	if got := loadRecord(t, db, "event-1").Status; got != StatusPublished {
		t.Fatalf("status = %s, want %s", got, StatusPublished)
	}
}

func TestConfigValidateRejectsLeaseNotAbovePublishTimeout(t *testing.T) {
	cfg := testConfig()
	cfg.Lease = cfg.PublishTimeout

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() = nil, want error for lease <= publish timeout")
	}
}
