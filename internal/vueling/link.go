package vueling

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// LinkOptions describes a Vueling search or flight-selection link.
type LinkOptions struct {
	Origin, Destination                     string
	Date, ReturnDate                        time.Time
	Adults, Children                        int
	Infants, Discount                       int
	Culture, Currency                       string
	OutboundFlight, OutboundFareBasis       string
	ReturnFlight, ReturnFareBasis, METToken string
}

// FlightURL builds a Vueling flight-selection deep link.
func FlightURL(o LinkOptions) (string, error) {
	return flightURL(o, "/booking", false)
}

// FlightSearchURL builds the route used by Vueling's rendered flight results.
func FlightSearchURL(o LinkOptions) (string, error) {
	return flightURL(o, "/booking/flightSearch", true)
}

func flightURL(o LinkOptions, path string, search bool) (string, error) {
	o.Origin, o.Destination = strings.ToUpper(strings.TrimSpace(o.Origin)), strings.ToUpper(strings.TrimSpace(o.Destination))
	o.OutboundFlight, o.ReturnFlight = strings.ToUpper(strings.TrimSpace(o.OutboundFlight)), strings.ToUpper(strings.TrimSpace(o.ReturnFlight))
	o.OutboundFareBasis, o.ReturnFareBasis = strings.ToUpper(strings.TrimSpace(o.OutboundFareBasis)), strings.ToUpper(strings.TrimSpace(o.ReturnFareBasis))
	if !IsIATACode(o.Origin) || !IsIATACode(o.Destination) || o.Origin == o.Destination || o.Date.IsZero() {
		return "", fmt.Errorf("different IATA codes and a departure date are required")
	}
	if err := ValidatePassengerCounts(o.Adults, o.Children, o.Infants); err != nil {
		return "", err
	}
	if !o.ReturnDate.IsZero() && o.ReturnDate.Before(o.Date) {
		return "", fmt.Errorf("return date must be on or after departure date")
	}
	if o.Discount < 0 || o.Discount > 5 {
		return "", fmt.Errorf("discount must be between 0 and 5")
	}
	for name, flight := range map[string]string{"outbound": o.OutboundFlight, "return": o.ReturnFlight} {
		if flight != "" && !validFlightNumber(flight) {
			return "", fmt.Errorf("%s flight must use VY followed by 3 or 4 digits", name)
		}
	}
	for name, fare := range map[string]string{"outbound": o.OutboundFareBasis, "return": o.ReturnFareBasis} {
		if fare != "" && !isAlphaNumeric(fare) {
			return "", fmt.Errorf("%s fare basis must be alphanumeric", name)
		}
	}
	if o.OutboundFareBasis != "" && o.OutboundFlight == "" {
		return "", fmt.Errorf("outbound fare basis requires an outbound flight")
	}
	if o.ReturnFlight != "" && o.ReturnDate.IsZero() {
		return "", fmt.Errorf("return flight requires a return date")
	}
	if o.ReturnFareBasis != "" && o.ReturnFlight == "" {
		return "", fmt.Errorf("return fare basis requires a return flight")
	}
	if o.METToken != "" && !isAlphaNumeric(o.METToken) {
		return "", fmt.Errorf("MET token must be alphanumeric")
	}
	if o.Culture == "" {
		o.Culture = "es-ES"
	}
	currency, err := NormalizeCurrency(o.Currency)
	if err != nil {
		return "", err
	}
	o.Currency = currency
	q := url.Values{"o": {o.Origin}, "d": {o.Destination}, "dd": {o.Date.Format(time.DateOnly)}, "adt": {fmt.Sprint(o.Adults)}, "c": {o.Culture}, "cur": {o.Currency}}
	if !o.ReturnDate.IsZero() {
		q.Set("rd", o.ReturnDate.Format(time.DateOnly))
	}
	if o.Children > 0 {
		q.Set("chd", fmt.Sprint(o.Children))
	}
	if o.Infants > 0 {
		q.Set("inf", fmt.Sprint(o.Infants))
	}
	if o.Discount > 0 {
		q.Set("dt", fmt.Sprint(o.Discount))
	}
	for key, value := range map[string]string{"ofn": o.OutboundFlight, "ofb": o.OutboundFareBasis, "rfn": o.ReturnFlight, "rfb": o.ReturnFareBasis, "MET_Token": o.METToken} {
		if value != "" {
			q.Set(key, value)
		}
	}
	if search {
		q.Set("chd", fmt.Sprint(o.Children))
		q.Set("inf", fmt.Sprint(o.Infants))
		q.Set("dt", fmt.Sprint(o.Discount))
		q.Set("b", "1")
	}
	return (&url.URL{Scheme: "https", Host: "tickets.vueling.com", Path: path, RawQuery: q.Encode()}).String(), nil
}

// ValidatePassengerCounts checks the passenger limits shared by searches and booking links.
func ValidatePassengerCounts(adults, children, infants int) error {
	if adults < 1 || adults > 9 || children < 0 || children > 9-adults || infants < 0 || infants > adults || infants > 9-adults-children {
		return fmt.Errorf("passengers must total 1–9, with at least one adult and no more infants than adults")
	}
	return nil
}

// IsIATACode reports whether value is a three-letter uppercase airport code.
func IsIATACode(s string) bool {
	if len(s) != 3 {
		return false
	}
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func validFlightNumber(s string) bool {
	if !strings.HasPrefix(s, "VY") || len(s) < 5 || len(s) > 6 {
		return false
	}
	for _, r := range s[2:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isAlphaNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// NormalizeCurrency upper-cases a currency code Vueling sells in, defaulting to EUR.
func NormalizeCurrency(currency string) (string, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	switch currency {
	case "":
		return "EUR", nil
	case "EUR", "SEK", "NOK", "DKK", "CHF", "GBP", "USD":
		return currency, nil
	default:
		return "", fmt.Errorf("unsupported currency %q", currency)
	}
}
