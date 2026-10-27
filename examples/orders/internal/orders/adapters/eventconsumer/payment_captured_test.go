package eventconsumer

import (
	"context"
	"errors"
	"testing"

	"example.com/orders/internal/orders/domain"
	"example.com/orders/internal/orders/ports"
	"example.com/orders/internal/orders/usecases"
	"example.com/orders/internal/platform/messaging"
)

type markPaidStub struct {
	err   error
	calls []usecases.MarkPaidRequest
}

func (s *markPaidStub) Execute(_ context.Context, req usecases.MarkPaidRequest) error {
	s.calls = append(s.calls, req)
	return s.err
}

func validMessage() messaging.Message {
	return messaging.Message{
		ID:       "event-1",
		Topic:    TopicPaymentCaptured,
		TenantID: "tenant-1",
		Payload:  []byte(`{"payment_id":"pay-1","order_id":"order-1"}`),
	}
}

func TestHandleMapsMessageToCommand(t *testing.T) {
	stub := &markPaidStub{}
	handler, _ := NewPaymentCaptured(stub)

	if err := handler.Handle(t.Context(), validMessage()); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	want := usecases.MarkPaidRequest{TenantID: "tenant-1", OrderID: "order-1", PaymentID: "pay-1", EventID: "event-1"}
	if len(stub.calls) != 1 || stub.calls[0] != want {
		t.Fatalf("calls = %+v, want [%+v]", stub.calls, want)
	}
}

func TestHandleClassifiesOutcomes(t *testing.T) {
	malformed := validMessage()
	malformed.Payload = []byte(`{not json`)
	missingTenant := validMessage()
	missingTenant.TenantID = ""

	tests := []struct {
		name          string
		msg           messaging.Message
		commandErr    error
		wantPermanent bool
		wantCalls     int
	}{
		{name: "malformed payload", msg: malformed, wantPermanent: true},
		{name: "missing tenant", msg: missingTenant, wantPermanent: true},
		{name: "unknown order", msg: validMessage(), commandErr: ports.ErrNotFound, wantPermanent: true, wantCalls: 1},
		{name: "order not payable", msg: validMessage(), commandErr: domain.ErrInvalidTransition, wantPermanent: true, wantCalls: 1},
		{name: "version conflict is retried", msg: validMessage(), commandErr: ports.ErrConcurrentModification, wantCalls: 1},
		{name: "storage outage is retried", msg: validMessage(), commandErr: errors.New("unavailable"), wantCalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &markPaidStub{err: tt.commandErr}
			handler, _ := NewPaymentCaptured(stub)

			err := handler.Handle(t.Context(), tt.msg)

			if err == nil {
				t.Fatal("Handle() error = nil, want an error")
			}
			if got := errors.Is(err, messaging.ErrPermanent); got != tt.wantPermanent {
				t.Errorf("permanent = %v, want %v (err: %v)", got, tt.wantPermanent, err)
			}
			if len(stub.calls) != tt.wantCalls {
				t.Errorf("command calls = %d, want %d", len(stub.calls), tt.wantCalls)
			}
		})
	}
}
