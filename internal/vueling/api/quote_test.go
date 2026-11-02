package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

const quoteAvailability = `{"data":{"amsAvy":{"currencyCode":"EUR","trips":[{"trips":[{"journeysAvailableByMarket":[{"key":"BCN|BIO","value":[
{"segments":[{"identifier":{"carrierCode":"VY","identifier":"1383"}},{"identifier":{"carrierCode":"VY","identifier":"1050"}}],"fares":[{"fareAvailabilityKey":"c","fareSellKey":"sell-c","details":[{"bundleReferences":[]}]}]},
{"segments":[{"identifier":{"carrierCode":"VY","identifier":"1460"}}],"fares":[
 {"fareAvailabilityKey":"op","fareSellKey":"sell-op","details":[{"bundleReferences":[]}]},
 {"fareAvailabilityKey":"ba","fareSellKey":"sell-ba","details":[{"bundleReferences":["VllCQSEx","VkNCMiEw","VllPUCE0","VkNCMyEy"]},{"bundleReferences":["VllCQSEx"]}]}]}
]}]}]}],
"faresAvailable":[
 {"value":{"fareAvailabilityKey":"ba","fares":[{"productClass":"BA","passengerFares":[{"amsFareAmount":64.99}]}]}},
 {"value":{"fareAvailabilityKey":"op","fares":[{"productClass":"OP","passengerFares":[{"amsFareAmount":120.99}]}]}},
 {"value":{"fareAvailabilityKey":"c","fares":[{"productClass":"BA","passengerFares":[{"amsFareAmount":100}]},{"productClass":"BA","passengerFares":[{"amsFareAmount":50}]}]}}],
"bundleOffers":[
 {"key":"VkNCMiEw","value":{"bundleCode":"VCB2","bundlePrices":[{"totalPrice":45,"passengerType":"ADT"},{"totalPrice":40,"passengerType":"CHD"}]}},
 {"key":"VllCQSEx","value":{"bundleCode":"VYBA","bundlePrices":[{"totalPrice":0,"passengerType":"ADT"}]}},
 {"key":"VkNCMyEy","value":{"bundleCode":"VCB3","bundlePrices":[{"totalPrice":102,"passengerType":"ADT"}]}},
 {"key":"VllPUCE0","value":{"bundleCode":"VYOP","bundlePrices":[{"totalPrice":51,"passengerType":"ADT"}]}},
 {"key":"VlBSTyE1","value":{"bundleCode":"VPRO","bundlePrices":[{"totalPrice":68,"passengerType":"ADT"}]}}]}}}`

const quoteBreakdown = `{"itineraryType":"OneWay","currencyCode":"EUR","flights":[{"flightInfo":[{"departureStation":"BCN","departureDate":"2026-11-10T07:00:00","arrivalStation":"BIO","arrivalDate":"2026-11-10T08:20:00","flightIdentifier":"VY-1460"}],
"concepts":[{"code":"JourneyTotalPaxAdult","itemAmount":40.48,"itemCount":2,"totalAmount":80.96},{"code":"JourneyTotalPaxChild","itemAmount":40.48,"itemCount":1,"totalAmount":40.48},{"code":"JourneyTotalFees","itemAmount":73.53,"itemCount":1,"totalAmount":73.53,"detailedTaxes":{"JD":41.94,"ES":17.73,"OG":1.89,"QV":11.97}}],"totalAmount":194.97}],"discounts":[],"services":[],"totalAmount":194.97}`

func quoteServer(t *testing.T, availability, breakdown string, sold *[]string) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /asm/v1/Auth", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"accessToken":"jwt"}`))
	})
	mux.HandleFunc("POST /avy/v1/graphql", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(availability))
	})
	mux.HandleFunc("POST /bkd/v4/breakdown/Availability", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			FareSellKeyList []string `json:"fareSellKeyList"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || r.Header.Get("Authorization") != "Bearer jwt" {
			http.Error(w, "bad breakdown request", http.StatusBadRequest)
			return
		}
		*sold = append(*sold, body.FareSellKeyList...)
		_, _ = w.Write([]byte(breakdown))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return &Client{HTTP: server.Client(), AMS: server.URL}
}

var quoteQuery = FlightQuery{Origin: "BCN", Destination: "BIO", Date: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC), Adults: 2, Children: 1, Currency: "EUR"}

func TestQuoteBreaksDownTheCheapestFareAndPricesItsBundles(t *testing.T) {
	var sold []string
	client := quoteServer(t, quoteAvailability, quoteBreakdown, &sold)
	quote, err := client.Quote(context.Background(), quoteQuery, "vy1460")
	if err != nil {
		t.Fatal(err)
	}
	want := vueling.Quote{
		Flight: "VY1460", Date: "2026-11-10", Origin: "BCN", Destination: "BIO", Departure: "07:00", Arrival: "08:20", Currency: "EUR",
		Fare: 121.44, Taxes: 73.53, TaxDetail: map[string]float64{"JD": 41.94, "ES": 17.73, "OG": 1.89, "QV": 11.97}, Total: 194.97,
		Bundles: []vueling.Bundle{
			{Code: "VYBA", Name: "Fly Light", PricePerPerson: 0, Total: 194.97},
			{Code: "VCB2", Name: "Fly", PricePerPerson: 45, Total: 324.97},
			{Code: "VCB3", Name: "Fly Grande", PricePerPerson: 102, Total: 500.97},
		},
	}
	if !reflect.DeepEqual(quote, want) {
		t.Fatalf("quote = %+v\nwant %+v", quote, want)
	}
	if !reflect.DeepEqual(sold, []string{"sell-ba"}) {
		t.Fatalf("breakdown fare sell keys = %v, want the cheapest fare's", sold)
	}
}

func TestQuoteSeparatesTheInfantFeeFromTaxes(t *testing.T) {
	const infantBreakdown = `{"currencyCode":"EUR","flights":[{"flightInfo":[{"departureStation":"BCN","departureDate":"2026-11-10T07:00:00","arrivalStation":"BIO","arrivalDate":"2026-11-10T08:20:00"}],
"concepts":[{"code":"JourneyTotalPaxAdult","itemAmount":40.48,"itemCount":2,"totalAmount":80.96},{"code":"JourneyTotalFees","itemAmount":49.02,"itemCount":1,"totalAmount":49.02,"detailedTaxes":{"JD":27.96,"ES":11.82,"OG":1.26,"QV":7.98}},
{"code":"JourneyTotalPaxINFANT","itemAmount":25,"itemCount":2,"totalAmount":50},{"code":"JourneyTotalFeesINFANT","itemAmount":22.96,"itemCount":2,"totalAmount":45.92}],"totalAmount":225.9}],"discounts":[],"services":[],"totalAmount":225.9}`
	var sold []string
	client := quoteServer(t, quoteAvailability, infantBreakdown, &sold)
	query := quoteQuery
	query.Children, query.Infants = 0, 2
	quote, err := client.Quote(context.Background(), query, "VY1460")
	if err != nil {
		t.Fatal(err)
	}
	if quote.Fare != 80.96 || quote.Taxes != 49.02 || quote.Infants != 2 || quote.InfantFee != 95.92 || quote.Total != 225.9 {
		t.Fatalf("quote = %+v, want the infants' 95.92 apart from 49.02 taxes", quote)
	}
	if !reflect.DeepEqual(quote.TaxDetail, map[string]float64{"JD": 27.96, "ES": 11.82, "OG": 1.26, "QV": 7.98}) {
		t.Fatalf("tax detail = %v", quote.TaxDetail)
	}
	if fly := quote.Bundles[1]; fly.Code != "VCB2" || fly.Total != 315.9 {
		t.Fatalf("Fly bundle = %+v, want 225.9 plus 45 for each adult and nothing for infants", fly)
	}
}

func TestQuoteReportsBundleOfferWithoutAdultPrice(t *testing.T) {
	availability := strings.Replace(quoteAvailability,
		`"bundleCode":"VCB3","bundlePrices":[{"totalPrice":102,"passengerType":"ADT"}]`,
		`"bundleCode":"VCB3","bundlePrices":[{"totalPrice":102,"passengerType":"CHD"}]`, 1)
	if availability == quoteAvailability {
		t.Fatal("test fixture did not replace VCB3 adult price")
	}
	var sold []string
	client := quoteServer(t, availability, quoteBreakdown, &sold)
	if _, err := client.Quote(context.Background(), quoteQuery, "VY1460"); err == nil || !strings.Contains(err.Error(), "no adult price for VCB3 bundle") {
		t.Fatalf("error = %v, want missing VCB3 adult price", err)
	}
}

func TestQuoteRejectsNegativeBundlePrice(t *testing.T) {
	availability := strings.Replace(quoteAvailability,
		`"bundleCode":"VCB3","bundlePrices":[{"totalPrice":102,"passengerType":"ADT"}]`,
		`"bundleCode":"VCB3","bundlePrices":[{"totalPrice":-102,"passengerType":"ADT"}]`, 1)
	if availability == quoteAvailability {
		t.Fatal("test fixture did not replace VCB3 adult price")
	}
	var sold []string
	client := quoteServer(t, availability, quoteBreakdown, &sold)
	if _, err := client.Quote(context.Background(), quoteQuery, "VY1460"); err == nil || !strings.Contains(err.Error(), "negative") {
		t.Fatalf("error = %v, want negative bundle price error", err)
	}
}

func TestQuoteRejectsBreakdownPassengerCountMismatch(t *testing.T) {
	adultBreakdown := strings.Replace(quoteBreakdown,
		`{"code":"JourneyTotalPaxAdult","itemAmount":40.48,"itemCount":2,"totalAmount":80.96},`, "", 1)
	childBreakdown := strings.Replace(quoteBreakdown,
		`{"code":"JourneyTotalPaxChild","itemAmount":40.48,"itemCount":1,"totalAmount":40.48},`, "", 1)
	if adultBreakdown == quoteBreakdown || childBreakdown == quoteBreakdown {
		t.Fatal("test fixture did not remove the passenger concepts")
	}
	infantQuery := quoteQuery
	infantQuery.Infants = 1
	tests := []struct {
		name, breakdown, want string
		query                 FlightQuery
	}{
		{"adult", adultBreakdown, "adults=0", quoteQuery},
		{"child", childBreakdown, "children=0", quoteQuery},
		{"infant", quoteBreakdown, "requested infants=1", infantQuery},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var sold []string
			client := quoteServer(t, quoteAvailability, test.breakdown, &sold)
			if _, err := client.Quote(context.Background(), test.query, "VY1460"); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestQuoteMatchesConnectionsAndReportsMissingFlights(t *testing.T) {
	var sold []string
	client := quoteServer(t, quoteAvailability, quoteBreakdown, &sold)
	for _, flight := range []string{"VY1383", "VY1383+VY1050"} {
		if _, err := client.Quote(context.Background(), quoteQuery, flight); err != nil {
			t.Fatalf("%s: %v", flight, err)
		}
	}
	if !reflect.DeepEqual(sold, []string{"sell-c", "sell-c"}) {
		t.Fatalf("breakdown fare sell keys = %v", sold)
	}
	for _, flight := range []string{"VY1050", "VY9999"} {
		if _, err := client.Quote(context.Background(), quoteQuery, flight); err == nil || !strings.Contains(err.Error(), "no bookable flight "+flight) {
			t.Fatalf("%s: error = %v", flight, err)
		}
	}
}

func TestQuoteReportsBreakdownAndAvailabilityFailures(t *testing.T) {
	var sold []string
	tests := []struct {
		name, availability, breakdown, want string
	}{
		{"empty breakdown", quoteAvailability, `{"flights":[]}`, "no price breakdown"},
		{"bad breakdown", quoteAvailability, `nope`, "price breakdown"},
		{"no availability", `{"data":{}}`, quoteBreakdown, "no availability data"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := quoteServer(t, test.availability, test.breakdown, &sold)
			if _, err := client.Quote(context.Background(), quoteQuery, "VY1460"); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "down", http.StatusBadGateway) }))
	defer failing.Close()
	if _, err := (&Client{HTTP: failing.Client(), AMS: failing.URL}).Quote(context.Background(), quoteQuery, "VY1460"); err == nil || !strings.Contains(err.Error(), "anonymous Vueling session") {
		t.Fatalf("error = %v, want a session error", err)
	}
}

func TestQuoteRejectsNegativeBreakdownAmounts(t *testing.T) {
	tests := []struct {
		name, old, replacement string
	}{
		{"total", `"services":[],"totalAmount":194.97}`, `"services":[],"totalAmount":-1}`},
		{"fare", `"code":"JourneyTotalPaxAdult","itemAmount":40.48,"itemCount":2,"totalAmount":80.96`, `"code":"JourneyTotalPaxAdult","itemAmount":40.48,"itemCount":2,"totalAmount":-200`},
		{"taxes", `"totalAmount":73.53`, `"totalAmount":-73.53`},
		{"tax detail", `"JD":41.94`, `"JD":-41.94`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			breakdown := strings.Replace(quoteBreakdown, test.old, test.replacement, 1)
			if breakdown == quoteBreakdown {
				t.Fatal("test fixture did not replace the breakdown amount")
			}
			var sold []string
			client := quoteServer(t, quoteAvailability, breakdown, &sold)
			if _, err := client.Quote(context.Background(), quoteQuery, "VY1460"); err == nil || !strings.Contains(err.Error(), "negative") {
				t.Fatalf("error = %v, want a negative breakdown amount error", err)
			}
		})
	}
}

func TestQuoteRejectsNegativePassengerCounts(t *testing.T) {
	child := strings.Replace(quoteBreakdown,
		`"code":"JourneyTotalPaxChild","itemAmount":40.48,"itemCount":1,"totalAmount":40.48`,
		`"code":"JourneyTotalPaxChild","itemAmount":40.48,"itemCount":-1,"totalAmount":40.48`, 1)
	infant := strings.Replace(quoteBreakdown,
		`"code":"JourneyTotalPaxChild","itemAmount":40.48,"itemCount":1`,
		`"code":"JourneyTotalPaxChild","itemAmount":40.48,"itemCount":0`, 1)
	infant = strings.Replace(infant, `{"code":"JourneyTotalFees"`,
		`{"code":"JourneyTotalPaxINFANT","itemAmount":0,"itemCount":-1,"totalAmount":0},{"code":"JourneyTotalFees"`, 1)
	if child == quoteBreakdown || infant == quoteBreakdown {
		t.Fatal("test fixture did not replace or insert the passenger count")
	}
	for _, test := range []struct{ name, breakdown string }{{"child", child}, {"infant", infant}} {
		t.Run(test.name, func(t *testing.T) {
			query := quoteQuery
			query.Children = 0
			var sold []string
			client := quoteServer(t, quoteAvailability, test.breakdown, &sold)
			if _, err := client.Quote(context.Background(), query, "VY1460"); err == nil || !strings.Contains(err.Error(), "negative passenger count") {
				t.Fatalf("error = %v, want a negative passenger count error", err)
			}
		})
	}
}

func TestQuoteMarksAnOvernightArrival(t *testing.T) {
	var breakdown breakdownResponse
	overnight := `{"currencyCode":"EUR","flights":[{"flightInfo":[{"departureStation":"LPA","departureDate":"2026-11-20T20:30:00","arrivalStation":"BCN","arrivalDate":"2026-11-21T00:35:00"}],"concepts":[],"totalAmount":99}],"totalAmount":99}`
	if err := json.Unmarshal([]byte(overnight), &breakdown); err != nil {
		t.Fatal(err)
	}
	quote, err := breakdownQuote(breakdown, []string{"VY3009"}, time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if quote.ArrivalDate != "2026-11-21" || vueling.ArrivalClock(quote.Arrival, quote.Date, quote.ArrivalDate) != "00:35 +1" {
		t.Fatalf("quote = %+v", quote)
	}
}
