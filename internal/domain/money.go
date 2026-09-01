// Package domain holds the ledger's business rules. It has no knowledge of
// databases, HTTP, or any transport. Everything here is unit-testable without
// external services.
package domain

import (
	"fmt"
	"math"
	"strings"
)

// Currency is an ISO 4217 alphabetic code.
type Currency string

const (
	BRL Currency = "BRL"
	USD Currency = "USD"
	EUR Currency = "EUR"
	JPY Currency = "JPY"
)

// exponents maps a currency to the number of decimal places in its minor unit.
// Most currencies use 2 (centavos, cents); JPY uses 0.
var exponents = map[Currency]int{
	BRL: 2,
	USD: 2,
	EUR: 2,
	JPY: 0,
}

// Money is an exact monetary amount stored as an integer count of minor units
// (centavos for BRL, cents for USD) together with its currency.
//
// Floating point is never used. 0.1 + 0.2 != 0.3 in binary floating point, and
// a ledger that cannot add cents exactly is not a ledger.
//
// The zero value is not valid; construct with NewMoney or Zero.
type Money struct {
	minor    int64
	currency Currency
}

// NewMoney builds a Money from a count of minor units.
func NewMoney(minor int64, c Currency) (Money, error) {
	if _, ok := exponents[c]; !ok {
		return Money{}, fmt.Errorf("%w: %q", ErrInvalidCurrency, c)
	}
	return Money{minor: minor, currency: c}, nil
}

// MustMoney is NewMoney that panics on an invalid currency. Use only in tests
// and in package-level constants, never on a request path.
func MustMoney(minor int64, c Currency) Money {
	m, err := NewMoney(minor, c)
	if err != nil {
		panic(err)
	}
	return m
}

// Zero returns a zero amount in the given currency.
func Zero(c Currency) (Money, error) { return NewMoney(0, c) }

func (m Money) Minor() int64       { return m.minor }
func (m Money) Currency() Currency { return m.currency }
func (m Money) IsZero() bool       { return m.minor == 0 }
func (m Money) IsNegative() bool   { return m.minor < 0 }
func (m Money) IsPositive() bool   { return m.minor > 0 }

// Add returns m + o. It fails on a currency mismatch or an int64 overflow.
func (m Money) Add(o Money) (Money, error) {
	if m.currency != o.currency {
		return Money{}, fmt.Errorf("%w: %s + %s", ErrCurrencyMismatch, m.currency, o.currency)
	}
	if (o.minor > 0 && m.minor > math.MaxInt64-o.minor) ||
		(o.minor < 0 && m.minor < math.MinInt64-o.minor) {
		return Money{}, fmt.Errorf("%w: %d + %d", ErrOverflow, m.minor, o.minor)
	}
	return Money{minor: m.minor + o.minor, currency: m.currency}, nil
}

// Sub returns m - o. It fails on a currency mismatch or an int64 overflow.
func (m Money) Sub(o Money) (Money, error) {
	if m.currency != o.currency {
		return Money{}, fmt.Errorf("%w: %s - %s", ErrCurrencyMismatch, m.currency, o.currency)
	}
	if (o.minor < 0 && m.minor > math.MaxInt64+o.minor) ||
		(o.minor > 0 && m.minor < math.MinInt64+o.minor) {
		return Money{}, fmt.Errorf("%w: %d - %d", ErrOverflow, m.minor, o.minor)
	}
	return Money{minor: m.minor - o.minor, currency: m.currency}, nil
}

// Neg returns -m. math.MinInt64 has no positive counterpart, so it overflows.
func (m Money) Neg() (Money, error) {
	if m.minor == math.MinInt64 {
		return Money{}, fmt.Errorf("%w: cannot negate %d", ErrOverflow, m.minor)
	}
	return Money{minor: -m.minor, currency: m.currency}, nil
}

// Compare returns -1 if m < o, 0 if equal, +1 if m > o. Currencies must match.
func (m Money) Compare(o Money) (int, error) {
	if m.currency != o.currency {
		return 0, fmt.Errorf("%w: %s vs %s", ErrCurrencyMismatch, m.currency, o.currency)
	}
	switch {
	case m.minor < o.minor:
		return -1, nil
	case m.minor > o.minor:
		return 1, nil
	default:
		return 0, nil
	}
}

// String renders the amount in major units, e.g. "R$ 1234.56" as "1234.56 BRL".
func (m Money) String() string {
	exp, ok := exponents[m.currency]
	if !ok {
		return fmt.Sprintf("%d %s", m.minor, m.currency)
	}
	if exp == 0 {
		return fmt.Sprintf("%d %s", m.minor, m.currency)
	}

	neg := m.minor < 0
	abs := m.minor
	if neg {
		// Negating math.MinInt64 overflows, so work in uint64 space.
		u := uint64(math.MaxInt64) + 1
		if m.minor == math.MinInt64 {
			return fmt.Sprintf("-%d.%0*d %s", u/pow10(exp), exp, u%pow10(exp), m.currency)
		}
		abs = -m.minor
	}

	div := int64(pow10(exp))
	sign := ""
	if neg {
		sign = "-"
	}
	return fmt.Sprintf("%s%d.%0*d %s", sign, abs/div, exp, abs%div, m.currency)
}

func pow10(n int) uint64 {
	p := uint64(1)
	for i := 0; i < n; i++ {
		p *= 10
	}
	return p
}

// ParseCurrency validates and normalises a currency code from user input.
func ParseCurrency(s string) (Currency, error) {
	c := Currency(strings.ToUpper(strings.TrimSpace(s)))
	if _, ok := exponents[c]; !ok {
		return "", fmt.Errorf("%w: %q", ErrInvalidCurrency, s)
	}
	return c, nil
}
