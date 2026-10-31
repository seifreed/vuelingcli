package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

func statusServer(t *testing.T, wantPath, wantQuery, fixture string) *Client {
	t.Helper()
	body := `{"flights":[]}`
	if fixture != "" {
		data, err := os.ReadFile("testdata/" + fixture)
		if err != nil {
			t.Fatal(err)
		}
		body = string(data)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wantPath || r.URL.RawQuery != wantQuery || r.Header.Get("Origin") != wwwOrigin {
			http.Error(w, "unexpected "+r.URL.String(), http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return &Client{HTTP: server.Client(), APIW: server.URL}
}

func TestFlightStatusByFlight(t *testing.T) {
	client := statusServer(t, "/api/v1/flights/VY1008", "date=2026-11-01", "status-flight-vy1008.json")
	got, err := client.FlightStatus(context.Background(), "VY1008", "", "", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	want := []vueling.FlightStatus{{
		Flight: "VY1008", Date: "2026-11-01", Status: "En Hora", Origin: "BCN", Destination: "MAD",
		Departure: "18:35", Arrival: "20:00",
		Details: "scheduled 18:35–20:00; BCN terminal 1, gate A04; MAD terminal 4, belt 1014",
	}}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("FlightStatus = %+v, want %+v", got, want)
	}
}

func TestFlightStatusByRouteShowsUpdatedTimes(t *testing.T) {
	client := statusServer(t, "/api/v1/flights", "date=2026-10-29&destinationCode=MAD&originCode=BCN", "status-route-bcn-mad-past.json")
	got, err := client.FlightStatus(context.Background(), "", "BCN", "MAD", time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Flight != "VY1008" || got[1].Departure != "17:43" || got[1].Arrival != "18:53" || got[1].Status != "Aterrizado" {
		t.Fatalf("FlightStatus = %+v", got)
	}
	if !strings.HasPrefix(got[1].Details, "scheduled 17:30–18:55;") || !vueling.AllFinal(got) {
		t.Fatalf("details or finality wrong: %+v", got)
	}
}

func TestFlightStatusReportsNoFlights(t *testing.T) {
	client := statusServer(t, "/api/v1/flights/VY9999", "date=2026-11-01", "")
	_, err := client.FlightStatus(context.Background(), "VY9999", "", "", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if !errors.Is(err, ErrNoFlightStatus) {
		t.Fatalf("FlightStatus error = %v, want ErrNoFlightStatus", err)
	}
}

func TestFlightStatusRejectsBadSearches(t *testing.T) {
	client := &Client{APIW: "http://unused.invalid"}
	for _, tc := range []struct{ flight, origin, destination, want string }{
		{"VY1008/../x", "", "", "flight must be VY"},
		{"", "BCN", "BCN", "two different"},
		{"", "bcn", "MAD", "two different"},
	} {
		if _, err := client.FlightStatus(context.Background(), tc.flight, tc.origin, tc.destination, time.Now()); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("FlightStatus(%q, %q, %q) error = %v, want %q", tc.flight, tc.origin, tc.destination, err, tc.want)
		}
	}
}

func TestStatusFlightMapping(t *testing.T) {
	flight := statusFlight{FlightCode: "VY1", Status: "DerivedLanded"}
	flight.Segment.Origin = statusStop{IATA: "BCN", Scheduled: "2026-11-01T18:35:00", ToShow: "0001-01-01T00:00:00"}
	flight.Segment.Destination = statusStop{IATA: "VLC", Scheduled: "bad", ToShow: "2026-11-01T20:05:00", Gate: " "}
	flight.Connection = &struct {
		DurationMinutes int    `json:"durationMinutes"`
		Location        string `json:"location"`
	}{45, "PMI"}
	got := flight.toStatus("2026-11-02")
	want := vueling.FlightStatus{
		Flight: "VY1", Date: "2026-11-01", Status: "Aterrizado en VLC", Origin: "BCN", Destination: "VLC",
		Departure: "18:35", Arrival: "20:05", Details: "scheduled 18:35–; connection in PMI (45 min)",
	}
	if got != want {
		t.Fatalf("toStatus = %+v, want %+v", got, want)
	}
	for status, want := range map[string]string{"Cancelled": "No operado", "Delayed": "Nueva hora", "Flying": "En vuelo", "Derived": "Desviado a VLC", "Boarding": "Boarding"} {
		if got := statusLabel(status, "VLC"); got != want {
			t.Errorf("statusLabel(%q) = %q, want %q", status, got, want)
		}
	}
}
