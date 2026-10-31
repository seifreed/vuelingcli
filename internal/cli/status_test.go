package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/seifreed/vuelingcli/internal/vueling"
	"github.com/seifreed/vuelingcli/internal/vueling/api"
)

func newStatusAPICLI(t *testing.T, fixture string) (*CLI, *bytes.Buffer, *[]string) {
	t.Helper()
	body := []byte(`{"flights":[]}`)
	if fixture != "" {
		var err error
		if body, err = os.ReadFile("../vueling/api/testdata/" + fixture); err != nil {
			t.Fatal(err)
		}
	}
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.String())
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	cli, stdout, _ := newTestCLI()
	return cli.WithAPI(&api.Client{HTTP: server.Client(), APIW: server.URL}), stdout, &requests
}

func TestFlightStatusReadsAPIByFlight(t *testing.T) {
	cli, stdout, requests := newStatusAPICLI(t, "status-flight-vy1008.json")
	if err := cli.flightStatus([]string{"vy1008", "--date", "2026-11-01"}); err != nil {
		t.Fatal(err)
	}
	if want := "VY1008 2026-11-01 En Hora 18:35–20:00 BCN→MAD\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	if len(*requests) != 1 || (*requests)[0] != "/api/v1/flights/VY1008?date=2026-11-01" {
		t.Fatalf("requests = %v", *requests)
	}
}

func TestFlightStatusReadsAPIByRouteForPastDateAsJSON(t *testing.T) {
	cli, stdout, requests := newStatusAPICLI(t, "status-route-bcn-mad-past.json")
	if err := cli.flightStatus([]string{"Barcelona", "MAD", "--date", "2020-10-29", "--json"}); err != nil {
		t.Fatal(err)
	}
	var got []vueling.FlightStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || len(got) != 2 || got[0].Arrival != "08:43" {
		t.Fatalf("stdout = %s (%v)", stdout, err)
	}
	if (*requests)[0] != "/api/v1/flights?date=2020-10-29&destinationCode=MAD&originCode=BCN" {
		t.Fatalf("requests = %v", *requests)
	}
}

func TestFlightStatusWatchStopsWhenAPIFlightsLanded(t *testing.T) {
	cli, stdout, requests := newStatusAPICLI(t, "status-route-bcn-mad-landed.json")
	if err := cli.flightStatus([]string{"BCN", "MAD", "--watch", "5"}); err != nil {
		t.Fatal(err)
	}
	if len(*requests) != 1 || !strings.Contains(stdout.String(), "VY1006") || !strings.Contains(stdout.String(), "Aterrizado 10:10–11:34") {
		t.Fatalf("requests = %v, stdout = %q", *requests, stdout)
	}
}

func TestFlightStatusReportsNoAPIFlights(t *testing.T) {
	cli, _, _ := newStatusAPICLI(t, "")
	if err := cli.flightStatus([]string{"VY9999"}); err == nil || !strings.Contains(err.Error(), "found no flight status") {
		t.Fatalf("flightStatus error = %v, want no-flights error", err)
	}
}

func TestFlightStatusBrowserFlagUsesChrome(t *testing.T) {
	cli, _, requests := newStatusAPICLI(t, "status-flight-vy1008.json")
	for _, args := range [][]string{{"VY1008", "--browser"}, {"VY1008", "--cdp-url", "http://127.0.0.1:9333"}} {
		if err := cli.flightStatus(args); err == nil || !strings.Contains(err.Error(), "unexpected page read") {
			t.Fatalf("flightStatus(%v) error = %v, want the Chrome reader", args, err)
		}
	}
	if len(*requests) != 0 {
		t.Fatalf("--browser called the API: %v", *requests)
	}
	if err := cli.flightStatus([]string{"VY1008", "--browser", "--date", "2020-01-01"}); err == nil || !strings.Contains(err.Error(), "today or later") {
		t.Fatalf("--browser past date error = %v", err)
	}
}

func TestFlightStatusHelpMentionsBrowser(t *testing.T) {
	cli, stdout, _ := newTestCLI()
	if err := cli.flightStatus([]string{"--help"}); err != nil || !strings.Contains(stdout.String(), "--browser") {
		t.Fatalf("help = %q, %v", stdout, err)
	}
}
