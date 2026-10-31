package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
	"github.com/seifreed/vuelingcli/internal/vueling/api"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

const twoJourneys = `{"data":{"amsAvy":{"currencyCode":"EUR","trips":[{"trips":[{"journeysAvailableByMarket":[{"key":"BCN|BIO","value":[
{"segments":[{"identifier":{"carrierCode":"VY","identifier":"1460"},"designator":{"origin":"BCN","destination":"BIO"},"legs":[{"legInfo":{}}]}],"designator":{"origin":"BCN","destination":"BIO","departure":"2026-11-03T07:00:00.000Z","arrival":"2026-11-03T08:20:00.000Z"},"fares":[{"fareAvailabilityKey":"a"}]},
{"segments":[{"identifier":{"carrierCode":"VY","identifier":"1464"},"designator":{"origin":"BCN","destination":"BIO"},"legs":[{"legInfo":{}}]}],"designator":{"origin":"BCN","destination":"BIO","departure":"2026-11-03T13:30:00.000Z","arrival":"2026-11-03T14:50:00.000Z"},"fares":[{"fareAvailabilityKey":"b"}]}
]}]}]}],"faresAvailable":[{"value":{"fareAvailabilityKey":"a","fares":[{"passengerFares":[{"amsFareAmount":197.99}]}]}},{"value":{"fareAvailabilityKey":"b","fares":[{"passengerFares":[{"amsFareAmount":47.99}]}]}}]}}}`

const noJourneys = `{"data":{"amsAvy":{"currencyCode":"EUR","trips":[],"faresAvailable":[]}}}`

func newAPITestCLI(t *testing.T, availability string) (*CLI, *bytes.Buffer) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/Auth") {
			_ = json.NewEncoder(w).Encode(map[string]string{"accessToken": "jwt"})
			return
		}
		_, _ = w.Write([]byte(availability))
	}))
	t.Cleanup(server.Close)
	stdout := new(bytes.Buffer)
	cli := New("test", stdout, new(bytes.Buffer), func(string) web.PageReader { return testPageReader{} })
	return cli.WithAPI(&api.Client{HTTP: server.Client(), AMS: server.URL}), stdout
}

func futureDate() string {
	return time.Now().AddDate(0, 0, 30).Format(time.DateOnly)
}

func TestFlightsReadsTheAvailabilityAPIWithoutChrome(t *testing.T) {
	cli, stdout := newAPITestCLI(t, twoJourneys)
	if err := cli.flights([]string{"BCN", "BIO", "--date", futureDate(), "--cheapest", "--limit", "1"}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[1], "VY1464 13:30–14:50 47.99 EUR | 13:30 BCN → 14:50 BIO VY1464 Directo") {
		t.Fatalf("flights printed %q", stdout)
	}
}

func TestFlightsReportsAnEmptyDay(t *testing.T) {
	cli, _ := newAPITestCLI(t, noJourneys)
	err := cli.flights([]string{"BCN", "BIO", "--date", futureDate()})
	if err == nil || !strings.Contains(err.Error(), "Vueling shows no flights for that route on the requested date") {
		t.Fatalf("error = %v, want a no-flights error", err)
	}
}

func TestFlightsBrowserFlagUsesChrome(t *testing.T) {
	cli, _ := newAPITestCLI(t, twoJourneys)
	err := cli.flights([]string{"BCN", "BIO", "--date", futureDate(), "--browser"})
	if err == nil || !strings.Contains(err.Error(), "unexpected page read") {
		t.Fatalf("error = %v, want the Chrome reader to be used", err)
	}
}

func TestRequireBothDirections(t *testing.T) {
	outbound := []vueling.Flight{{Direction: "outbound"}}
	if err := requireBothDirections(outbound, false); err != nil {
		t.Fatal(err)
	}
	if err := requireBothDirections(outbound, true); err == nil || !strings.Contains(err.Error(), "no return flights") {
		t.Fatalf("error = %v, want a no-return-flights error", err)
	}
	if err := requireBothDirections(append(outbound, vueling.Flight{Direction: "return"}), true); err != nil {
		t.Fatal(err)
	}
}
