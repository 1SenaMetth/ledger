package domain

import (
	"errors"
	"math"
	"testing"
)

func TestMoneyAdd(t *testing.T) {
	tests := []struct {
		name    string
		a, b    Money
		want    int64
		wantErr error
	}{
		{"simple", MustMoney(100, BRL), MustMoney(250, BRL), 350, nil},
		{"add zero", MustMoney(100, BRL), MustMoney(0, BRL), 100, nil},
		{"add negative", MustMoney(100, BRL), MustMoney(-40, BRL), 60, nil},
		{"crosses zero", MustMoney(10, BRL), MustMoney(-25, BRL), -15, nil},
		{"currency mismatch", MustMoney(100, BRL), MustMoney(100, USD), 0, ErrCurrencyMismatch},
		{"overflow high", MustMoney(math.MaxInt64, BRL), MustMoney(1, BRL), 0, ErrOverflow},
		{"overflow low", MustMoney(math.MinInt64, BRL), MustMoney(-1, BRL), 0, ErrOverflow},
		{"max plus zero is fine", MustMoney(math.MaxInt64, BRL), MustMoney(0, BRL), math.MaxInt64, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.a.Add(tt.b)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Add() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && got.Minor() != tt.want {
				t.Errorf("Add() = %d, want %d", got.Minor(), tt.want)
			}
		})
	}
}

func TestMoneySub(t *testing.T) {
	tests := []struct {
		name    string
		a, b    Money
		want    int64
		wantErr error
	}{
		{"simple", MustMoney(250, BRL), MustMoney(100, BRL), 150, nil},
		{"goes negative", MustMoney(100, BRL), MustMoney(250, BRL), -150, nil},
		{"sub negative", MustMoney(100, BRL), MustMoney(-50, BRL), 150, nil},
		{"currency mismatch", MustMoney(100, BRL), MustMoney(100, EUR), 0, ErrCurrencyMismatch},
		{"overflow low", MustMoney(math.MinInt64, BRL), MustMoney(1, BRL), 0, ErrOverflow},
		{"overflow high", MustMoney(math.MaxInt64, BRL), MustMoney(-1, BRL), 0, ErrOverflow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.a.Sub(tt.b)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Sub() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && got.Minor() != tt.want {
				t.Errorf("Sub() = %d, want %d", got.Minor(), tt.want)
			}
		})
	}
}

func TestMoneyNeg(t *testing.T) {
	m := MustMoney(500, BRL)
	got, err := m.Neg()
	if err != nil {
		t.Fatalf("Neg() unexpected error: %v", err)
	}
	if got.Minor() != -500 {
		t.Errorf("Neg() = %d, want -500", got.Minor())
	}

	if _, err := MustMoney(math.MinInt64, BRL).Neg(); !errors.Is(err, ErrOverflow) {
		t.Errorf("Neg(MinInt64) error = %v, want ErrOverflow", err)
	}
}

// Adding then subtracting the same amount must return the original value.
// This is the property that matters most for a ledger.
func TestMoneyRoundTrip(t *testing.T) {
	amounts := []int64{0, 1, -1, 99, 100, 123456789, -987654321}
	for _, start := range amounts {
		for _, delta := range amounts {
			a := MustMoney(start, BRL)
			b := MustMoney(delta, BRL)

			added, err := a.Add(b)
			if err != nil {
				continue
			}
			back, err := added.Sub(b)
			if err != nil {
				t.Fatalf("Sub after Add failed: %v", err)
			}
			if back.Minor() != start {
				t.Errorf("round trip %d +%d -%d = %d, want %d", start, delta, delta, back.Minor(), start)
			}
		}
	}
}

func TestMoneyString(t *testing.T) {
	tests := []struct {
		money Money
		want  string
	}{
		{MustMoney(123456, BRL), "1234.56 BRL"},
		{MustMoney(5, BRL), "0.05 BRL"},
		{MustMoney(0, BRL), "0.00 BRL"},
		{MustMoney(-2599, USD), "-25.99 USD"},
		{MustMoney(1000, JPY), "1000 JPY"},
	}

	for _, tt := range tests {
		if got := tt.money.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}

func TestNewMoneyRejectsUnknownCurrency(t *testing.T) {
	if _, err := NewMoney(100, Currency("XXX")); !errors.Is(err, ErrInvalidCurrency) {
		t.Errorf("NewMoney() error = %v, want ErrInvalidCurrency", err)
	}
}

func TestParseCurrency(t *testing.T) {
	got, err := ParseCurrency("  brl ")
	if err != nil {
		t.Fatalf("ParseCurrency() unexpected error: %v", err)
	}
	if got != BRL {
		t.Errorf("ParseCurrency() = %q, want %q", got, BRL)
	}
	if _, err := ParseCurrency("bitcoin"); !errors.Is(err, ErrInvalidCurrency) {
		t.Errorf("ParseCurrency() error = %v, want ErrInvalidCurrency", err)
	}
}
