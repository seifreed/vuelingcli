package vueling

import (
	"math"
	"testing"
)

func TestParseFare(t *testing.T) {
	for _, tc := range []struct {
		text string
		want float64
		ok   bool
	}{
		{"49,99 €", 49.99, true},
		{"49.99 EUR", 49.99, true},
		{"35 €", 35, true},
		{"9,9 €", 9.9, true},
		{"1.234,50 €", 1234.5, true},
		{"1,234.50 USD", 1234.5, true},
		{"1.234 €", 1234, true},
		{"-49,99 €", 0, false},
		{"−49,99 EUR", 0, false},
		{"€", 0, false},
		{"", 0, false},
	} {
		got, ok := ParseFare(tc.text)
		if got != tc.want || ok != tc.ok {
			t.Errorf("ParseFare(%q) = %v, %v; want %v, %v", tc.text, got, ok, tc.want, tc.ok)
		}
	}
}

func TestRoundCents(t *testing.T) {
	if got, want := RoundCents(0.1+0.2), 0.3; math.Abs(got-want) > 1e-9 {
		t.Fatalf("RoundCents = %v, want %v", got, want)
	}
}

func TestFormatAmounts(t *testing.T) {
	for _, tc := range []struct {
		amount  float64
		fixed   string
		compact string
	}{
		{48, "48.00", "48"},
		{48.5, "48.50", "48.50"},
		{12.34, "12.34", "12.34"},
	} {
		if got := FormatAmount(tc.amount); got != tc.fixed {
			t.Errorf("FormatAmount(%v) = %q, want %q", tc.amount, got, tc.fixed)
		}
		if got := FormatCompactAmount(tc.amount); got != tc.compact {
			t.Errorf("FormatCompactAmount(%v) = %q, want %q", tc.amount, got, tc.compact)
		}
	}
}

func TestFormatFare(t *testing.T) {
	for _, tc := range []struct {
		amount   float64
		currency string
		want     string
	}{
		{48, "EUR", "48 €"},
		{48.5, "EUR", "48,50 €"},
		{12.5, "USD", "12,50 USD"},
		{12, "", "12 €"},
	} {
		if got := FormatFare(tc.amount, tc.currency); got != tc.want {
			t.Errorf("FormatFare(%v, %q) = %q, want %q", tc.amount, tc.currency, got, tc.want)
		}
	}
}
