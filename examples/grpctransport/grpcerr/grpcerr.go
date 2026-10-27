// Package grpcerr maps application errors to gRPC status codes in one place.
//
// Handlers return application and domain errors unchanged. A single server
// interceptor translates them using the service's rule table, sanitizes the
// message, and logs unexpected failures exactly once.
package grpcerr

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Rule maps errors matching Target (with errors.Is) to Code. Message is the
// public, static text sent to clients; the original error text never leaves
// the process because it may contain internal details.
type Rule struct {
	Target  error
	Code    codes.Code
	Message string
}

// Mapper translates errors with an ordered rule table.
type Mapper struct {
	rules []Rule
}

// NewMapper returns a Mapper. Rules are evaluated in order; put more specific
// targets first.
func NewMapper(rules ...Rule) *Mapper {
	return &Mapper{rules: rules}
}

// Status returns the public status for err. It reports expected=false when err
// did not match a rule and was mapped to codes.Internal, which callers log.
func (m *Mapper) Status(err error) (st *status.Status, expected bool) {
	if err == nil {
		return status.New(codes.OK, ""), true
	}
	if st, ok := status.FromError(err); ok {
		return st, true // already a deliberate status from the handler
	}
	switch {
	case errors.Is(err, context.Canceled):
		return status.New(codes.Canceled, "request canceled"), true
	case errors.Is(err, context.DeadlineExceeded):
		return status.New(codes.DeadlineExceeded, "deadline exceeded"), true
	}
	for _, rule := range m.rules {
		if errors.Is(err, rule.Target) {
			return status.New(rule.Code, rule.Message), true
		}
	}
	return status.New(codes.Internal, "internal error"), false
}

// UnaryServerInterceptor maps handler errors to statuses and logs unmapped
// errors once, with the full internal error, at the transport boundary.
func (m *Mapper) UnaryServerInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}
		st, expected := m.Status(err)
		if !expected {
			logger.ErrorContext(ctx, "unexpected rpc failure", "method", info.FullMethod, "error", err)
		}
		return nil, st.Err()
	}
}
