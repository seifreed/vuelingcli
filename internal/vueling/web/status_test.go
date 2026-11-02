package web

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

func TestParseStatusCard(t *testing.T) {
	date := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	got, err := parseStatusCard([]string{"En Hora", "VY1158", "13:00", "BCNBarcelona", "MaltaMLA", "15:10", "VER DETALLES"}, date)
	if err != nil {
		t.Fatal(err)
	}
	want := vueling.FlightStatus{Flight: "VY1158", Date: "2026-10-31", Status: "En Hora", Origin: "BCN", Destination: "MLA", Departure: "13:00", Arrival: "15:10", Details: "En Hora VY1158 13:00 BCNBarcelona MaltaMLA 15:10 VER DETALLES"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	got, err = parseStatusCard([]string{"En Hora", "VY392D", "14:40", "BCNBarcelona", "Palma (Mallorca)PMI", "15:35", "VER DETALLES"}, date)
	if err != nil {
		t.Fatal(err)
	}
	if got.Flight != "VY392D" || got.Origin != "BCN" || got.Destination != "PMI" || got.Arrival != "15:35" {
		t.Fatalf("lettered flight card parsed as %+v", got)
	}
}

func TestFlightStatusFromPageRejectsIncompleteCard(t *testing.T) {
	reader := &testPageReader{output: []byte(`[[]]`)}
	date := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	if statuses, err := (Source{}).FlightStatusFromPage(context.Background(), reader, "VY1234", "", "", date); err == nil {
		t.Fatalf("FlightStatusFromPage returned incomplete statuses %v with no error", statuses)
	}
}

func TestFlightStatusFromPageBuildsFlightAndRouteQueries(t *testing.T) {
	date := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	output := []byte(`[["En Hora","VY1158","13:00","BCNBarcelona","MaltaMLA","15:10"]]`)
	for _, tc := range []struct {
		name, flight, origin, destination, queryKey, queryValue string
	}{
		{"flight", "VY1158", "", "", "vuelingcliFlight", "VY1158"},
		{"route", "", "BCN", "PMI", "vuelingcliFrom", "BCN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &testPageReader{output: output}
			got, err := (Source{}).FlightStatusFromPage(context.Background(), reader, tc.flight, tc.origin, tc.destination, date)
			if err != nil || len(got) != 1 || got[0].Flight != "VY1158" {
				t.Fatalf("FlightStatusFromPage = %+v, %v", got, err)
			}
			if !strings.Contains(reader.pageURL, tc.queryKey+"="+tc.queryValue) || !strings.Contains(reader.pageURL, "vuelingcliDay=31-9") {
				t.Fatalf("status URL = %q", reader.pageURL)
			}
		})
	}
}

func TestFlightStatusFromPageRejectsInvalidSearchAndPageResults(t *testing.T) {
	date := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, flight, origin, destination string
		reader                            *testPageReader
		want                              string
	}{
		{"invalid flight", "VY", "", "", nil, "flight must be VY"},
		{"invalid route", "", "BCN", "BCN", nil, "route needs two different"},
		{"page read error", "VY1158", "", "", &testPageReader{err: errors.New("reader failed")}, "reader failed"},
		{"page error", "VY1158", "", "", &testPageReader{output: []byte(`{"error":"status unavailable"}`)}, "Vueling status unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := tc.reader
			if reader == nil {
				reader = &testPageReader{}
			}
			if _, err := (Source{}).FlightStatusFromPage(context.Background(), reader, tc.flight, tc.origin, tc.destination, date); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("FlightStatusFromPage error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestFlightStatusFromPageRejectsNoCards(t *testing.T) {
	reader := &testPageReader{output: []byte(`[]`)}
	date := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)

	statuses, err := (Source{}).FlightStatusFromPage(context.Background(), reader, "VY1234", "", "", date)
	if err == nil || err.Error() != "Vueling did not render any flight status cards" {
		t.Fatalf("FlightStatusFromPage returned %d statuses and error %v", len(statuses), err)
	}
}

func TestFlightStatusFromPagePreservesDecodeError(t *testing.T) {
	reader := &testPageReader{output: []byte("{")}
	date := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	_, err := (Source{}).FlightStatusFromPage(context.Background(), reader, "VY1234", "", "", date)
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("FlightStatusFromPage error = %v, want wrapped JSON syntax error", err)
	}
}

func TestFlightStatusScriptSkipsHiddenPaddingDays(t *testing.T) {
	// The picker repeats next month's first days as hidden, disabled padding cells with the same data-date.
	if !strings.Contains(flightStatusScript, `data-date="' + day + '"]')].filter(b => b.offsetParent)`) {
		t.Fatal("flight-status script must pick the visible day button, not the hidden padding cell")
	}
}
