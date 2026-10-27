package domain

import (
	"errors"
	"math"
	"testing"
)

func TestMoneyAdd(t *testing.T) {
	a, _ := NewMoney(150, "EUR")
	b, _ := NewMoney(275, "EUR")

	sum, err := a.Add(b)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if got, want := sum.Minor(), int64(425); got != want {
		t.Fatalf("Add() = %d, want %d", got, want)
	}
}

func TestMoneyAddRejectsCurrencyMismatch(t *testing.T) {
	eur, _ := NewMoney(1, "EUR")
	usd, _ := NewMoney(1, "USD")

	if _, err := eur.Add(usd); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("Add() error = %v, want %v", err, ErrCurrencyMismatch)
	}
}

func TestMoneyOverflow(t *testing.T) {
	large, _ := NewMoney(math.MaxInt64, "EUR")
	one, _ := NewMoney(1, "EUR")

	if _, err := large.Add(one); !errors.Is(err, ErrAmountOverflow) {
		t.Errorf("Add() error = %v, want %v", err, ErrAmountOverflow)
	}
	if _, err := large.Times(2); !errors.Is(err, ErrAmountOverflow) {
		t.Errorf("Times() error = %v, want %v", err, ErrAmountOverflow)
	}
}

func TestNewMoneyRejectsInvalidCurrency(t *testing.T) {
	for _, currency := range []Currency{"", "eur", "EURO", "E1R"} {
		if _, err := NewMoney(1, currency); err == nil {
			t.Errorf("NewMoney(1, %q) error = nil, want error", currency)
		}
	}
}
