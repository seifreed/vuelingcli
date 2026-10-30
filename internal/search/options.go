package search

import (
	"fmt"
	"regexp"
	"time"
)

// Options contains the user supplied criteria shared by the CLI and search client.
type Options struct {
	Origin, Destination string
	Date, ReturnDate    time.Time
	Adults, Children    int
	Infants             int
	After, Before       string
	Direct, Cheapest    bool
	Available           bool
	Limit               int
	JSON, TOON          bool
}

var clock = regexp.MustCompile(`^(?:[01]\d|2[0-3]):[0-5]\d$`)

// Validate rejects impossible itineraries before they reach Vueling.
func (o Options) Validate(today time.Time) error {
	if o.Origin == "" || o.Destination == "" {
		return fmt.Errorf("origin and destination are required")
	}
	if o.Origin == o.Destination {
		return fmt.Errorf("origin and destination must differ")
	}
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	if o.Date.IsZero() || o.Date.Before(today) {
		return fmt.Errorf("date must be today or later")
	}
	if !o.ReturnDate.IsZero() && o.ReturnDate.Before(o.Date) {
		return fmt.Errorf("return date must be on or after departure date")
	}
	if o.Adults < 1 || o.Children < 0 || o.Infants < 0 || o.Infants > o.Adults || o.Adults+o.Children+o.Infants > 9 {
		return fmt.Errorf("passengers must total 1–9, with at least one adult and no more infants than adults")
	}
	for name, value := range map[string]string{"after": o.After, "before": o.Before} {
		if value != "" && !clock.MatchString(value) {
			return fmt.Errorf("--%s must use 24-hour HH:MM format", name)
		}
	}
	if o.Limit < 0 {
		return fmt.Errorf("limit cannot be negative")
	}
	if o.JSON && o.TOON {
		return fmt.Errorf("--json and --toon cannot be used together")
	}
	return nil
}
