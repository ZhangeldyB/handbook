package usecases

import (
	"context"
	"errors"
	"fmt"

	"example.com/orders/internal/orders/ports"
	"example.com/orders/internal/platform/deps"
)

// PlaceDeps are the required collaborators of Place.
type PlaceDeps struct {
	Orders ports.OrderRepository
	Clock  ports.Clock
	IDs    ports.IDGenerator
}

// Place submits a draft order.
type Place struct {
	orders ports.OrderRepository
	clock  ports.Clock
	ids    ports.IDGenerator
}

// NewPlace validates its dependencies once.
func NewPlace(d PlaceDeps) (*Place, error) {
	if err := errors.Join(
		deps.Require("Orders", d.Orders),
		deps.Require("Clock", d.Clock),
		deps.Require("IDs", d.IDs),
	); err != nil {
		return nil, fmt.Errorf("new place: %w", err)
	}
	return &Place{orders: d.Orders, clock: d.Clock, ids: d.IDs}, nil
}

// PlaceRequest identifies the order to place.
type PlaceRequest struct {
	TenantID string
	OrderID  string
}

// Execute places the order and saves it with its OrderPlaced event.
func (c *Place) Execute(ctx context.Context, req PlaceRequest) error {
	order, err := c.orders.Get(ctx, req.TenantID, req.OrderID)
	if err != nil {
		return fmt.Errorf("get order %q: %w", req.OrderID, err)
	}
	if err := order.Place(c.ids.NewID(), c.clock.Now()); err != nil {
		return err
	}
	if err := c.orders.Save(ctx, order); err != nil {
		return fmt.Errorf("save order %q: %w", req.OrderID, err)
	}
	return nil
}
