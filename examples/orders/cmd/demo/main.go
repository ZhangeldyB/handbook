// Command demo wires the orders example end to end: it creates and places an
// order, relays the OrderPlaced event through the outbox, and consumes a
// duplicated payment event idempotently.
//
// This file is the composition root: it is the only place that knows every
// concrete type.
package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"example.com/orders/internal/orders/adapters/eventconsumer"
	"example.com/orders/internal/orders/adapters/persistence"
	"example.com/orders/internal/orders/queries"
	"example.com/orders/internal/orders/usecases"
	"example.com/orders/internal/platform/memstore"
	"example.com/orders/internal/platform/messaging"
	"example.com/orders/internal/platform/outbox"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(context.Background(), logger); err != nil {
		logger.Error("demo failed", "error", err)
		os.Exit(1)
	}
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type randomIDs struct{}

func (randomIDs) NewID() string { return rand.Text() }

func run(ctx context.Context, logger *slog.Logger) error {
	db := memstore.New()
	repo, err := persistence.New(db)
	if err != nil {
		return err
	}
	clock, ids := systemClock{}, randomIDs{}

	createDraft, err := usecases.NewCreateDraft(usecases.CreateDraftDeps{Orders: repo, Idempotency: repo, Clock: clock, IDs: ids})
	if err != nil {
		return err
	}
	place, err := usecases.NewPlace(usecases.PlaceDeps{Orders: repo, Clock: clock, IDs: ids})
	if err != nil {
		return err
	}
	markPaid, err := usecases.NewMarkPaid(usecases.MarkPaidDeps{Orders: repo, Clock: clock, IDs: ids})
	if err != nil {
		return err
	}
	reads, err := queries.New(repo)
	if err != nil {
		return err
	}

	// The in-process router stands in for a broker. A fake payments service
	// reacts to OrderPlaced by capturing payment and publishing an event that
	// the orders consumer handles.
	router := messaging.NewRouter()
	paymentCaptured, err := eventconsumer.NewPaymentCaptured(markPaid)
	if err != nil {
		return err
	}
	router.Subscribe(eventconsumer.TopicPaymentCaptured, paymentCaptured)
	router.Subscribe(persistence.TopicOrderPlaced, fakePaymentsService(router))
	router.Subscribe(persistence.TopicOrderPaid, messaging.HandlerFunc(func(ctx context.Context, msg messaging.Message) error {
		logger.InfoContext(ctx, "notification sent", "event_id", msg.ID, "topic", msg.Topic)
		return nil
	}))

	relay, err := outbox.NewRelay(outbox.RelayDeps{
		DB: db, Publisher: router, Logger: logger, Now: clock.Now, NewToken: ids.NewID,
	}, outbox.Config{
		BatchSize: 100, PublishTimeout: 2 * time.Second, Lease: 10 * time.Second, MaxAttempts: 5,
		BaseBackoff: time.Second, MaxBackoff: time.Minute, PollInterval: time.Second,
	})
	if err != nil {
		return err
	}

	const tenantID = "tenant-1"
	req := usecases.CreateDraftRequest{
		TenantID: tenantID, CustomerID: "customer-42", Currency: "EUR",
		Lines:          []usecases.LineInput{{SKU: "book", Quantity: 2, UnitPrice: 1999}},
		IdempotencyKey: "client-key-1",
	}
	created, err := createDraft.Execute(ctx, req)
	if err != nil {
		return fmt.Errorf("create draft: %w", err)
	}
	retried, err := createDraft.Execute(ctx, req) // a client retry after a timeout
	if err != nil {
		return fmt.Errorf("retry create draft: %w", err)
	}
	logger.InfoContext(ctx, "draft created", "order_id", created.OrderID, "retry_returned_same_order", retried.OrderID == created.OrderID)

	if err := place.Execute(ctx, usecases.PlaceRequest{TenantID: tenantID, OrderID: created.OrderID}); err != nil {
		return fmt.Errorf("place order: %w", err)
	}

	// Two relay cycles: the first publishes OrderPlaced (which triggers the
	// payment event and marks the order paid), the second publishes OrderPaid.
	for range 2 {
		if _, err := relay.RunOnce(ctx); err != nil {
			return fmt.Errorf("relay outbox: %w", err)
		}
	}

	view, err := reads.Get(ctx, tenantID, created.OrderID)
	if err != nil {
		return fmt.Errorf("get order: %w", err)
	}
	logger.InfoContext(ctx, "order state", "order_id", view.OrderID, "status", view.Status, "total_minor", view.TotalMinor, "currency", view.Currency)
	return nil
}

// fakePaymentsService captures payment for every placed order and delivers the
// resulting event twice, simulating at-least-once redelivery.
func fakePaymentsService(router *messaging.Router) messaging.Handler {
	return messaging.HandlerFunc(func(ctx context.Context, msg messaging.Message) error {
		var placed struct {
			OrderID string `json:"order_id"`
		}
		if err := json.Unmarshal(msg.Payload, &placed); err != nil {
			return errors.Join(messaging.ErrPermanent, err)
		}
		payload, err := json.Marshal(map[string]string{"payment_id": "pay-" + placed.OrderID, "order_id": placed.OrderID})
		if err != nil {
			return err
		}
		captured := messaging.Message{
			ID: "payment-event-" + placed.OrderID, Topic: eventconsumer.TopicPaymentCaptured,
			TenantID: msg.TenantID, Key: placed.OrderID, Payload: payload, OccurredAt: time.Now().UTC(),
		}
		for range 2 {
			if err := router.Publish(ctx, captured); err != nil {
				return err
			}
		}
		return nil
	})
}
