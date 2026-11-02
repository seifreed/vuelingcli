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

func allFlightsServer(t *testing.T, respond func(w http.ResponseWriter, request allFlightsRequest)) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /asm/v1/Auth", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"accessToken":"jwt","expiration":1199}`))
	})
	mux.HandleFunc("POST /avy/v3/AvailabilityServices/allFlights", func(w http.ResponseWriter, r *http.Request) {
		var request allFlightsRequest
		if r.Header.Get("Authorization") != "Bearer jwt" || r.Header.Get("Origin") != ticketsOrigin || json.NewDecoder(r.Body).Decode(&request) != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		respond(w, request)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return &Client{HTTP: server.Client(), AMS: server.URL}
}

func TestScheduleSendsTheDatePickerRequestAndKeepsAvailableDays(t *testing.T) {
	fixture := readFixture(t, "allflights-bcn-esu.json")
	var sent allFlightsRequest
	client := allFlightsServer(t, func(w http.ResponseWriter, request allFlightsRequest) {
		sent = request
		_, _ = w.Write(fixture)
	})
	days, err := client.Schedule(context.Background(), "BCN", "ESU", "EUR", time.Date(2027, 3, 5, 0, 0, 0, 0, time.UTC), 2)
	if err != nil {
		t.Fatal(err)
	}
	want := allFlightsRequest{OriginCode: "BCN", DestinationCode: "ESU", Year: 2027, Month: 3, CurrencyCode: "EUR", MonthsRange: 2, FlightType: "OW"}
	if sent != want {
		t.Fatalf("request = %+v, want %+v", sent, want)
	}
	if len(days) != 8 {
		t.Fatalf("got %d days, want 8: %+v", len(days), days)
	}
	first := vueling.ScheduledDay{Date: "2027-03-03", Flight: "VY8502", Departure: "16:20", Fare: "27,78 €"}
	if !reflect.DeepEqual(days[0], first) || days[7].Date != "2027-03-27" {
		t.Fatalf("days = %+v", days)
	}
}

func TestScheduleTreatsAnEmptyAnswerAsNoFlights(t *testing.T) {
	client := allFlightsServer(t, func(http.ResponseWriter, allFlightsRequest) {})
	days, err := client.Schedule(context.Background(), "BCN", "FEZ", "EUR", time.Now(), 1)
	if err != nil || len(days) != 0 {
		t.Fatalf("days, err = %+v, %v", days, err)
	}
}

func TestScheduleReportsAPIErrors(t *testing.T) {
	client := allFlightsServer(t, func(w http.ResponseWriter, _ allFlightsRequest) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	if _, err := client.Schedule(context.Background(), "BCN", "BIO", "EUR", time.Now(), 1); err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("error = %v", err)
	}
}

func TestScheduledDaysKeepsCheapestDuplicateAndUnpricedDays(t *testing.T) {
	days, err := scheduledDays([]byte(`[
{"departureDate":"2027-01-02T09:00:00","flightID":"7","carrierCode":"VY","isConnectionFlight":true,"isInvalidPrice":true,"price":-1,"isAvailableDay":true},
{"departureDate":"2027-01-02T10:00:00","flightID":"8","carrierCode":"VY","price":9,"currency":"EUR","isAvailableDay":true},
{"departureDate":"2027-01-02T11:00:00","flightID":"9","carrierCode":"VY","price":5,"currency":"EUR","isAvailableDay":true},
{"departureDate":"2027-01-01T00:00:00","isAvailableDay":true},
{"departureDate":"bad","isAvailableDay":false}]`))
	want := []vueling.ScheduledDay{
		{Date: "2027-01-01", Departure: "00:00"},
		{Date: "2027-01-02", Flight: "VY9", Departure: "11:00", Fare: "5 €"},
	}
	if err != nil || !reflect.DeepEqual(days, want) {
		t.Fatalf("days, err = %+v, %v", days, err)
	}
	if _, err := scheduledDays([]byte(`{`)); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("error = %v", err)
	}
}

func TestScheduledDaysReportsMalformedAvailableDate(t *testing.T) {
	if _, err := scheduledDays([]byte(`[{"departureDate":"bad","isAvailableDay":true}]`)); err == nil || !strings.Contains(err.Error(), `departure date "bad"`) {
		t.Fatalf("scheduledDays error = %v, want malformed available date", err)
	}
}

func TestScheduledDaysRejectsNegativePrices(t *testing.T) {
	_, err := scheduledDays([]byte(`[{"departureDate":"2027-01-02T10:00:00","price":-1,"isAvailableDay":true}]`))
	if err == nil || !strings.Contains(err.Error(), "negative price") {
		t.Fatalf("scheduledDays error = %v, want a negative fare error", err)
	}
}
