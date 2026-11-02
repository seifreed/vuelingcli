package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

func TestDealsSendsTheBestPricesQueryAndSortsByPrice(t *testing.T) {
	var query string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"bestPrices":{"value":[
{"originCode":"BCN","destinationCode":"AMS","isDirect":true,"departureDate":"2026-12-09T06:10:00","integerPartPrice":43,"decimalPartPrice":0},
{"originCode":"BCN","destinationCode":"AMS","isDirect":false,"departureDate":"2026-12-20T06:10:00","integerPartPrice":29,"decimalPartPrice":95},
{"originCode":"BCN","destinationCode":"BIO","isDirect":true,"departureDate":"2026-11-30T11:50:00","integerPartPrice":21,"decimalPartPrice":99}],"isSuccess":true}}`))
	}))
	defer server.Close()
	deals, err := (&Client{HTTP: server.Client(), APIW: server.URL}).Deals(context.Background(), "BCN", []string{"BIO", "AMS"}, time.Date(2026, 11, 1, 15, 0, 0, 0, time.UTC), 4, "EUR")
	if err != nil {
		t.Fatal(err)
	}
	if want := "currencyCode=EUR&destinationCodes=BIO%2CAMS&months=4&originCode=BCN&startDate=2026-11-01T00%3A00%3A00"; query != want {
		t.Fatalf("query = %s, want %s", query, want)
	}
	want := []vueling.Deal{
		{Destination: "BIO", Date: "2026-11-30", Price: 21.99, Currency: "EUR", Direct: true},
		{Destination: "AMS", Date: "2026-12-20", Price: 29.95, Currency: "EUR", Direct: false},
	}
	if !reflect.DeepEqual(deals, want) {
		t.Fatalf("deals = %+v, want %+v", deals, want)
	}
}

func TestDealsReportsAFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"bestPrices":{"value":[],"isSuccess":false,"error":{"description":"bad origin"}}}`))
	}))
	defer server.Close()
	_, err := (&Client{HTTP: server.Client(), APIW: server.URL}).Deals(context.Background(), "XXX", []string{"BIO"}, time.Now(), 1, "EUR")
	if err == nil || !strings.Contains(err.Error(), "bad origin") {
		t.Fatalf("error = %v, want the API description", err)
	}
}

func TestDealsRejectsMalformedRows(t *testing.T) {
	for _, tc := range []struct {
		name, row, want string
	}{
		{"destination", `{"destinationCode":"XX","departureDate":"2026-12-09T06:10:00","integerPartPrice":43,"decimalPartPrice":0}`, "invalid deal destination"},
		{"date", `{"destinationCode":"AMS","departureDate":"not-a-date","integerPartPrice":43,"decimalPartPrice":0}`, "invalid deal date"},
		{"price", `{"destinationCode":"AMS","departureDate":"2026-12-09T06:10:00","integerPartPrice":43,"decimalPartPrice":100}`, "invalid deal price"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"bestPrices":{"value":[` + tc.row + `],"isSuccess":true}}`))
			}))
			defer server.Close()
			_, err := (&Client{HTTP: server.Client(), APIW: server.URL}).Deals(context.Background(), "BCN", nil, time.Now(), 1, "EUR")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Deals error = %v, want %q", err, tc.want)
			}
		})
	}
}
