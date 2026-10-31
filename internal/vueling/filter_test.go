package vueling

import (
	"slices"
	"testing"
)

func TestFilterFlights(t *testing.T) {
	flights := []Flight{
		{Direction: "outbound", Flight: "VY1000", Departure: "06:30", Fare: "89.99", Itinerary: "VY1000 Directo 06:30 08:45"},
		{Direction: "outbound", Flight: "VY1001", Departure: "09:15", Fare: "49.99", Itinerary: "VY1001 1 escala 09:15 14:00"},
		{Direction: "outbound", Flight: "VY1002", Departure: "7:05", Fare: "", Itinerary: "VY1002 Directo 7:05 09:20"},
		{Direction: "outbound", Flight: "VY1003", Departure: "18:00", Fare: "49.99", Itinerary: "VY1003 VY2003 18:00 23:10"},
		{Direction: "outbound", Flight: "VY1004", Departure: "21:00", Fare: "39", Itinerary: "VY1004 Directo 21:00 23:15"},
		{Direction: "return", Flight: "VY2000", Departure: "10:00", Fare: "120.50", Itinerary: "VY2000 Directo 10:00 12:00"},
		{Direction: "return", Flight: "VY2001", Departure: "12:00", Fare: "60", Itinerary: "VY2001 Directo 12:00 14:00"},
	}
	tests := []struct {
		name   string
		filter FlightFilter
		want   []string
	}{
		{"none", FlightFilter{}, []string{"VY1000", "VY1001", "VY1002", "VY1003", "VY1004", "VY2000", "VY2001"}},
		{"cheapest stable unparseable last", FlightFilter{Cheapest: true}, []string{"VY1004", "VY1001", "VY1003", "VY1000", "VY1002", "VY2001", "VY2000"}},
		{"limit per direction", FlightFilter{Limit: 1}, []string{"VY1000", "VY2000"}},
		{"cheapest limit", FlightFilter{Cheapest: true, Limit: 2}, []string{"VY1004", "VY1001", "VY2001", "VY2000"}},
		{"after inclusive", FlightFilter{After: "09:15"}, []string{"VY1001", "VY1003", "VY1004", "VY2000", "VY2001"}},
		{"before inclusive", FlightFilter{Before: "07:05"}, []string{"VY1000", "VY1002"}},
		{"window", FlightFilter{After: "07:00", Before: "18:00"}, []string{"VY1001", "VY1002", "VY1003", "VY2000", "VY2001"}},
		{"direct", FlightFilter{Direct: true}, []string{"VY1000", "VY1002", "VY1004", "VY2000", "VY2001"}},
		{"direct cheapest window", FlightFilter{Direct: true, Cheapest: true, After: "07:00"}, []string{"VY1004", "VY1002", "VY2001", "VY2000"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			for _, flight := range FilterFlights(flights, test.filter) {
				got = append(got, flight.Flight)
			}
			if !slices.Equal(got, test.want) {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestParseClock(t *testing.T) {
	if got, err := ParseClock("07:30"); err != nil || got != 450 {
		t.Fatalf("ParseClock(07:30) = %d, %v", got, err)
	}
	for _, value := range []string{"", "24:00", "7", "12:60", "noon"} {
		if _, err := ParseClock(value); err == nil {
			t.Fatalf("ParseClock(%q) accepted", value)
		}
	}
}
