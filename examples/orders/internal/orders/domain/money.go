package domain

import (
	"errors"
	"fmt"
	"math"
)

// ErrCurrencyMismatch is returned when money values of different currencies
// are combined.
var ErrCurrencyMismatch = errors.New("currency mismatch")

// ErrAmountOverflow is returned when an arithmetic result does not fit.
var ErrAmountOverflow = errors.New("amount overflow")

// Currency is an ISO 4217 alphabetic code such as "EUR".
type Currency string

// Valid reports whether c has the shape of an ISO 4217 code.
func (c Currency) Valid() bool {
	if len(c) != 3 {
		return false
	}
	for _, r := range c {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// Money is an exact amount in integer minor units (cents for EUR) of one
// currency. The zero Money has no currency and is only a placeholder.
type Money struct {
	minor    int64
	currency Currency
}

// NewMoney returns minor units of currency.
func NewMoney(minor int64, currency Currency) (Money, error) {
	if !currency.Valid() {
		return Money{}, fmt.Errorf("new money: invalid currency %q", currency)
	}
	return Money{minor: minor, currency: currency}, nil
}

// Zero returns zero minor units of currency.
func Zero(currency Currency) Money { return Money{currency: currency} }

// Minor returns the amount in minor units.
func (m Money) Minor() int64 { return m.minor }

// Currency returns the currency.
func (m Money) Currency() Currency { return m.currency }

// Add returns m + other. Both must have the same currency.
func (m Money) Add(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("add %s to %s: %w", other.currency, m.currency, ErrCurrencyMismatch)
	}
	if (other.minor > 0 && m.minor > math.MaxInt64-other.minor) ||
		(other.minor < 0 && m.minor < math.MinInt64-other.minor) {
		return Money{}, fmt.Errorf("add money: %w", ErrAmountOverflow)
	}
	return Money{minor: m.minor + other.minor, currency: m.currency}, nil
}

// Times returns m multiplied by a non-negative quantity.
func (m Money) Times(quantity int64) (Money, error) {
	if quantity < 0 {
		return Money{}, fmt.Errorf("multiply money by negative quantity %d", quantity)
	}
	if quantity != 0 && (m.minor > math.MaxInt64/quantity || m.minor < math.MinInt64/quantity) {
		return Money{}, fmt.Errorf("multiply money: %w", ErrAmountOverflow)
	}
	return Money{minor: m.minor * quantity, currency: m.currency}, nil
}

// String formats m for diagnostics, for example "1234 EUR (minor units)".
func (m Money) String() string {
	return fmt.Sprintf("%d %s (minor units)", m.minor, m.currency)
}
