package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/seifreed/vuelingcli/internal/vueling/api"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

const selectReturnAvailability = `{"data":{"amsAvy":{"currencyCode":"EUR","trips":[{"trips":[{"journeysAvailableByMarket":[{"key":"BIO|BCN","value":[
{"segments":[{"identifier":{"carrierCode":"VY","identifier":"1461"},"legs":[{}]}],"fares":[{"fareAvailabilityKey":"rd","fareSellKey":"sell-rd","details":[{"bundleReferences":["VllCQSEx","VkNCMiEw"]}]}]},
{"segments":[{"identifier":{"carrierCode":"VY","identifier":"2000"},"legs":[{}]},{"identifier":{"carrierCode":"VY","identifier":"2001"},"legs":[{}]}],"fares":[{"fareAvailabilityKey":"rc","fareSellKey":"sell-rc","details":[{"bundleReferences":["VllCQSEx"]}]}]}]}]}]}],
"faresAvailable":[{"value":{"fareAvailabilityKey":"rd","fares":[{"productClass":"BA","passengerFares":[{"amsFareAmount":50}]}]}},{"value":{"fareAvailabilityKey":"rc","fares":[{"productClass":"BA","passengerFares":[{"amsFareAmount":30}]}]}}],
"bundleOffers":[{"key":"VkNCMiEw","value":{"bundleCode":"VCB2","bundlePrices":[{"totalPrice":40,"passengerType":"ADT"}]}},{"key":"VllCQSEx","value":{"bundleCode":"VYBA","bundlePrices":[{"totalPrice":0,"passengerType":"ADT"}]}}]}}}`

const selectReturnBreakdown = `{"currencyCode":"EUR","flights":[{"flightInfo":[{"departureStation":"BIO","departureDate":"2026-11-15T21:00:00","arrivalStation":"BCN","arrivalDate":"2026-11-15T22:10:00"}],
"concepts":[{"code":"JourneyTotalPaxAdult","itemCount":1,"totalAmount":30.5},{"code":"JourneyTotalFees","itemCount":1,"totalAmount":19.5,"detailedTaxes":{"JD":19.5}}],"totalAmount":50}],"totalAmount":50}`

const selectConnectionBreakdown = `{"currencyCode":"EUR","flights":[{"flightInfo":[{"departureStation":"BIO","departureDate":"2026-11-15T06:00:00","arrivalStation":"MAD","arrivalDate":"2026-11-15T07:00:00"},{"departureStation":"MAD","departureDate":"2026-11-15T09:00:00","arrivalStation":"BCN","arrivalDate":"2026-11-15T10:15:00"}],
"concepts":[{"code":"JourneyTotalPaxAdult","itemCount":1,"totalAmount":10},{"code":"JourneyTotalFees","itemCount":1,"totalAmount":20}],"totalAmount":30}],"totalAmount":30}`

// newSelectTripTestCLI serves outbound and return availability by origin and breakdowns by fare sell key,
// recording the fare sell keys of every breakdown request and failing on any endpoint that is not read-only.
func newSelectTripTestCLI(t *testing.T) (*CLI, *bytes.Buffer, *[][]string) {
	t.Helper()
	breakdowns := map[string]string{"sell-ba": selectBreakdown, "sell-rd": selectReturnBreakdown, "sell-rc": selectConnectionBreakdown}
	var requested [][]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch {
		case strings.HasSuffix(r.URL.Path, "/Auth"):
			_ = json.NewEncoder(w).Encode(map[string]string{"accessToken": "jwt"})
		case strings.HasSuffix(r.URL.Path, "/breakdown/Availability"):
			var request struct {
				FareSellKeyList []string `json:"fareSellKeyList"`
			}
			_ = json.Unmarshal(body, &request)
			requested = append(requested, request.FareSellKeyList)
			_, _ = w.Write([]byte(breakdowns[request.FareSellKeyList[0]]))
		case strings.HasSuffix(r.URL.Path, "/graphql") && strings.Contains(string(body), `"origin":"BIO"`):
			_, _ = w.Write([]byte(selectReturnAvailability))
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
	return cli.WithAPI(&api.Client{HTTP: server.Client(), AMS: server.URL}), stdout, &requested
}

func TestSelectPricesBothDirectionsOfAReturnTrip(t *testing.T) {
	cli, stdout, requested := newSelectTripTestCLI(t)
	date, returnDate := futureCLIDate(30), futureCLIDate(35)
	if err := cli.selectFlight([]string{"BCN", "BIO", "--date", date, "--cheapest", "--return", returnDate, "--return-flight", "vy1461"}); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("VY1460 %s 07:00 BCN → 08:20 BIO\n", date) +
		"fare 40.48 + taxes and fees 24.51 (ES 5.91, JD 13.98, OG 0.63, QV 3.99) = 64.99 EUR\n" +
		"Fly Light 64.99 · Fly 109.99 · Fly Pro 132.99\n" +
		fmt.Sprintf("VY1461 %s 21:00 BIO → 22:10 BCN\n", returnDate) +
		"fare 30.50 + taxes and fees 19.50 (JD 19.50) = 50.00 EUR\n" +
		"Fly Light 50.00 · Fly 90.00\n" +
		"trip total 114.99 EUR\n" +
		fmt.Sprintf("Nothing was booked. Confirm on vueling.com: https://tickets.vueling.com/booking?adt=1&c=es-ES&cur=EUR&d=BIO&dd=%s&o=BCN&ofn=VY1460&rd=%s&rfn=VY1461\n", date, returnDate)
	if stdout.String() != want {
		t.Fatalf("select printed %q, want %q", stdout, want)
	}
	if !reflect.DeepEqual(*requested, [][]string{{"sell-ba"}, {"sell-rd"}}) {
		t.Fatalf("breakdown requests = %v, want one fare sell key per direction", *requested)
	}
}

func TestSelectCheapestReturnHonoursDirectAndJSONKeepsTheQuote(t *testing.T) {
	cli, stdout, _ := newSelectTripTestCLI(t)
	args := []string{"BCN", "BIO", "--date", futureCLIDate(30), "--cheapest", "--return", futureCLIDate(35), "--return-cheapest", "--json"}
	if err := cli.selectFlight(args); err != nil {
		t.Fatal(err)
	}
	var cheapest struct {
		Quote       struct{ Flight string } `json:"quote"`
		ReturnQuote struct{ Flight string } `json:"returnQuote"`
		Total       float64                 `json:"total"`
		Booked      bool                    `json:"booked"`
		URL         string                  `json:"url"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &cheapest); err != nil {
		t.Fatal(err)
	}
	if cheapest.Quote.Flight != "VY1460" || cheapest.ReturnQuote.Flight != "VY2000+VY2001" || cheapest.Total != 94.99 || cheapest.Booked || !strings.Contains(cheapest.URL, "rfn=VY2000") {
		t.Fatalf("select --return-cheapest --json = %s", stdout)
	}
	stdout.Reset()
	if err := cli.selectFlight(append(args, "--direct")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"flight":"VY1461"`) || !strings.Contains(stdout.String(), `"total":114.99`) {
		t.Fatalf("select --direct --json = %s", stdout)
	}
	stdout.Reset()
	if err := cli.selectFlight([]string{"BCN", "BIO", "--date", futureCLIDate(30), "--cheapest", "--json"}); err != nil {
		t.Fatal(err)
	}
	var oneWay map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &oneWay); err != nil {
		t.Fatal(err)
	}
	if _, ok := oneWay["returnQuote"]; ok || len(oneWay) != 3 {
		t.Fatalf("one-way select --json = %s, want only quote, url and booked", stdout)
	}
}
