package persistence

import (
	"errors"
	"fmt"
	"time"

	"example.com/orders/internal/orders/domain"
)

// orderRow is the stored shape of an order. Field names are part of the
// storage contract: renaming one is a schema migration.
type orderRow struct {
	TenantID     string    `json:"tenant_id"`
	OrderID      string    `json:"order_id"`
	CustomerID   string    `json:"customer_id"`
	Currency     string    `json:"currency"`
	Status       string    `json:"status"`
	Lines        []lineRow `json:"lines"`
	TotalMinor   int64     `json:"total_minor"` // denormalized for the read side
	CreatedAt    time.Time `json:"created_at"`
	PlacedAt     time.Time `json:"placed_at,omitzero"`
	PaymentID    string    `json:"payment_id,omitempty"`
	CancelReason string    `json:"cancel_reason,omitempty"`
	Version      int64     `json:"version"`
}

type lineRow struct {
	SKU            string `json:"sku"`
	Quantity       int64  `json:"quantity"`
	UnitPriceMinor int64  `json:"unit_price_minor"`
}

// toRow maps every Snapshot field. version is the version being written.
func toRow(s domain.Snapshot, total domain.Money, version int64) orderRow {
	lines := make([]lineRow, 0, len(s.Lines))
	for _, line := range s.Lines {
		lines = append(lines, lineRow{
			SKU:            line.SKU(),
			Quantity:       line.Quantity(),
			UnitPriceMinor: line.UnitPrice().Minor(),
		})
	}
	return orderRow{
		TenantID:     s.TenantID,
		OrderID:      s.OrderID,
		CustomerID:   s.CustomerID,
		Currency:     string(s.Currency),
		Status:       string(s.Status),
		Lines:        lines,
		TotalMinor:   total.Minor(),
		CreatedAt:    s.CreatedAt,
		PlacedAt:     s.PlacedAt,
		PaymentID:    s.PaymentID,
		CancelReason: s.CancelReason,
		Version:      version,
	}
}

// toSnapshot maps every stored field back, validating values instead of
// trusting them.
func toSnapshot(row orderRow) (domain.Snapshot, error) {
	status, err := domain.ParseStatus(row.Status)
	if err != nil {
		return domain.Snapshot{}, err
	}
	currency := domain.Currency(row.Currency)
	lines := make([]domain.Line, 0, len(row.Lines))
	var errs []error
	for i, lr := range row.Lines {
		price, err := domain.NewMoney(lr.UnitPriceMinor, currency)
		if err != nil {
			errs = append(errs, fmt.Errorf("line %d: %w", i, err))
			continue
		}
		line, err := domain.NewLine(lr.SKU, lr.Quantity, price)
		if err != nil {
			errs = append(errs, fmt.Errorf("line %d: %w", i, err))
			continue
		}
		lines = append(lines, line)
	}
	if err := errors.Join(errs...); err != nil {
		return domain.Snapshot{}, fmt.Errorf("map order %q lines: %w", row.OrderID, err)
	}
	return domain.Snapshot{
		TenantID:     row.TenantID,
		OrderID:      row.OrderID,
		CustomerID:   row.CustomerID,
		Currency:     currency,
		Status:       status,
		Lines:        lines,
		CreatedAt:    row.CreatedAt,
		PlacedAt:     row.PlacedAt,
		PaymentID:    row.PaymentID,
		CancelReason: row.CancelReason,
		Version:      row.Version,
	}, nil
}
