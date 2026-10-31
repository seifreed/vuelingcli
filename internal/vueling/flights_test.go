package vueling

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseRenderedFlight(t *testing.T) {
	for _, tc := range []struct {
		name string
		card renderedFlight
		want Flight
	}{
		{
			name: "outbound euro fare",
			card: renderedFlight{Direction: "outbound", Label: "Seleccionar vuelo por 49,99 €", Details: "07:15\nBCN\n  VY1234 Vueling\n09:05\nAGP"},
			want: Flight{Direction: "outbound", Date: "2027-02-03", Flight: "VY1234", Departure: "07:15", Arrival: "09:05", Fare: "49.99", Currency: "EUR", Itinerary: "07:15 BCN VY1234 Vueling 09:05 AGP", Flights: []string{"VY1234"}},
		},
		{
			name: "return pound fare",
			card: renderedFlight{Direction: "return", Label: "Seleccionar vuelo por 35 £", Details: "18:40 LGW VY7001 Vueling 20:00 BCN"},
			want: Flight{Direction: "return", Date: "2027-02-08", Flight: "VY7001", Departure: "18:40", Arrival: "20:00", Fare: "35", Currency: "GBP", Itinerary: "18:40 LGW VY7001 Vueling 20:00 BCN", Flights: []string{"VY7001"}},
		},
		{
			name: "dollar fare",
			card: renderedFlight{Direction: "outbound", Label: "Seleccionar vuelo por 120.50 $", Details: "10:00 VY812 12:30"},
			want: Flight{Direction: "outbound", Date: "2027-02-03", Flight: "VY812", Departure: "10:00", Arrival: "12:30", Fare: "120.50", Currency: "USD", Itinerary: "10:00 VY812 12:30", Flights: []string{"VY812"}},
		},
		{
			name: "currency code is upper-cased",
			card: renderedFlight{Direction: "outbound", Label: "Seleccionar vuelo por 99 chf", Details: "06:00 VY6000 07:30"},
			want: Flight{Direction: "outbound", Date: "2027-02-03", Flight: "VY6000", Departure: "06:00", Arrival: "07:30", Fare: "99", Currency: "CHF", Itinerary: "06:00 VY6000 07:30", Flights: []string{"VY6000"}},
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
	wantDays := []CalendarDay{{Date: "03/01/2027", Fare: "29 €"}, {Date: "15/01/2027", Fare: "40 €"}}
	if !reflect.DeepEqual(got.Days, wantDays) {
		t.Errorf("Days = %v, want %v", got.Days, wantDays)
	}
	wantReturn := []CalendarDay{{Date: "20/02/2027", Fare: "45 €"}}
	if !reflect.DeepEqual(got.ReturnDays, wantReturn) {
		t.Errorf("ReturnDays = %v, want %v", got.ReturnDays, wantReturn)
	}
}

func TestDecodeCalendarErrors(t *testing.T) {
	for _, tc := range []struct {
		name, output, want string
	}{
		{"empty list", `[]`, "did not render"},
		{"empty object", `{}`, "did not render"},
		{"not JSON", `nope`, "did not render"},
		{"malformed days", `{"days":"x"}`, "did not render"},
		{"non-string field", `{"fare":12}`, "did not render"},
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

func TestValidateDevToolsEndpoint(t *testing.T) {
	for _, tc := range []struct {
		endpoint string
		ok       bool
	}{
		{"http://127.0.0.1:9222", true},
		{"http://localhost:9222", true},
		{"http://[::1]:9222", true},
		{"https://127.0.0.1:9222", false},
		{"http://192.168.1.10:9222", false},
		{"http://example.com:9222", false},
		{"http://user:pass@127.0.0.1:9222", false},
		{"http://", false},
		{"127.0.0.1:9222", false},
		{"", false},
	} {
		err := validateDevToolsEndpoint(tc.endpoint)
		if (err == nil) != tc.ok {
			t.Errorf("validateDevToolsEndpoint(%q) error = %v, want ok=%v", tc.endpoint, err, tc.ok)
		}
	}
}
