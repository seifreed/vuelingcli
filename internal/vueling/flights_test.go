package vueling

import "testing"

func TestArrivalClockMarksLaterLandingDays(t *testing.T) {
	tests := []struct {
		name                       string
		departureDate, arrivalDate string
		want                       string
	}{
		{"same day", "2026-11-10", "", "00:45"},
		{"next day", "2026-11-10", "2026-11-11", "00:45 +1"},
		{"across a month", "2026-11-30", "2026-12-02", "00:45 +2"},
		{"unreadable departure", "", "2026-11-11", "00:45"},
		{"earlier day", "2026-11-10", "2026-11-09", "00:45"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ArrivalClock("00:45", test.departureDate, test.arrivalDate); got != test.want {
				t.Fatalf("ArrivalClock = %q, want %q", got, test.want)
			}
		})
	}
	flight := Flight{Date: "2026-11-10", Arrival: "00:45", ArrivalDate: "2026-11-11"}
	if got := flight.ArrivalClock(); got != "00:45 +1" {
		t.Fatalf("Flight.ArrivalClock = %q", got)
	}
}
