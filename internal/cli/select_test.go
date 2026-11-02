package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
	"github.com/seifreed/vuelingcli/internal/vueling/api"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

const selectAvailability = `{"data":{"amsAvy":{"currencyCode":"EUR","trips":[{"trips":[{"journeysAvailableByMarket":[{"key":"BCN|BIO","value":[
{"segments":[{"identifier":{"carrierCode":"VY","identifier":"1460"}}],"fares":[{"fareAvailabilityKey":"ba","fareSellKey":"sell-ba","details":[{"bundleReferences":["VllCQSEx","VkNCMiEw","VlBSTyE1"]}]}]}]}]}]}],
"faresAvailable":[{"value":{"fareAvailabilityKey":"ba","fares":[{"productClass":"BA","passengerFares":[{"amsFareAmount":64.99}]}]}}],
"bundleOffers":[{"key":"VkNCMiEw","value":{"bundleCode":"VCB2","bundlePrices":[{"totalPrice":45,"passengerType":"ADT"}]}},{"key":"VllCQSEx","value":{"bundleCode":"VYBA","bundlePrices":[{"totalPrice":0,"passengerType":"ADT"}]}},{"key":"VlBSTyE1","value":{"bundleCode":"VPRO","bundlePrices":[{"totalPrice":68,"passengerType":"ADT"}]}}]}}}`

const selectBreakdown = `{"currencyCode":"EUR","flights":[{"flightInfo":[{"departureStation":"BCN","departureDate":"2026-11-10T07:00:00","arrivalStation":"BIO","arrivalDate":"2026-11-10T08:20:00"}],
"concepts":[{"code":"JourneyTotalPaxAdult","itemCount":1,"totalAmount":40.48},{"code":"JourneyTotalFees","itemCount":1,"totalAmount":24.51,"detailedTaxes":{"JD":13.98,"OG":0.63,"QV":3.99,"ES":5.91}}],"totalAmount":64.99}],"totalAmount":64.99}`

const selectInfantBreakdown = `{"currencyCode":"EUR","flights":[{"flightInfo":[{"departureStation":"BCN","departureDate":"2026-11-10T07:00:00","arrivalStation":"BIO","arrivalDate":"2026-11-10T08:20:00"}],
"concepts":[{"code":"JourneyTotalPaxAdult","itemCount":1,"totalAmount":40.48},{"code":"JourneyTotalFees","itemCount":1,"totalAmount":24.51,"detailedTaxes":{"JD":13.98,"OG":0.63,"QV":3.99,"ES":5.91}},{"code":"JourneyTotalPaxINFANT","itemCount":1,"totalAmount":25},{"code":"JourneyTotalFeesINFANT","itemCount":1,"totalAmount":22.96}],"totalAmount":112.95}],"totalAmount":112.95}`

func futureCLIDate(days int) string {
	return time.Now().AddDate(0, 0, days).Format(time.DateOnly)
}

func newSelectTestCLI(t *testing.T) (*CLI, *bytes.Buffer) {
	t.Helper()
	return newSelectTestCLIWithBreakdown(t, selectBreakdown)
}

func newSelectTestCLIWithBreakdown(t *testing.T, breakdown string) (*CLI, *bytes.Buffer) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/Auth"):
			_ = json.NewEncoder(w).Encode(map[string]string{"accessToken": "jwt"})
		case strings.HasSuffix(r.URL.Path, "/breakdown/Availability"):
			_, _ = w.Write([]byte(breakdown))
		case strings.HasSuffix(r.URL.Path, "/graphql"):
			_, _ = w.Write([]byte(selectAvailability))
		default:
			t.Errorf("select called %s, which is not a read-only endpoint", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	stdout := new(bytes.Buffer)
	cli := New("test", stdout, new(bytes.Buffer), func(string) web.PageReader { return testPageReader{} })
	return cli.WithAPI(&api.Client{HTTP: server.Client(), AMS: server.URL}), stdout
}

func TestSelectPrintsTheBreakdownAndBundleTotals(t *testing.T) {
	cli, stdout := newSelectTestCLI(t)
	date := futureCLIDate(30)
	if err := cli.selectFlight([]string{"BCN", "BIO", "--date", date, "--flight", "vy1460"}); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("VY1460 %s 07:00 BCN → 08:20 BIO\n", date) +
		"fare 40.48 + taxes and fees 24.51 (ES 5.91, JD 13.98, OG 0.63, QV 3.99) = 64.99 EUR\n" +
		"Fly Light 64.99 · Fly 109.99 · Fly Pro 132.99\n" +
		fmt.Sprintf("Nothing was booked. Confirm on vueling.com: https://tickets.vueling.com/booking?adt=1&c=es-ES&cur=EUR&d=BIO&dd=%s&o=BCN&ofn=VY1460\n", date)
	if stdout.String() != want {
		t.Fatalf("select printed %q, want %q", stdout, want)
	}
	stdout.Reset()
	if err := cli.selectFlight([]string{"BCN", "BIO", "--date=" + date, "--flight=VY1460", "--json"}); err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{`"booked":false`, `"total":64.99`, `"name":"Fly Pro","pricePerPerson":68,"total":132.99`, `"url":"https://tickets.vueling.com/booking?`} {
		if !strings.Contains(stdout.String(), part) {
			t.Fatalf("select --json = %s, missing %s", stdout, part)
		}
	}
}

func TestSelectShowsTheInfantFeeAsItsOwnLine(t *testing.T) {
	cli, stdout := newSelectTestCLIWithBreakdown(t, selectInfantBreakdown)
	date := futureCLIDate(30)
	if err := cli.selectFlight([]string{"BCN", "BIO", "--date", date, "--flight", "VY1460", "--infants", "1"}); err != nil {
		t.Fatal(err)
	}
	want := "fare 40.48 + taxes and fees 24.51 (ES 5.91, JD 13.98, OG 0.63, QV 3.99) + infant fee 47.96 (1 infant) = 112.95 EUR\n" +
		"Fly Light 112.95 · Fly 157.95 · Fly Pro 180.95\n"
	if !strings.Contains(stdout.String(), want) || !strings.Contains(stdout.String(), "&inf=1&") {
		t.Fatalf("select printed %q, want %q", stdout, want)
	}
	stdout.Reset()
	if err := cli.selectFlight([]string{"BCN", "BIO", "--date", date, "--flight", "VY1460", "--infants", "1", "--json"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"taxes":24.51,`) || !strings.Contains(stdout.String(), `"infants":1,"infantFee":47.96,"total":112.95`) {
		t.Fatalf("select --json = %s, want the infant fee apart from taxes", stdout)
	}
}

func TestInfantSummaryCountsInfants(t *testing.T) {
	if got := infantSummary(vueling.Quote{}); got != "" {
		t.Fatalf("infantSummary without infants = %q", got)
	}
	if got := infantSummary(vueling.Quote{Infants: 2, InfantFee: 95.92}); got != " + infant fee 95.92 (2 infants)" {
		t.Fatalf("infantSummary = %q", got)
	}
}

func TestSelectValidatesItsOptions(t *testing.T) {
	cli, stdout := newSelectTestCLI(t)
	date, returnDate, earlierDate := futureCLIDate(30), futureCLIDate(32), futureCLIDate(29)
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"BCN", "BIO", "--date", date}, "usage: vueling select"},
		{[]string{"BCN", "--date", date, "--flight", "VY1460"}, "usage: vueling select"},
		{[]string{"BCN", "BIO", "--date", date, "--flight", "VY1460", "--return", returnDate}, "usage: vueling select"},
		{[]string{"BCN", "BIO", "--date", date, "--flight", "VY1460", "--cheapest"}, "usage: vueling select"},
		{[]string{"BCN", "BIO", "--date", date, "--flight", "VY1460", "--return", returnDate, "--return-flight", "VY1461", "--return-cheapest"}, "usage: vueling select"},
		{[]string{"BCN", "BIO", "--date", date, "--flight", "VY1460", "--return-flight", "VY1461"}, "need --return"},
		{[]string{"BCN", "BIO", "--date", date, "--flight", "VY1460", "--direct"}, "--direct needs --cheapest"},
		{[]string{"BCN", "BIO", "--date", date, "--cheapest", "--return", earlierDate, "--return-cheapest"}, "return date must be on or after departure date"},
		{[]string{"BCN", "BIO", "--flight", "VY1460"}, "usage: vueling select"},
		{[]string{"BCN", "BIO", "--date", date, "--flight", "VY1460", "--seat", "1A"}, "unknown option --seat"},
		{[]string{"BCN", "BIO", "--date", date, "--flight"}, "--flight requires a value"},
		{[]string{"BCN", "BIO", "--date", date, "--flight", "VY1460", "--adults", "x"}, "invalid --adults"},
		{[]string{"BCN", "BIO", "--date", date, "--flight", "VY9999"}, "no bookable flight VY9999"},
	}
	for _, test := range tests {
		if err := cli.selectFlight(test.args); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("select %v: error = %v, want %q", test.args, err, test.want)
		}
	}
	if err := cli.selectFlight([]string{"--help"}); err != nil || !strings.Contains(stdout.String(), "never selects, sells, or books") {
		t.Fatalf("select --help = %v, %q", err, stdout)
	}
	noAPI := New("test", new(bytes.Buffer), new(bytes.Buffer), func(string) web.PageReader { return testPageReader{} })
	if err := noAPI.selectFlight([]string{"BCN", "BIO", "--date", date, "--flight", "VY1460"}); err == nil || !strings.Contains(err.Error(), "HTTP API") {
		t.Fatalf("error = %v, want a missing-API error", err)
	}
}

func TestTaxSummaryIsEmptyWithoutTaxes(t *testing.T) {
	if got := taxSummary(nil); got != "" {
		t.Fatalf("taxSummary(nil) = %q", got)
	}
}
