package vueling

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

var fareAmountPattern = regexp.MustCompile(`[-−]?[0-9]+(?:[.,][0-9]+)*`)

// FormatAmount renders a monetary amount with two decimal places.
func FormatAmount(amount float64) string {
	return strconv.FormatFloat(amount, 'f', 2, 64)
}

// FormatCompactAmount omits zero cents while keeping non-zero amounts to two decimal places.
func FormatCompactAmount(amount float64) string {
	return strings.TrimSuffix(FormatAmount(amount), ".00")
}

// FormatFare renders a price the way Vueling's calendar does, such as "48 €" or "48,99 €".
func FormatFare(amount float64, currency string) string {
	text := FormatCompactAmount(amount)
	text = strings.Replace(text, ".", ",", 1)
	if currency == "EUR" || currency == "" {
		currency = "€"
	}
	return text + " " + currency
}

// ParseFare reads the amount of a displayed fare such as "49,99 €" or "1.234,50 EUR".
func ParseFare(text string) (float64, bool) {
	amount := fareAmountPattern.FindString(text)
	if amount == "" {
		return 0, false
	}
	if strings.HasPrefix(amount, "-") || strings.HasPrefix(amount, "−") {
		return 0, false
	}
	digits := strings.NewReplacer(".", "", ",", "")
	if i := strings.LastIndexAny(amount, ".,"); i >= 0 && len(amount)-i-1 <= 2 {
		amount = digits.Replace(amount[:i]) + "." + amount[i+1:]
	} else {
		amount = digits.Replace(amount)
	}
	value, err := strconv.ParseFloat(amount, 64)
	return value, err == nil
}

// RoundCents rounds a monetary amount to two decimal places.
func RoundCents(amount float64) float64 {
	return math.Round(amount*100) / 100
}
