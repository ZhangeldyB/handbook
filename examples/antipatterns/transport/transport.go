// Package transport contrasts handler mistakes with their fixes. Functions
// ending in Bad are intentionally wrong; never copy them.
package transport

import (
	"context"
	"errors"
	"log/slog"
)

type Principal struct{ TenantID, Subject string }

type principalKey struct{}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

type GetOrderRequest struct{ TenantID, OrderID string }

type GetOrder interface {
	Execute(ctx context.Context, tenantID, orderID string) (string, error)
}

var ErrUnauthenticated = errors.New("unauthenticated")

// GetBad trusts the tenant in the request body, so any caller can read any
// tenant's data by changing one field.
func GetBad(ctx context.Context, q GetOrder, req GetOrderRequest) (string, error) {
	return q.Execute(ctx, req.TenantID, req.OrderID)
}

// GetGood derives the tenant from the authenticated principal.
func GetGood(ctx context.Context, q GetOrder, req GetOrderRequest) (string, error) {
	p, ok := PrincipalFrom(ctx)
	if !ok {
		return "", ErrUnauthenticated
	}
	return q.Execute(ctx, p.TenantID, req.OrderID)
}

type Repo interface {
	Load(ctx context.Context, id string) (status string, err error)
	SetStatus(ctx context.Context, id, status string) error
}

// CancelBad decides a business transition and writes storage from the handler.
func CancelBad(ctx context.Context, repo Repo, id string) error {
	status, err := repo.Load(ctx, id)
	if err != nil {
		return err
	}
	if status == "PAID" {
		return errors.New("cannot cancel")
	}
	return repo.SetStatus(ctx, id, "CANCELLED")
}

type CancelCommand interface {
	Execute(ctx context.Context, tenantID, orderID, reason string) error
}

// CancelGood maps the request and calls one application command; the domain
// owns the transition rule.
func CancelGood(ctx context.Context, cancel CancelCommand, orderID, reason string) error {
	p, ok := PrincipalFrom(ctx)
	if !ok {
		return ErrUnauthenticated
	}
	return cancel.Execute(ctx, p.TenantID, orderID, reason)
}

// LoadBad logs and returns the same error; every layer that does this
// duplicates the log line and its severity.
func LoadBad(ctx context.Context, logger *slog.Logger, repo Repo, id string) (string, error) {
	status, err := repo.Load(ctx, id)
	if err != nil {
		logger.ErrorContext(ctx, "load failed", "error", err)
		return "", err
	}
	return status, nil
}
