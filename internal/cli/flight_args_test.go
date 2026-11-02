package cli

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/usecase"
	"github.com/seifreed/vuelingcli/internal/vueling"
)

func TestParseFlightInput(t *testing.T) {
	today := time.Now()
	departure := today.AddDate(0, 0, 30).Format("2006-01-02")
	returnDate := today.AddDate(0, 0, 35).Format("2006-01-02")
	for _, test := range []struct {
		name     string
		values   map[string]string
		adults   int
		children int
		infants  int
	}{
		{name: "defaults passengers", values: map[string]string{"date": departure}, adults: 1},
		{name: "parses return and passengers", values: map[string]string{"date": departure, "return": returnDate, "adults": "2", "children": "1", "infants": "1"}, adults: 2, children: 1, infants: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseFlightInput(test.values, []string{"Barcelona", "BIO"}, errors.New("usage"))
			if err != nil {
				t.Fatal(err)
			}
			if got.origin != "BCN" || got.destination != "BIO" || got.date.Format("2006-01-02") != departure || got.adults != test.adults || got.children != test.children || got.infants != test.infants {
				t.Fatalf("flight input = %+v", got)
			}
			if test.values["return"] != "" && got.returnDate.Format("2006-01-02") != returnDate {
				t.Fatalf("return date = %v, want %s", got.returnDate, test.values["return"])
			}
		})
	}
}

func TestParseFlightInputErrors(t *testing.T) {
	usage := errors.New("usage")
	date := time.Now().AddDate(0, 0, 30).Format(time.DateOnly)
	for _, test := range []struct {
		name       string
		positional []string
		key, value string
		want       string
	}{
		{name: "missing destination", positional: []string{"BCN"}, want: "usage"},
		{name: "missing date", positional: []string{"BCN", "BIO"}, key: "date", want: "usage"},
		{name: "unknown origin", positional: []string{"Atlantis", "BIO"}, want: "origin:"},
		{name: "unknown destination", positional: []string{"BCN", "Atlantis"}, want: "destination:"},
		{name: "invalid date", positional: []string{"BCN", "BIO"}, key: "date", value: "soon", want: "invalid date"},
		{name: "invalid return", positional: []string{"BCN", "BIO"}, key: "return", value: "soon", want: "invalid date"},
		{name: "invalid adults", positional: []string{"BCN", "BIO"}, key: "adults", value: "x", want: "invalid --adults"},
		{name: "invalid children", positional: []string{"BCN", "BIO"}, key: "children", value: "x", want: "invalid --children"},
		{name: "invalid infants", positional: []string{"BCN", "BIO"}, key: "infants", value: "x", want: "invalid --infants"},
	} {
		t.Run(test.name, func(t *testing.T) {
			values := map[string]string{"date": date}
			if test.key != "" {
				values[test.key] = test.value
			}
			got, err := parseFlightInput(values, test.positional, usage)
			if test.want == "usage" {
				if err != usage {
					t.Fatalf("error = %v, want the original usage error", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("flight input = %+v, error = %v, want %q", got, err, test.want)
			}
		})
	}
}

func TestParseFlightInputRejectsInvalidPassengerCounts(t *testing.T) {
	today := time.Now()
	date := today.AddDate(0, 0, 30).Format(time.DateOnly)
	for _, test := range []struct {
		name   string
		values map[string]string
	}{
		{name: "no adults", values: map[string]string{"adults": "0"}},
		{name: "negative adults", values: map[string]string{"adults": "-1"}},
		{name: "negative children", values: map[string]string{"children": "-1"}},
		{name: "negative infants", values: map[string]string{"infants": "-1"}},
		{name: "more infants than adults", values: map[string]string{"infants": "2"}},
		{name: "more than nine passengers", values: map[string]string{"adults": "8", "children": "2"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			values := map[string]string{"date": date}
			for key, value := range test.values {
				values[key] = value
			}
			if _, err := parseFlightInput(values, []string{"BCN", "BIO"}, errors.New("usage")); err == nil || !strings.Contains(err.Error(), "passengers must total") {
				t.Fatalf("parseFlightInput error = %v; want passenger count error", err)
			}
		})
	}
}

func TestParseFlightInputRejectsReturnBeforeDeparture(t *testing.T) {
	today := time.Now()
	values := map[string]string{
		"date":   today.AddDate(0, 0, 30).Format(time.DateOnly),
		"return": today.AddDate(0, 0, 29).Format(time.DateOnly),
	}
	if _, err := parseFlightInput(values, []string{"BCN", "BIO"}, errors.New("usage")); err == nil || !strings.Contains(err.Error(), "return date must be on or after departure date") {
		t.Fatalf("parseFlightInput error = %v; want return date order error", err)
	}
}

func TestFlightOptionsBuildDomainRequests(t *testing.T) {
	date := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	returnDate := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	options := flightOptions{
		flightInput: flightInput{origin: "BCN", destination: "BIO", date: date, returnDate: returnDate, adults: 2, children: 1, infants: 1},
		discount:    3,
		currency:    "EUR",
	}
	wantQuery := usecase.FlightQuery{
		Origin: "BCN", Destination: "BIO", Date: date, ReturnDate: returnDate,
		Adults: 2, Children: 1, Infants: 1, Discount: 3, Currency: "EUR",
	}
	if got := options.flightQuery(); got != wantQuery {
		t.Errorf("flightQuery() = %+v, want %+v", got, wantQuery)
	}
	wantLink := vueling.LinkOptions{
		Origin: "BCN", Destination: "BIO", Date: date, ReturnDate: returnDate,
		Adults: 2, Children: 1, Infants: 1, Discount: 3, Currency: "EUR",
	}
	if got := options.linkOptions(); got != wantLink {
		t.Errorf("linkOptions() = %+v, want %+v", got, wantLink)
	}
}

func TestParseFlightOptionsNormalizesCurrencyForTheAPIAndLink(t *testing.T) {
	options, err := parseFlightOptions([]string{"BCN", "BIO", "--date", "+30", "--currency", " eur "})
	if err != nil {
		t.Fatal(err)
	}
	if options.currency != "EUR" || options.linkOptions().Currency != "EUR" {
		t.Fatalf("currency = %q for API and %q for link, want EUR for both", options.currency, options.linkOptions().Currency)
	}
	if _, err := parseFlightOptions([]string{"BCN", "BIO", "--date", "+30", "--currency", "CAD"}); err == nil || !strings.Contains(err.Error(), "invalid --currency") {
		t.Fatalf("error = %v, want invalid currency", err)
	}
}
