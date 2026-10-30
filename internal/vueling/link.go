package vueling

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type LinkOptions struct {
	Origin, Destination string
	Date, ReturnDate    time.Time
	Adults, Children    int
	Infants             int
	Discount            int
	Culture, Currency   string
}

// FlightURL builds the documented Vueling flight-selection deep link.
func FlightURL(o LinkOptions) (string, error) {
	o.Origin, o.Destination = strings.ToUpper(strings.TrimSpace(o.Origin)), strings.ToUpper(strings.TrimSpace(o.Destination))
	if len(o.Origin) != 3 || len(o.Destination) != 3 || o.Origin == o.Destination || o.Date.IsZero() {
		return "", fmt.Errorf("different IATA codes and a departure date are required")
	}
	if o.Adults < 1 || o.Children < 0 || o.Infants < 0 || o.Infants > o.Adults || o.Adults+o.Children+o.Infants > 9 {
		return "", fmt.Errorf("passengers must total 1–9, with at least one adult and no more infants than adults")
	}
	if !o.ReturnDate.IsZero() && o.ReturnDate.Before(o.Date) {
		return "", fmt.Errorf("return date must be on or after departure date")
	}
	if o.Discount < 0 || o.Discount > 5 {
		return "", fmt.Errorf("discount must be between 0 and 5")
	}
	if o.Culture == "" {
		o.Culture = "es-ES"
	}
	if o.Currency == "" {
		o.Currency = "EUR"
	}
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
	return "https://tickets.vueling.com/booking?" + q.Encode(), nil
}
