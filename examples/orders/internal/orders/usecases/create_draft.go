// Package usecases holds the orders application commands. Each command loads
// aggregates through ports, calls domain behavior, and saves through the
// repository; it contains no transport, storage, or broker code.
package usecases

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"example.com/orders/internal/orders/domain"
	"example.com/orders/internal/orders/ports"
	"example.com/orders/internal/platform/deps"
)

const scopeCreateDraft = "CreateDraft"

// ErrIdempotencyKeyReused means a client reused an idempotency key with a
// different request payload.
var ErrIdempotencyKeyReused = errors.New("idempotency key reused with a different request")

// CreateDraftDeps are the required collaborators of CreateDraft.
type CreateDraftDeps struct {
	Orders      ports.OrderRepository
	Idempotency ports.IdempotencyStore
	Clock       ports.Clock
	IDs         ports.IDGenerator
}

// CreateDraft creates a draft order. A request with an idempotency key returns
// the originally created order ID when the client retries.
type CreateDraft struct {
	orders      ports.OrderRepository
	idempotency ports.IdempotencyStore
	clock       ports.Clock
	ids         ports.IDGenerator
}

// NewCreateDraft validates its dependencies once.
func NewCreateDraft(d CreateDraftDeps) (*CreateDraft, error) {
	if err := errors.Join(
		deps.Require("Orders", d.Orders),
		deps.Require("Idempotency", d.Idempotency),
		deps.Require("Clock", d.Clock),
		deps.Require("IDs", d.IDs),
	); err != nil {
		return nil, fmt.Errorf("new create draft: %w", err)
	}
	return &CreateDraft{orders: d.Orders, idempotency: d.Idempotency, clock: d.Clock, ids: d.IDs}, nil
}

// LineInput is one requested order line.
type LineInput struct {
	SKU       string
	Quantity  int64
	UnitPrice int64 // minor units of the order currency
}

// CreateDraftRequest is the application input. TenantID comes from the
// authenticated principal, never from the request body.
type CreateDraftRequest struct {
	TenantID       string
	CustomerID     string
	Currency       string
	Lines          []LineInput
	IdempotencyKey string // optional
}

// CreateDraftResult is returned to the caller.
type CreateDraftResult struct {
	OrderID string
}

// Execute creates the draft, or returns the original result for a replayed
// idempotency key.
func (c *CreateDraft) Execute(ctx context.Context, req CreateDraftRequest) (CreateDraftResult, error) {
	fingerprint := fingerprintOf(req)
	if req.IdempotencyKey != "" {
		if result, ok, err := c.replay(ctx, req, fingerprint); err != nil || ok {
			return result, err
		}
	}

	order, err := c.buildOrder(req)
	if err != nil {
		return CreateDraftResult{}, err
	}

	var records []ports.IdempotencyRecord
	if req.IdempotencyKey != "" {
		records = append(records, ports.IdempotencyRecord{
			Scope:       scopeCreateDraft,
			Key:         req.IdempotencyKey,
			Fingerprint: fingerprint,
			Response:    order.ID(),
		})
	}

	err = c.orders.Save(ctx, order, records...)
	if errors.Is(err, ports.ErrDuplicate) {
		// A concurrent request with the same key won the race; return its result.
		if result, ok, replayErr := c.replay(ctx, req, fingerprint); replayErr != nil || ok {
			return result, replayErr
		}
	}
	if err != nil {
		return CreateDraftResult{}, fmt.Errorf("save draft order %q: %w", order.ID(), err)
	}
	return CreateDraftResult{OrderID: order.ID()}, nil
}

func (c *CreateDraft) buildOrder(req CreateDraftRequest) (*domain.Order, error) {
	order, err := domain.NewOrder(domain.NewOrderParams{
		TenantID:   req.TenantID,
		OrderID:    c.ids.NewID(),
		CustomerID: req.CustomerID,
		Currency:   domain.Currency(req.Currency),
		Now:        c.clock.Now(),
	})
	if err != nil {
		return nil, err
	}
	for i, in := range req.Lines {
		price, err := domain.NewMoney(in.UnitPrice, domain.Currency(req.Currency))
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i, err)
		}
		line, err := domain.NewLine(in.SKU, in.Quantity, price)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i, err)
		}
		if err := order.AddLine(line); err != nil {
			return nil, fmt.Errorf("line %d: %w", i, err)
		}
	}
	return order, nil
}

func (c *CreateDraft) replay(ctx context.Context, req CreateDraftRequest, fingerprint string) (CreateDraftResult, bool, error) {
	record, ok, err := c.idempotency.Lookup(ctx, req.TenantID, scopeCreateDraft, req.IdempotencyKey)
	if err != nil {
		return CreateDraftResult{}, false, fmt.Errorf("look up idempotency key: %w", err)
	}
	if !ok {
		return CreateDraftResult{}, false, nil
	}
	if record.Fingerprint != fingerprint {
		return CreateDraftResult{}, false, ErrIdempotencyKeyReused
	}
	return CreateDraftResult{OrderID: record.Response}, true, nil
}

// fingerprintOf hashes the fields that define the request's meaning. The
// idempotency key itself is excluded.
func fingerprintOf(req CreateDraftRequest) string {
	h := sha256.New()
	write := func(s string) { _, _ = h.Write([]byte(strconv.Quote(s))) }
	write(req.TenantID)
	write(req.CustomerID)
	write(req.Currency)
	for _, line := range req.Lines {
		write(line.SKU)
		write(strconv.FormatInt(line.Quantity, 10))
		write(strconv.FormatInt(line.UnitPrice, 10))
	}
	return hex.EncodeToString(h.Sum(nil))
}
