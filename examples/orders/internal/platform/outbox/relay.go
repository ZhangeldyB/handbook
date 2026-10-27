package outbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"example.com/orders/internal/platform/deps"
	"example.com/orders/internal/platform/memstore"
	"example.com/orders/internal/platform/messaging"
)

// Config tunes the relay. All durations must be positive.
type Config struct {
	// BatchSize bounds how many records one cycle claims.
	BatchSize int
	// PublishTimeout bounds one publish attempt.
	PublishTimeout time.Duration
	// Lease is how long a claim stays exclusive. It must exceed PublishTimeout
	// so a slow attempt cannot overlap a reclaim of the same record.
	Lease time.Duration
	// MaxAttempts moves a record to StatusDead after this many failures.
	MaxAttempts int
	// BaseBackoff and MaxBackoff bound the capped exponential retry delay.
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	// PollInterval is the delay between cycles in Run.
	PollInterval time.Duration
}

// Validate reports configuration that would make the relay unsafe.
func (c Config) Validate() error {
	var errs []error
	if c.BatchSize <= 0 || c.MaxAttempts <= 0 {
		errs = append(errs, errors.New("batch size and max attempts must be positive"))
	}
	if c.PublishTimeout <= 0 || c.Lease <= 0 || c.BaseBackoff <= 0 || c.MaxBackoff <= 0 || c.PollInterval <= 0 {
		errs = append(errs, errors.New("durations must be positive"))
	}
	if c.Lease <= c.PublishTimeout {
		errs = append(errs, errors.New("lease must exceed publish timeout"))
	}
	if c.MaxBackoff < c.BaseBackoff {
		errs = append(errs, errors.New("max backoff must not be below base backoff"))
	}
	return errors.Join(errs...)
}

// RelayDeps are the required collaborators of a Relay.
type RelayDeps struct {
	DB        *memstore.DB
	Publisher messaging.Publisher
	Logger    *slog.Logger
	// Now returns the current time.
	Now func() time.Time
	// NewToken returns a unique claim token.
	NewToken func() string
}

// Relay claims due outbox records, publishes them, and records the outcome.
// Run it from one owned goroutine per process; several replicas may run
// concurrently because claims are leased.
type Relay struct {
	db       *memstore.DB
	pub      messaging.Publisher
	logger   *slog.Logger
	now      func() time.Time
	newToken func() string
	jitter   func(time.Duration) time.Duration
	cfg      Config
}

// NewRelay validates its dependencies and configuration once.
func NewRelay(d RelayDeps, cfg Config) (*Relay, error) {
	if err := errors.Join(
		deps.Require("DB", d.DB),
		deps.Require("Publisher", d.Publisher),
		deps.Require("Logger", d.Logger),
		deps.Require("Now", d.Now),
		deps.Require("NewToken", d.NewToken),
		cfg.Validate(),
	); err != nil {
		return nil, fmt.Errorf("new outbox relay: %w", err)
	}
	return &Relay{
		db:       d.DB,
		pub:      d.Publisher,
		logger:   d.Logger,
		now:      d.Now,
		newToken: d.NewToken,
		jitter:   equalJitter,
		cfg:      cfg,
	}, nil
}

// Run processes batches until ctx is canceled. It stops claiming new work as
// soon as ctx is done and returns after the in-flight batch finishes.
func (r *Relay) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.cfg.PollInterval)
	defer ticker.Stop()
	for {
		if _, err := r.RunOnce(ctx); err != nil && ctx.Err() == nil {
			r.logger.ErrorContext(ctx, "outbox relay cycle failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// RunOnce claims one batch, publishes it, and records each outcome. It returns
// the number of records published successfully.
func (r *Relay) RunOnce(ctx context.Context) (int, error) {
	token := r.newToken()
	claimed, err := r.claim(ctx, token)
	if err != nil {
		return 0, fmt.Errorf("claim outbox batch: %w", err)
	}

	published := 0
	var errs []error
	for _, rec := range claimed {
		pubErr := r.publish(ctx, rec)
		if err := r.recordOutcome(ctx, rec, token, pubErr); err != nil {
			errs = append(errs, fmt.Errorf("record outcome for %q: %w", rec.ID, err))
			continue
		}
		if pubErr == nil {
			published++
		}
	}
	return published, errors.Join(errs...)
}

func (r *Relay) claim(ctx context.Context, token string) ([]Record, error) {
	now := r.now()
	var claimed []Record
	err := r.db.InTx(ctx, func(tx *memstore.Tx) error {
		claimed = claimed[:0]
		for _, key := range tx.Keys(Table) {
			if len(claimed) == r.cfg.BatchSize {
				break
			}
			var rec Record
			if err := tx.Get(Table, key, &rec); err != nil {
				return err
			}
			if !isDue(rec, now) {
				continue
			}
			rec.Status = StatusClaimed
			rec.ClaimToken = token
			rec.ClaimedUntil = now.Add(r.cfg.Lease)
			if err := tx.Put(Table, key, rec); err != nil {
				return err
			}
			claimed = append(claimed, rec)
		}
		return nil
	})
	return claimed, err
}

func isDue(rec Record, now time.Time) bool {
	switch rec.Status {
	case StatusPending:
		return !rec.NextAttemptAt.After(now)
	case StatusClaimed:
		return !rec.ClaimedUntil.After(now) // lease expired: the claimer died or stalled
	default:
		return false
	}
}

func (r *Relay) publish(ctx context.Context, rec Record) error {
	ctx, cancel := context.WithTimeout(ctx, r.cfg.PublishTimeout)
	defer cancel()
	return r.pub.Publish(ctx, messaging.Message{
		ID:         rec.ID,
		Topic:      rec.Topic,
		TenantID:   rec.TenantID,
		Key:        rec.AggregateID,
		Payload:    rec.Payload,
		OccurredAt: rec.OccurredAt,
	})
}

// recordOutcome writes the attempt result only while this relay still owns the
// claim. A record whose lease expired and was reclaimed is left to its new
// owner.
func (r *Relay) recordOutcome(ctx context.Context, claimed Record, token string, pubErr error) error {
	now := r.now()
	key := recordKey(claimed)
	// Record the outcome even when shutdown canceled ctx mid-publish, but
	// bound the write so shutdown cannot hang on it.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.cfg.PublishTimeout)
	defer cancel()
	return r.db.InTx(writeCtx, func(tx *memstore.Tx) error {
		var rec Record
		if err := tx.Get(Table, key, &rec); err != nil {
			return err
		}
		if rec.Status != StatusClaimed || rec.ClaimToken != token {
			return nil // lost the lease; the new owner records its own outcome
		}
		rec.ClaimToken = ""
		rec.ClaimedUntil = time.Time{}
		if pubErr == nil {
			rec.Status = StatusPublished
			rec.PublishedAt = now
			rec.LastError = ""
			return tx.Put(Table, key, rec)
		}

		rec.Attempts++
		rec.LastError = classify(pubErr)
		if errors.Is(pubErr, messaging.ErrPermanent) || rec.Attempts >= r.cfg.MaxAttempts {
			rec.Status = StatusDead
			r.logger.ErrorContext(ctx, "outbox record dead-lettered",
				"event_id", rec.ID, "topic", rec.Topic, "attempts", rec.Attempts, "reason", rec.LastError)
		} else {
			rec.Status = StatusPending
			rec.NextAttemptAt = now.Add(r.backoff(rec.Attempts))
		}
		return tx.Put(Table, key, rec)
	})
}

// backoff returns a capped exponential delay with jitter for the given attempt
// number (1-based).
func (r *Relay) backoff(attempt int) time.Duration {
	delay := r.cfg.BaseBackoff
	for i := 1; i < attempt && delay < r.cfg.MaxBackoff; i++ {
		delay *= 2
	}
	return r.jitter(min(delay, r.cfg.MaxBackoff))
}

// equalJitter returns a random delay in [d/2, d].
func equalJitter(d time.Duration) time.Duration {
	half := d / 2
	return half + rand.N(half+1) //nolint:gosec // retry jitter needs spread, not unpredictability
}

// classify stores a bounded, non-sensitive failure class instead of raw error
// text, which may contain payload fragments or credentials.
func classify(err error) string {
	switch {
	case errors.Is(err, messaging.ErrPermanent):
		return "permanent"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout_outcome_unknown"
	case errors.Is(err, context.Canceled):
		return "canceled"
	default:
		return "transient"
	}
}
