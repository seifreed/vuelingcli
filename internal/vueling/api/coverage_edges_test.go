package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type readFailure struct{ err error }

func (body readFailure) Read([]byte) (int, error) { return 0, body.err }
func (body readFailure) Close() error             { return nil }

func TestDoRejectsInvalidRequestsBodiesAndTransportFailures(t *testing.T) {
	client := &Client{HTTP: http.DefaultClient}
	if err := client.do(context.Background(), http.MethodPost, "https://example.test", wwwOrigin, "", make(chan int), new(any)); err == nil {
		t.Fatal("do accepted a body that JSON cannot encode")
	}
	if err := client.do(context.Background(), http.MethodGet, "://bad", wwwOrigin, "", nil, new(any)); err == nil {
		t.Fatal("do accepted an invalid request URL")
	}
	transportErr := errors.New("dial failed")
	client.HTTP = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, transportErr })}
	if err := client.do(context.Background(), http.MethodGet, "https://example.test", wwwOrigin, "", nil, new(any)); !errors.Is(err, transportErr) {
		t.Fatalf("do transport error = %v", err)
	}
	readErr := errors.New("read failed")
	client.HTTP = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: readFailure{readErr}}, nil
	})}
	if err := client.do(context.Background(), http.MethodGet, "https://example.test", wwwOrigin, "", nil, new(any)); !errors.Is(err, readErr) {
		t.Fatalf("do body read error = %v", err)
	}
}

func TestAlertAcceptsAnInactiveEmptyBannerAndReportsReadErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"pae":false}`)) }))
	defer server.Close()
	alert, err := (&Client{HTTP: server.Client(), WWW: server.URL}).Alert(context.Background(), "es-ES")
	if err != nil || alert.Active || alert.Text != "" || alert.URL != "" {
		t.Fatalf("inactive empty alert = %+v, %v", alert, err)
	}
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "broken", http.StatusBadGateway) })
	if _, err := (&Client{HTTP: server.Client(), WWW: server.URL}).Alert(context.Background(), "es-ES"); err == nil || !strings.Contains(err.Error(), "read Vueling's alert banner") {
		t.Fatalf("Alert read error = %v", err)
	}
}

func TestCalendarMapsNotFoundToNoRouteFares(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, err := (&Client{HTTP: server.Client(), APIW: server.URL}).Calendar(context.Background(), "BCN", "XXX", "EUR")
	if err == nil || !strings.Contains(err.Error(), "sells no fares from BCN to XXX") {
		t.Fatalf("Calendar 404 error = %v", err)
	}
}

func TestDealsHandlesEmptyFailureDescriptionsAndHTTPFailures(t *testing.T) {
	for _, response := range []struct {
		body string
		code int
	}{
		{body: `{"bestPrices":{"isSuccess":false}}`, code: http.StatusOK},
		{body: `failure`, code: http.StatusBadGateway},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(response.code)
			_, _ = io.WriteString(w, response.body)
		}))
		_, err := (&Client{HTTP: server.Client(), APIW: server.URL}).Deals(context.Background(), "BCN", nil, time.Now(), 1, "EUR")
		server.Close()
		if err == nil {
			t.Fatalf("Deals(%d) succeeded", response.code)
		}
	}
}

func TestFeesReportsMalformedTableAndHTTPFailures(t *testing.T) {
	for _, body := range []string{"<vy-spa-table settings='{'", "failed"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if body == "failed" {
				http.Error(w, body, http.StatusBadGateway)
				return
			}
			_, _ = io.WriteString(w, body)
		}))
		_, err := (&Client{HTTP: server.Client(), WWW: server.URL}).Fees(context.Background())
		server.Close()
		if err == nil {
			t.Fatalf("Fees(%q) succeeded", body)
		}
	}
}

func TestRoutesAndStatusPropagateReadErrorsAndEmptyRoutes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "failed", http.StatusBadGateway) }))
	if _, err := (&Client{HTTP: server.Client(), Static: server.URL}).Routes(context.Background(), "BCN"); err == nil || !strings.Contains(err.Error(), "read Vueling's route network") {
		t.Fatalf("Routes read error = %v", err)
	}
	if _, err := (&Client{HTTP: server.Client(), APIW: server.URL}).FlightStatus(context.Background(), "", "BCN", "BIO", time.Now()); err == nil {
		t.Fatal("FlightStatus ignored an API read error")
	}
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `{"flights":[]}`) })
	if _, err := (&Client{HTTP: server.Client(), APIW: server.URL}).FlightStatus(context.Background(), "", "BCN", "BIO", time.Now()); !errors.Is(err, ErrNoFlightStatus) {
		t.Fatalf("empty route FlightStatus error = %v", err)
	}
	server.Close()
}

func TestUnusualFlightMappingInputs(t *testing.T) {
	for code, want := range map[string]string{"BA": "Basic", "OP": "Optima", "TF": "TimeFlex", "X": "X"} {
		if got := bundleName(code); got != want {
			t.Errorf("bundleName(%q) = %q, want %q", code, got, want)
		}
	}
	if got := bundleFares(nil); got != nil {
		t.Fatalf("bundleFares(empty) = %v, want nil", got)
	}
	if pickMatches(journey{}, "", false) {
		t.Fatal("pickMatches accepted a journey with no segments")
	}
}

func TestFlightsPropagatesReturnFareAndAvailabilityFailures(t *testing.T) {
	response := readFixture(t, "flights-bcn-bio-2026-11-03.json")
	calls := 0
	client := availabilityServer(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 2 {
			http.Error(w, "return unavailable", http.StatusBadGateway)
			return
		}
		_, _ = w.Write(response)
	})
	query := FlightQuery{Origin: "BCN", Destination: "BIO", Date: bcnBioDate, ReturnDate: bcnBioDate.AddDate(0, 0, 5), Adults: 1, Currency: "EUR"}
	if _, err := client.Flights(context.Background(), query); err == nil || !strings.Contains(err.Error(), "read Vueling return flights") {
		t.Fatalf("Flights return availability error = %v", err)
	}

	negativeFare := `{"data":{"amsAvy":{"faresAvailable":[{"value":{"fareAvailabilityKey":"a","fares":[{"productClass":"BA","passengerFares":[{"amsFareAmount":-1}]}]}}]}}}`
	client = availabilityServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(negativeFare)) })
	if _, err := client.Flights(context.Background(), query); err == nil || !strings.Contains(err.Error(), "negative fare amount") {
		t.Fatalf("Flights negative fare error = %v", err)
	}
}

func TestFlightsSkipsUnbookableJourneysAndUsesRequestedCurrency(t *testing.T) {
	noSegments := `{"data":{"amsAvy":{"trips":[{"trips":[{"journeysAvailableByMarket":[{"key":"BCN|BIO","value":[{"fares":[{"fareAvailabilityKey":"a","fareSellKey":"sell"}]}]}]}]}],"faresAvailable":[{"value":{"fareAvailabilityKey":"a","fares":[{"productClass":"BA","passengerFares":[{"amsFareAmount":20}]}]}}]}}}`
	client := availabilityServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(noSegments)) })
	query := FlightQuery{Origin: "BCN", Destination: "BIO", Date: bcnBioDate, Adults: 1, Currency: "EUR"}
	flights, err := client.Flights(context.Background(), query)
	if err != nil || len(flights) != 0 {
		t.Fatalf("Flights unbookable journey = %+v, %v", flights, err)
	}

	noCurrency := strings.Replace(noSegments, `"fares":[{"fareAvailabilityKey":"a","fareSellKey":"sell"}]`, `"segments":[{"identifier":{"carrierCode":"VY","identifier":"1234"}}],"designator":{"origin":"BCN","destination":"BIO","departure":"2026-11-03T07:00:00Z","arrival":"2026-11-03T08:00:00Z"},"fares":[{"fareAvailabilityKey":"a","fareSellKey":"sell"}]`, 1)
	client = availabilityServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(noCurrency)) })
	flights, err = client.Flights(context.Background(), query)
	if err != nil || len(flights) != 1 || flights[0].Currency != "EUR" {
		t.Fatalf("Flights currency fallback = %+v, %v", flights, err)
	}
}

func TestQuotePropagatesFareMappingErrors(t *testing.T) {
	negativeFare := `{"data":{"amsAvy":{"faresAvailable":[{"value":{"fareAvailabilityKey":"a","fares":[{"productClass":"BA","passengerFares":[{"amsFareAmount":-1}]}]}}]}}}`
	client := quoteServer(t, negativeFare, quoteBreakdown, new([]string))
	if _, err := client.Quote(context.Background(), quoteQuery, "VY1460"); err == nil || !strings.Contains(err.Error(), "negative fare amount") {
		t.Fatalf("Quote fare mapping error = %v", err)
	}
}

func TestScheduledDaysKeepsTheCheapestDuplicate(t *testing.T) {
	days, err := scheduledDays([]byte(`[
{"departureDate":"2027-01-02T09:00:00","price":5,"isAvailableDay":true},
{"departureDate":"2027-01-02T10:00:00","price":8,"isAvailableDay":true}]`))
	if err != nil || len(days) != 1 || days[0].Departure != "09:00" || days[0].Fare != "5 €" {
		t.Fatalf("scheduledDays duplicate = %+v, %v", days, err)
	}
}
