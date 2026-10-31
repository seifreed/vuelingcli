package vueling

import (
	"slices"
	"testing"
)

func TestFilterFlights(t *testing.T) {
	flights := []Flight{
		{Direction: "outbound", Flight: "VY1000", Departure: "06:30", Fare: "89.99", Itinerary: "VY1000 Directo 06:30 08:45"},
		{Direction: "outbound", Flight: "VY1001", Departure: "09:15", Fare: "49.99", Stops: 1, Itinerary: "VY1001 1 escala 09:15 14:00"},
		{Direction: "outbound", Flight: "VY1002", Departure: "7:05", Fare: "", Itinerary: "VY1002 Directo 7:05 09:20"},
		{Direction: "outbound", Flight: "VY1003", Departure: "18:00", Fare: "49.99", Stops: 1, Itinerary: "VY1003 VY2003 18:00 23:10"},
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
		{"return window", FlightFilter{After: "09:15", ReturnBefore: "11:00"}, []string{"VY1001", "VY1003", "VY1004", "VY2000"}},
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

func TestFilterFlightsOnRenderedConnections(t *testing.T) {
	// Cards read from Vueling for SVQ-AMS on 2026-11-12; every option connects in BCN.
	card := func(flight, departure, fare, itinerary string) Flight {
		return Flight{Direction: "outbound", Flight: flight, Departure: departure, Fare: fare, Stops: 1, Itinerary: itinerary}
	}
	flights := []Flight{
		card("VY1383", "06:45", "282", "06:45 SVQ Sevilla Conexión en BCN: 1 h 40 min Ámsterdam AMS 12:40 Vueling VY1383 Vueling VY1050 SELECCIONAR VUELO POR 282 EUR 282 €"),
		card("VY1381", "09:00", "303", "09:00 SVQ Sevilla Conexión en BCN: 2 h 40 min Ámsterdam AMS 15:55 Vueling VY1381 Vueling VY1058 SELECCIONAR VUELO POR 303 EUR 303 €"),
		card("VY1393", "21:10", "274", "21:10 SVQ Sevilla Conexión en BCN: 11 h 15 min Ámsterdam AMS 12:40 +1 día Vueling VY1393 Vueling VY1050 SELECCIONAR VUELO POR 274 EUR 274 €"),
	}
	if got := FilterFlights(flights, FlightFilter{Direct: true}); len(got) != 0 {
		t.Fatalf("--direct kept connections: %v", got)
	}
	got := FilterFlights(flights, FlightFilter{Cheapest: true, Limit: 2})
	if len(got) != 2 || got[0].Fare != "274" || got[1].Fare != "282" {
		t.Fatalf("cheapest limit 2 = %v", got)
	}
}
