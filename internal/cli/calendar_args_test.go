package cli

import (
	"testing"
	"time"
)

func TestCalendarOptionsAccepts330CalendarDaysAcrossDST(t *testing.T) {
	location, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, location)
	options, err := parseCalendarOptions([]string{"BCN", "AGP", "--date", "2027-04-01", "--return", "2028-02-25"}, now)
	if err != nil {
		t.Fatalf("valid 330-day return window rejected: %v", err)
	}
	if options.returnDate != "25/02/2028" {
		t.Fatalf("return date = %q, want 25/02/2028", options.returnDate)
	}
}

func TestCalendarOptionsRejectsEmptyCurrency(t *testing.T) {
	_, err := parseCalendarOptions([]string{"BCN", "AGP", "--date", "+30", "--prices", "--currency="}, time.Now())
	if err == nil || err.Error() != "--currency requires a value" {
		t.Fatalf("empty calendar currency error = %v, want a missing-value error", err)
	}
}
