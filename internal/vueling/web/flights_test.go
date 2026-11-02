package web

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

type testPageReader struct {
	pageURL string
	script  string
	output  []byte
	err     error
}

func (reader *testPageReader) ReadPage(_ context.Context, pageURL, script string) ([]byte, error) {
	reader.pageURL, reader.script = pageURL, script
	return reader.output, reader.err
}

func TestFlightsFromPageUsesInjectedReader(t *testing.T) {
	reader := &testPageReader{output: []byte(`[{"direction":"outbound","label":"Seleccionar vuelo por 49 €","details":"07:15 BCN VY1234 Vueling 09:05 AGP"}]`)}
	flights, err := (Source{}).FlightsFromPage(context.Background(), reader, "https://example.test/search", "2027-02-03", "")
	if err != nil {
		t.Fatal(err)
	}
	if reader.pageURL != "https://example.test/search" || !strings.Contains(reader.script, "seleccionar vuelo por") {
		t.Fatalf("reader request = (%q, %q), want search URL and rendered flight script", reader.pageURL, reader.script)
	}
	want := vueling.Flight{Direction: "outbound", Date: "2027-02-03", Flight: "VY1234", Departure: "07:15", Arrival: "09:05", Fare: "49", Currency: "EUR", Itinerary: "07:15 BCN VY1234 Vueling 09:05 AGP", Flights: []string{"VY1234"}}
	if !reflect.DeepEqual(flights, []vueling.Flight{want}) {
		t.Fatalf("FlightsFromPage = %+v, want %+v", flights, []vueling.Flight{want})
	}
}

func TestFlightsFromPagePreservesDecodeError(t *testing.T) {
	reader := &testPageReader{output: []byte("{")}
	_, err := (Source{}).FlightsFromPage(context.Background(), reader, "https://example.test/search", "2027-02-03", "")
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("FlightsFromPage error = %v, want wrapped JSON syntax error", err)
	}
}

func TestFlightsFromPageReportsNoFlightsDay(t *testing.T) {
	reader := &testPageReader{output: []byte(`{"error":"shows no flights for that route on the requested date"}`)}
	_, err := (Source{}).FlightsFromPage(context.Background(), reader, "https://example.test/search", "2027-02-03", "")
	if err == nil || err.Error() != "Vueling shows no flights for that route on the requested date" {
		t.Fatalf("FlightsFromPage error = %v, want the page's no-flights message", err)
	}
}

func TestFlightsFromPageRejectsInvalidInputAndEmptyResults(t *testing.T) {
	for _, tc := range []struct {
		name   string
		url    string
		reader *testPageReader
		want   string
	}{
		{"invalid URL", "://bad", &testPageReader{}, "invalid Vueling search URL"},
		{"page read failure", "https://example.test/search", &testPageReader{err: errors.New("reader failed")}, "reader failed"},
		{"no cards", "https://example.test/search", &testPageReader{output: []byte(`[]`)}, "did not render flight offers"},
		{"unreadable card", "https://example.test/search", &testPageReader{output: []byte(`[{"direction":"outbound","label":"flight","details":""}]`)}, "could not parse a rendered Vueling flight card"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := (Source{}).FlightsFromPage(context.Background(), tc.reader, tc.url, "2027-02-03", ""); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("FlightsFromPage error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestParseRenderedFlightLegs(t *testing.T) {
	tests := []struct {
		name    string
		details string
		stops   int
		flights []string
		via     []string
	}{
		{"rendered SVQ-AMS connection", "06:45 SVQ Sevilla Conexión en BCN: 1 h 40 min Ámsterdam AMS 12:40 Vueling VY1383 Vueling VY1050 SELECCIONAR VUELO POR 282 EUR 282 €", 1, []string{"VY1383", "VY1050"}, []string{"BCN"}},
		{"rendered overnight connection", "21:10 SVQ Sevilla Conexión en BCN: 11 h 15 min Ámsterdam AMS 12:40 +1 día Vueling VY1393 Vueling VY1050 SELECCIONAR VUELO POR 274 EUR 274 €", 1, []string{"VY1393", "VY1050"}, []string{"BCN"}},
		{"direct", "VY1000 Directo 06:30 08:45", 0, []string{"VY1000"}, nil},
		{"stop text without second number", "VY1001 1 escala 09:15 14:00", 1, []string{"VY1001"}, nil},
		{"repeated number", "VY1003 VY2003 VY1003 18:00 23:10", 1, []string{"VY1003", "VY2003"}, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			card := renderedFlight{Direction: "outbound", Label: "SELECCIONAR VUELO POR 99 EUR", Details: test.details}
			flight, err := parseRenderedFlight(card, "12/11/2026", "")
			if err != nil {
				t.Fatal(err)
			}
			if flight.Stops != test.stops || !slices.Equal(flight.Flights, test.flights) || !slices.Equal(flight.Via, test.via) {
				t.Fatalf("legs = %d %v %v, want %d %v %v", flight.Stops, flight.Flights, flight.Via, test.stops, test.flights, test.via)
			}
			direct, err := vueling.FilterFlights([]vueling.Flight{flight}, vueling.FlightFilter{Direct: true})
			if err != nil {
				t.Fatal(err)
			}
			if (len(direct) == 1) != (test.stops == 0) {
				t.Fatalf("--direct kept %d of %d stops", len(direct), test.stops)
			}
		})
	}
}

func TestParseRenderedFlight(t *testing.T) {
	for _, tc := range []struct {
		name string
		card renderedFlight
		want vueling.Flight
	}{
		{
			name: "outbound euro fare",
			card: renderedFlight{Direction: "outbound", Label: "Seleccionar vuelo por 49,99 €", Details: "07:15\nBCN\n  VY1234 Vueling\n09:05\nAGP"},
			want: vueling.Flight{Direction: "outbound", Date: "2027-02-03", Flight: "VY1234", Departure: "07:15", Arrival: "09:05", Fare: "49.99", Currency: "EUR", Itinerary: "07:15 BCN VY1234 Vueling 09:05 AGP", Flights: []string{"VY1234"}},
		},
		{
			name: "return pound fare",
			card: renderedFlight{Direction: "return", Label: "Seleccionar vuelo por 35 £", Details: "18:40 LGW VY7001 Vueling 20:00 BCN"},
			want: vueling.Flight{Direction: "return", Date: "2027-02-08", Flight: "VY7001", Departure: "18:40", Arrival: "20:00", Fare: "35", Currency: "GBP", Itinerary: "18:40 LGW VY7001 Vueling 20:00 BCN", Flights: []string{"VY7001"}},
		},
		{
			name: "dollar fare",
			card: renderedFlight{Direction: "outbound", Label: "Seleccionar vuelo por 120.50 $", Details: "10:00 VY812 12:30"},
			want: vueling.Flight{Direction: "outbound", Date: "2027-02-03", Flight: "VY812", Departure: "10:00", Arrival: "12:30", Fare: "120.50", Currency: "USD", Itinerary: "10:00 VY812 12:30", Flights: []string{"VY812"}},
		},
		{
			name: "european grouped fare",
			card: renderedFlight{Direction: "outbound", Label: "Seleccionar vuelo por 1.234,50 €", Details: "10:00 VY1234 12:30"},
			want: vueling.Flight{Direction: "outbound", Date: "2027-02-03", Flight: "VY1234", Departure: "10:00", Arrival: "12:30", Fare: "1234.50", Currency: "EUR", Itinerary: "10:00 VY1234 12:30", Flights: []string{"VY1234"}},
		},
		{
			name: "us grouped fare",
			card: renderedFlight{Direction: "outbound", Label: "Seleccionar vuelo por 1,234.50 $", Details: "10:00 VY1234 12:30"},
			want: vueling.Flight{Direction: "outbound", Date: "2027-02-03", Flight: "VY1234", Departure: "10:00", Arrival: "12:30", Fare: "1234.50", Currency: "USD", Itinerary: "10:00 VY1234 12:30", Flights: []string{"VY1234"}},
		},
		{
			name: "currency code is upper-cased",
			card: renderedFlight{Direction: "outbound", Label: "Seleccionar vuelo por 99 chf", Details: "06:00 VY6000 07:30"},
			want: vueling.Flight{Direction: "outbound", Date: "2027-02-03", Flight: "VY6000", Departure: "06:00", Arrival: "07:30", Fare: "99", Currency: "CHF", Itinerary: "06:00 VY6000 07:30", Flights: []string{"VY6000"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRenderedFlight(tc.card, "2027-02-03", "2027-02-08")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseRenderedFlight = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParseRenderedFlightRejectsIncompleteCards(t *testing.T) {
	for name, card := range map[string]renderedFlight{
		"no flight number": {Direction: "outbound", Label: "Seleccionar vuelo por 49 €", Details: "07:15 09:05"},
		"one time":         {Direction: "outbound", Label: "Seleccionar vuelo por 49 €", Details: "07:15 VY1234"},
		"no fare":          {Direction: "outbound", Label: "Seleccionar vuelo", Details: "07:15 VY1234 09:05"},
		"no direction":     {Label: "Seleccionar vuelo por 49 €", Details: "07:15 VY1234 09:05"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseRenderedFlight(card, "2027-02-03", ""); err == nil {
				t.Fatal("incomplete card accepted")
			}
		})
	}
}

func TestParseRenderedFlightRejectsNegativeFare(t *testing.T) {
	for _, fare := range []string{"-49,99 €", "−49,99 €"} {
		card := renderedFlight{Direction: "outbound", Label: "Seleccionar vuelo por " + fare, Details: "07:15 VY1234 09:05"}
		if _, err := parseRenderedFlight(card, "2027-02-03", ""); err == nil {
			t.Fatalf("negative fare %q accepted", fare)
		}
	}
}

func TestParseRenderedFlightRejectsOutOfRangeFare(t *testing.T) {
	card := renderedFlight{Direction: "outbound", Label: "Seleccionar vuelo por " + strings.Repeat("9", 400) + " €", Details: "07:15 VY1234 09:05"}
	if _, err := parseRenderedFlight(card, "2027-02-03", ""); err == nil || !strings.Contains(err.Error(), "could not parse rendered Vueling fare") {
		t.Fatalf("parseRenderedFlight error = %v, want out-of-range fare error", err)
	}
}

func TestParseRenderedFlightUsesFinalConnectionArrival(t *testing.T) {
	card := renderedFlight{
		Direction: "outbound",
		Label:     "Seleccionar vuelo por 282 EUR",
		Details:   "06:45 SVQ VY1383 08:25 BCN 10:05 BCN VY1050 12:40 AMS",
	}
	flight, err := parseRenderedFlight(card, "2026-11-12", "")
	if err != nil {
		t.Fatal(err)
	}
	if flight.Departure != "06:45" || flight.Arrival != "12:40" {
		t.Fatalf("connection times = %s–%s, want 06:45–12:40", flight.Departure, flight.Arrival)
	}
}

func TestDecodeCalendar(t *testing.T) {
	output := []byte(`{"route":"IDA: Barcelona - Málaga","date":"14/01/2027","fare":"35 €","returnDate":"20/02/2027","days":[{"day":"15","fare":"40 €"},{"day":"3","fare":"29 €"},{"day":"3","fare":"31 €"}],"returnDays":[{"day":"20","fare":"45 €"}]}`)
	got, err := decodeCalendar(output)
	if err != nil {
		t.Fatal(err)
	}
	wantPrice := map[string]string{"route": "IDA: Barcelona - Málaga", "date": "14/01/2027", "fare": "35 €", "returnDate": "20/02/2027"}
	if !reflect.DeepEqual(got.Price, wantPrice) {
		t.Errorf("Price = %v, want %v", got.Price, wantPrice)
	}
	wantDays := []vueling.CalendarDay{{Date: "03/01/2027", Fare: "29 €"}, {Date: "15/01/2027", Fare: "40 €"}}
	if !reflect.DeepEqual(got.Days, wantDays) {
		t.Errorf("Days = %v, want %v", got.Days, wantDays)
	}
	wantReturn := []vueling.CalendarDay{{Date: "20/02/2027", Fare: "45 €"}}
	if !reflect.DeepEqual(got.ReturnDays, wantReturn) {
		t.Errorf("ReturnDays = %v, want %v", got.ReturnDays, wantReturn)
	}
}

func TestDecodeCalendarPreservesFieldError(t *testing.T) {
	_, err := decodeCalendar([]byte(`{"fare":42}`))
	var typeErr *json.UnmarshalTypeError
	if !errors.As(err, &typeErr) || !strings.Contains(err.Error(), `field "fare"`) {
		t.Fatalf("decodeCalendar error = %v, want wrapped fare field type error", err)
	}
}

func TestDecodeCalendarErrors(t *testing.T) {
	for _, tc := range []struct {
		name, output, want string
	}{
		{"empty list", `[]`, "could not decode rendered Vueling calendar:"},
		{"empty object", `{}`, "did not render"},
		{"not JSON", `nope`, "could not decode rendered Vueling calendar:"},
		{"malformed days", `{"days":"x"}`, `field "days"`},
		{"non-string field", `{"fare":12}`, `field "fare"`},
		{"page error", `{"error":"shows no fare for the requested date"}`, "Vueling shows no fare for the requested date"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeCalendar([]byte(tc.output))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("decodeCalendar(%s) error = %v, want it to contain %q", tc.output, err, tc.want)
			}
		})
	}
}
