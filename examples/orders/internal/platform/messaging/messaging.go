// Package messaging defines the broker-agnostic message envelope and the
// publish/handle contracts used by the outbox relay and event consumers.
//
// Delivery is at least once: a Message may be delivered more than once and out
// of order. Handlers must be idempotent on Message.ID.
package messaging

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrPermanent marks a failure that cannot succeed if retried unchanged, such as
// a malformed payload or an unknown topic. Wrap it with %w so callers can
// classify the failure with errors.Is.
var ErrPermanent = errors.New("permanent failure")

// Message is one published event.
type Message struct {
	// ID is the stable event ID assigned when the event was recorded. It never
	// changes across redeliveries.
	ID string
	// Topic is the versioned event type, for example "orders.order_placed.v1".
	Topic string
	// TenantID scopes the event. Consumers validate it like any other input.
	TenantID string
	// Key is the ordering/partition key, normally the aggregate ID.
	Key string
	// Payload is the encoded event body.
	Payload []byte
	// OccurredAt is when the domain transition happened.
	OccurredAt time.Time
}

// Publisher sends a message to the broker. A returned error does not prove the
// broker rejected the message: on a timeout the outcome is unknown and the
// message may still be delivered.
type Publisher interface {
	Publish(ctx context.Context, msg Message) error
}

// Handler processes one delivered message. Returning nil acknowledges it;
// returning an error that wraps ErrPermanent dead-letters it; any other error
// requests redelivery.
type Handler interface {
	Handle(ctx context.Context, msg Message) error
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(ctx context.Context, msg Message) error

// Handle calls f.
func (f HandlerFunc) Handle(ctx context.Context, msg Message) error { return f(ctx, msg) }

// Router is an in-process Publisher that delivers each message synchronously to
// the handlers registered for its topic. It stands in for a real broker in the
// example and in tests.
type Router struct {
	handlers map[string][]Handler
}

// NewRouter returns an empty Router.
func NewRouter() *Router {
	return &Router{handlers: make(map[string][]Handler)}
}

// Subscribe registers h for topic. Registration happens during startup, before
// any Publish call.
func (r *Router) Subscribe(topic string, h Handler) {
	r.handlers[topic] = append(r.handlers[topic], h)
}

// Publish delivers msg to every handler subscribed to its topic and returns the
// first handler error. A topic without subscribers is a permanent failure:
// silently dropping it would hide a missing registration.
func (r *Router) Publish(ctx context.Context, msg Message) error {
	handlers := r.handlers[msg.Topic]
	if len(handlers) == 0 {
		return fmt.Errorf("route topic %q: no subscribers: %w", msg.Topic, ErrPermanent)
	}
	for _, h := range handlers {
		if err := h.Handle(ctx, msg); err != nil {
			return fmt.Errorf("deliver %q to subscriber: %w", msg.Topic, err)
		}
	}
	return nil
}
