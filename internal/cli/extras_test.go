package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/seifreed/vuelingcli/internal/vueling"
	"github.com/seifreed/vuelingcli/internal/vueling/api"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

const extrasTestAvailability = `{"data":{"amsAvy":{"currencyCode":"EUR","trips":[{"trips":[{"journeysAvailableByMarket":[{"key":"BCN|BIO","value":[
{"journeyKey":"jk","designator":{"origin":"BCN","destination":"BIO"},"segments":[{"identifier":{"carrierCode":"VY","identifier":"1460"}}],"fares":[{"fareAvailabilityKey":"ba","fareSellKey":"sell-ba","details":[{"bundleReferences":["b1"]}]}]}]}]}]}],
"faresAvailable":[{"value":{"fareAvailabilityKey":"ba","fares":[{"productClass":"BA","passengerFares":[{"amsFareAmount":64.99}]}]}}],
"bundleOffers":[{"key":"b1","value":{"bundleCode":"VYBA","bundlePrices":[{"totalPrice":0,"passengerType":"ADT"}]}}]}}}`

const extrasTestSeatMap = `{"seatMaps":[{"passengerFees":[{"value":{"groups":[{"groupNumber":4,"amount":7},{"groupNumber":21,"amount":11}]}}],
"seatMap":{"decks":[{"value":{"compartments":[{"value":{"sections":[{"seatFam":{"name":"back"},"units":[{"type":"NormalSeat","group":4,"assignable":true,"isSeatAvailable":true},{"type":"NormalSeat","group":21,"assignable":true,"isSeatAvailable":true}]}]}}]}}]}}]}`

const extrasTestServices = `{"currencyCode":"EUR","paxSegmentsService":[{"serviceCode":"BG20","applyPax":"SpecifyPax","weight":20,"paxJourneySegments":[{"journeySegments":[{"journeyPrice":37.0}]}]},{"serviceCode":"FXPK","applyPax":"AllPax","price":14}]}`

// extrasAllowList is every call extras may make; anything else, such as passengers, contact details,
// seat or service sales, commit, payment or check-in, fails the test.
var extrasAllowList = map[string]bool{
	"POST /asm/v1/Auth":                        true,
	"POST /avy/v1/graphql":                     true,
	"POST /bkj/v2/Sell/selltrip/BSV":           true,
	"GET /bst/v2/SmartSeatMap":                 true,
	"GET /bpv/v3/BookingServices/Services/BPV": true,
	"DELETE /asm/v1/Auth":                      true,
}

func newExtrasTestCLI(t *testing.T) (*CLI, *bytes.Buffer, *[]string) {
	t.Helper()
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := r.Method + " " + r.URL.Path
		calls = append(calls, call)
		if !extrasAllowList[call] {
			t.Errorf("extras called %s, which is not on its allow-list", call)
			http.NotFound(w, r)
			return
		}
		switch call {
		case "POST /asm/v1/Auth":
			_ = json.NewEncoder(w).Encode(map[string]string{"accessToken": "jwt"})
		case "POST /avy/v1/graphql":
			_, _ = w.Write([]byte(extrasTestAvailability))
		case "POST /bkj/v2/Sell/selltrip/BSV":
			_, _ = w.Write([]byte(`{"success":true}`))
		case "GET /bst/v2/SmartSeatMap":
			_, _ = w.Write([]byte(extrasTestSeatMap))
		case "GET /bpv/v3/BookingServices/Services/BPV":
			_, _ = w.Write([]byte(extrasTestServices))
		}
	}))
	t.Cleanup(server.Close)
	stdout := new(bytes.Buffer)
	cli := New("test", stdout, new(bytes.Buffer), func(string) web.PageReader { return testPageReader{} })
	return cli.WithAPI(&api.Client{HTTP: server.Client(), AMS: server.URL}), stdout, &calls
}

func TestExtrasPrintsPricesAndOnlyCallsItsAllowList(t *testing.T) {
	cli, stdout, calls := newExtrasTestCLI(t)
	date := futureCLIDate(30)
	if code := cli.Run([]string{"extras", "BCN", "BIO", "--date", date, "--flight", "vy 1460"}); code != 0 {
		t.Fatalf("extras exited %d", code)
	}
	want := "VY1460 " + date + " BCN → BIO, with Fly Light (VYBA), prices in EUR\n" +
		"Seats:\n  Back 7.00–11.00, 2 free [back]\n" +
		"Bags:\n  Checked bag (20 kg) 37.00 [BG20]\n" +
		"Other extras:\n  Flex Pack (cancellation + flight change) 14.00, sold for all passengers [FXPK]\n" +
		extrasNotice + "\n"
	if stdout.String() != want {
		t.Fatalf("extras printed %q, want %q", stdout, want)
	}
	if last := (*calls)[len(*calls)-1]; last != "DELETE /asm/v1/Auth" {
		t.Fatalf("last call = %s, want the session ended", last)
	}
	stdout.Reset()
	if err := cli.extras([]string{"BCN", "BIO", "--date=" + date, "--cheapest", "--json"}); err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{`"booked":false`, `"sessionEnded":true`, `"seats":[{"code":"back","name":"Back","price":7,"maxPrice":11,"available":2,"currency":"EUR","perPassenger":true}]`, `"bags":[{"code":"BG20"`} {
		if !strings.Contains(stdout.String(), part) {
			t.Fatalf("extras --json = %s, missing %s", stdout, part)
		}
	}
}

func TestPrintExtrasFallsBackToTheBundleCode(t *testing.T) {
	stdout := new(bytes.Buffer)
	cli := &CLI{stdout: stdout}
	if err := cli.printExtras(vueling.Extras{Bundle: "NEW", Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "with NEW (NEW)") {
		t.Fatalf("extras printed %q, want the unknown bundle code", stdout)
	}
}

func TestExtrasValidatesItsOptions(t *testing.T) {
	cli, stdout, calls := newExtrasTestCLI(t)
	date := futureCLIDate(30)
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"BCN", "BIO", "--date", date}, "usage: vueling extras"},
		{[]string{"BCN", "BIO", "--date", date, "--flight", "VY1460", "--cheapest"}, "usage: vueling extras"},
		{[]string{"BCN", "--date", date, "--flight", "VY1460"}, "usage: vueling extras"},
		{[]string{"BCN", "BIO", "--flight", "VY1460"}, "usage: vueling extras"},
		{[]string{"BCN", "BIO", "--date", date, "--flight", "VY1460", "--bundle", "VYOP"}, "unknown --bundle VYOP"},
		{[]string{"BCN", "BIO", "--date", date, "--flight", "VY1460", "--return", futureCLIDate(32)}, "unknown option --return"},
		{[]string{"BCN", "BIO", "--date", date, "--flight"}, "--flight requires a value"},
		{[]string{"BCN", "BIO", "--date", date, "--flight", "VY1460", "--adults", "x"}, "invalid --adults"},
	}
	for _, test := range tests {
		if err := cli.extras(test.args); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("extras %v: error = %v, want %q", test.args, err, test.want)
		}
	}
	if len(*calls) != 0 {
		t.Fatalf("invalid options called Vueling: %v", *calls)
	}
	if err := cli.extras([]string{"--help"}); err != nil || !strings.Contains(stdout.String(), "never pays and never books") {
		t.Fatalf("extras --help = %v, %q", err, stdout)
	}
	noAPI := New("test", new(bytes.Buffer), new(bytes.Buffer), func(string) web.PageReader { return testPageReader{} })
	if err := noAPI.extras([]string{"BCN", "BIO", "--date", date, "--flight", "VY1460"}); err == nil || !strings.Contains(err.Error(), "HTTP API") {
		t.Fatalf("error = %v, want a missing-API error", err)
	}
}

func TestExtraLineMarksSegmentsAndFlatPrices(t *testing.T) {
	free := 3
	got := extraLine(vueling.ExtraPrice{Code: "front", Name: "Front", Segment: "VY1383 BCN-AMS", Price: 9, MaxPrice: 9, Available: &free, PerPassenger: true})
	if got != "VY1383 BCN-AMS Front 9.00, 3 free [front]" {
		t.Fatalf("extraLine = %q", got)
	}
	var b strings.Builder
	writeExtraSection(&b, "Bags", nil)
	if b.String() != "Bags: none offered\n" {
		t.Fatalf("empty section = %q", b.String())
	}
}
