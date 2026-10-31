package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

func TestCalendarReadsAvailabilityFixture(t *testing.T) {
	fixture, err := os.ReadFile("testdata/availability-bcn-bio.json")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if r.URL.Path != "/api/v2/availability" || query.Get("originCode") != "BCN" || query.Get("destinationCode") != "BIO" || query.Get("currencyCode") != "EUR" || query.Get("providerName") != "DmpsRetrieveAvailabilityProvider" || r.Header.Get("Origin") != wwwOrigin {
			http.Error(w, "unexpected request "+r.URL.String(), http.StatusBadRequest)
			return
		}
		_, _ = w.Write(fixture)
	}))
	defer server.Close()
	days, err := (&Client{HTTP: server.Client(), APIW: server.URL}).Calendar(context.Background(), "BCN", "BIO", "EUR")
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 316 || days[0].Date != "01/11/2026" || days[len(days)-1].Date != "12/09/2027" {
		t.Fatalf("Calendar returned %d days from %v to %v", len(days), days[0], days[len(days)-1])
	}
	if day, ok := vueling.FareOn(days, "03/11/2026"); !ok || day.Fare != "48 €" {
		t.Fatalf("03/11/2026 fare = %v, %v; want 48 €", day, ok)
	}
}

func TestCalendarReportsAPIErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()
	_, err := (&Client{HTTP: server.Client(), APIW: server.URL}).Calendar(context.Background(), "BCN", "BIO", "EUR")
	var status *StatusError
	if !errors.As(err, &status) || status.Status != http.StatusInternalServerError {
		t.Fatalf("Calendar error = %v, want HTTP 500", err)
	}
}

func TestCalendarDaysSkipsInvalidRowsAndSortsByDate(t *testing.T) {
	days := calendarDays([]availabilityDay{
		{ArrivalDate: "2027-01-10T00:00:00", Price: 30.5, Currency: "EUR"},
		{ArrivalDate: "2026-12-31T00:00:00", Price: 20, Currency: "GBP"},
		{ArrivalDate: "2027-01-10T00:00:00", Price: 10, Currency: "EUR"},
		{ArrivalDate: "2027-01-11T00:00:00", Price: 15, Currency: "EUR", IsInvalidPrice: true},
		{ArrivalDate: "2027-01-12T00:00:00", Price: 0, Currency: "EUR"},
		{ArrivalDate: "not a date", Price: 9, Currency: "EUR"},
	})
	want := []vueling.CalendarDay{{Date: "31/12/2026", Fare: "20 GBP"}, {Date: "10/01/2027", Fare: "30,50 €"}}
	if len(days) != len(want) || days[0] != want[0] || days[1] != want[1] {
		t.Fatalf("calendarDays = %v, want %v", days, want)
	}
}
