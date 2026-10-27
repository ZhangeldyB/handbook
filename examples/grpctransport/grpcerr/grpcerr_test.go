package grpcerr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	errOrderNotFound     = errors.New("order not found")
	errInvalidTransition = errors.New("invalid transition")
	errConcurrent        = errors.New("concurrent modification")
)

func testMapper() *Mapper {
	return NewMapper(
		Rule{Target: errOrderNotFound, Code: codes.NotFound, Message: "order not found"},
		Rule{Target: errInvalidTransition, Code: codes.FailedPrecondition, Message: "order is not in a valid state for this operation"},
		Rule{Target: errConcurrent, Code: codes.Aborted, Message: "order was modified concurrently; retry"},
	)
}

func TestStatusMapsByRule(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantCode     codes.Code
		wantExpected bool
	}{
		{name: "wrapped domain error", err: fmt.Errorf("place order %q: %w", "o-1", errInvalidTransition), wantCode: codes.FailedPrecondition, wantExpected: true},
		{name: "not found", err: errOrderNotFound, wantCode: codes.NotFound, wantExpected: true},
		{name: "conflict", err: errConcurrent, wantCode: codes.Aborted, wantExpected: true},
		{name: "cancellation passes through", err: fmt.Errorf("load: %w", context.Canceled), wantCode: codes.Canceled, wantExpected: true},
		{name: "deadline passes through", err: context.DeadlineExceeded, wantCode: codes.DeadlineExceeded, wantExpected: true},
		{name: "explicit status is preserved", err: status.Error(codes.InvalidArgument, "order_id is required"), wantCode: codes.InvalidArgument, wantExpected: true},
		{name: "unknown error is internal", err: errors.New("connection reset by 10.0.0.7"), wantCode: codes.Internal, wantExpected: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, expected := testMapper().Status(tt.err)
			if st.Code() != tt.wantCode || expected != tt.wantExpected {
				t.Fatalf("Status() = %v, %v; want %v, %v", st.Code(), expected, tt.wantCode, tt.wantExpected)
			}
		})
	}
}

func TestInternalErrorsAreSanitizedAndLoggedOnce(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	interceptor := testMapper().UnaryServerInterceptor(logger)
	secret := errors.New("dial db at 10.0.0.7: password rejected")

	_, err := interceptor(t.Context(), nil, &grpc.UnaryServerInfo{FullMethod: "/orders.v1.OrderService/PlaceOrder"},
		func(context.Context, any) (any, error) { return nil, secret })

	st, _ := status.FromError(err)
	if st.Code() != codes.Internal || strings.Contains(st.Message(), "10.0.0.7") {
		t.Fatalf("status = %v %q, want sanitized Internal", st.Code(), st.Message())
	}
	if got := strings.Count(logs.String(), "unexpected rpc failure"); got != 1 {
		t.Fatalf("logged %d times, want 1", got)
	}
}

func TestExpectedErrorsAreNotLoggedAsFailures(t *testing.T) {
	var logs bytes.Buffer
	interceptor := testMapper().UnaryServerInterceptor(slog.New(slog.NewTextHandler(&logs, nil)))

	_, err := interceptor(t.Context(), nil, &grpc.UnaryServerInfo{FullMethod: "/x"},
		func(context.Context, any) (any, error) { return nil, errOrderNotFound })

	if status.Code(err) != codes.NotFound || logs.Len() != 0 {
		t.Fatalf("code = %v, logs = %q; want NotFound and no log", status.Code(err), logs.String())
	}
}
