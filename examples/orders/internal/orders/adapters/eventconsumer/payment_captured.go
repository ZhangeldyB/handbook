// Package eventconsumer adapts messages from other bounded contexts into
// orders application commands. Handlers decode and validate the foreign
// contract, call exactly one command, and classify the outcome; they contain no
// business rules and no storage access.
package eventconsumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"example.com/orders/internal/orders/domain"
	"example.com/orders/internal/orders/ports"
	"example.com/orders/internal/orders/usecases"
	"example.com/orders/internal/platform/deps"
	"example.com/orders/internal/platform/messaging"
)

// TopicPaymentCaptured is the payments context's published event.
const TopicPaymentCaptured = "payments.payment_captured.v1"

// paymentCapturedV1 is the foreign wire contract. It is translated here and
// never passed inward.
type paymentCapturedV1 struct {
	PaymentID string `json:"payment_id"`
	OrderID   string `json:"order_id"`
}

type markPaidCommand interface {
	Execute(ctx context.Context, req usecases.MarkPaidRequest) error
}

// PaymentCaptured marks orders paid when the payments context captures a
// payment.
type PaymentCaptured struct {
	markPaid markPaidCommand
}

var _ messaging.Handler = (*PaymentCaptured)(nil)

// NewPaymentCaptured validates its dependency once.
func NewPaymentCaptured(markPaid markPaidCommand) (*PaymentCaptured, error) {
	if err := deps.Require("MarkPaid", markPaid); err != nil {
		return nil, fmt.Errorf("new payment captured handler: %w", err)
	}
	return &PaymentCaptured{markPaid: markPaid}, nil
}

// Handle implements messaging.Handler. Malformed messages and commands that can
// never succeed unchanged are permanent (dead-lettered for investigation);
// everything else is retried.
func (h *PaymentCaptured) Handle(ctx context.Context, msg messaging.Message) error {
	var payload paymentCapturedV1
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode %s %q: %w: %w", msg.Topic, msg.ID, messaging.ErrPermanent, err)
	}
	if msg.ID == "" || msg.TenantID == "" || payload.OrderID == "" || payload.PaymentID == "" {
		return fmt.Errorf("validate %s %q: missing identity: %w", msg.Topic, msg.ID, messaging.ErrPermanent)
	}

	err := h.markPaid.Execute(ctx, usecases.MarkPaidRequest{
		TenantID:  msg.TenantID,
		OrderID:   payload.OrderID,
		PaymentID: payload.PaymentID,
		EventID:   msg.ID,
	})
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ports.ErrNotFound), errors.Is(err, domain.ErrInvalidTransition):
		// A payment for an unknown or non-payable order will not succeed on
		// redelivery. Dead-letter it so an operator can investigate and replay.
		return fmt.Errorf("handle %s %q: %w: %w", msg.Topic, msg.ID, messaging.ErrPermanent, err)
	default:
		return fmt.Errorf("handle %s %q: %w", msg.Topic, msg.ID, err)
	}
}
