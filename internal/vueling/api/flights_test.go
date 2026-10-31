package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

func availabilityServer(t *testing.T, graphql http.HandlerFunc) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /asm/v1/Auth", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["profileId"] != anonymousProfileID || r.Header.Get("Origin") != ticketsOrigin {
			http.Error(w, "bad auth request", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"tokenType":"Bearer","accessToken":"jwt","expiration":1199,"userType":"Anonymous"}`))
	})
	mux.HandleFunc("POST /avy/v1/graphql", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer jwt" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		graphql(w, r)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return &Client{HTTP: server.Client(), AMS: server.URL}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func decodeJSON(t *testing.T, data []byte) any {
	t.Helper()
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

var bcnBioDate = time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)

func TestFlightsSendsTheBrowserRequestAndMapsEveryJourney(t *testing.T) {
	wantRequest := decodeJSON(t, readFixture(t, "flights-request.json"))
	response := readFixture(t, "flights-bcn-bio-2026-11-03.json")
	client := availabilityServer(t, func(w http.ResponseWriter, r *http.Request) {
		var got any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil || !reflect.DeepEqual(got, wantRequest) {
			http.Error(w, "unexpected availability request", http.StatusBadRequest)
			return
		}
		_, _ = w.Write(response)
	})
	flights, err := client.Flights(context.Background(), FlightQuery{Origin: "BCN", Destination: "BIO", Date: bcnBioDate, Adults: 1, Currency: "EUR"})
	if err != nil {
		t.Fatal(err)
	}
	fares := map[string]string{}
	for _, flight := range flights {
		fares[flight.Flight] = flight.Departure + " " + flight.Fare
	}
	want := map[string]string{"VY1460": "07:00 197.99", "VY1462": "08:55 102.99", "VY1466": "12:35 52.99", "VY1464": "13:30 47.99", "VY1470": "15:35 57.99", "VY1478": "21:05 121.99", "VY1480": "21:35 52.99"}
	if !reflect.DeepEqual(fares, want) {
		t.Fatalf("fares = %v, want %v", fares, want)
	}
	cheapest, err := vueling.FilterFlights(flights, vueling.FlightFilter{Cheapest: true, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	wantFlight := vueling.Flight{Direction: "outbound", Date: "2026-11-03", Flight: "VY1464", Departure: "13:30", Arrival: "14:50", Fare: "47.99", Currency: "EUR", Itinerary: "13:30 BCN → 14:50 BIO VY1464 Directo", Flights: []string{"VY1464"}}
	if len(cheapest) != 1 || !reflect.DeepEqual(cheapest[0], wantFlight) {
		t.Fatalf("cheapest = %+v, want %+v", cheapest, wantFlight)
	}
}

func TestFlightsSearchesTheReturnLegWithChildren(t *testing.T) {
	response := readFixture(t, "flights-bcn-bio-2026-11-03.json")
	var routes []string
	client := availabilityServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Variables struct {
				RequestAVY struct {
					Request struct {
						Criteria   []map[string]string
						Passengers []map[string]any
					}
				}
			}
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		request := body.Variables.RequestAVY.Request
		if len(request.Passengers) != 2 || request.Passengers[1]["type"] != "Child" || request.Passengers[1]["count"] != 2.0 {
			http.Error(w, "unexpected passengers", http.StatusBadRequest)
			return
		}
		routes = append(routes, request.Criteria[0]["origin"]+"-"+request.Criteria[0]["destination"]+" "+request.Criteria[0]["date"])
		_, _ = w.Write(response)
	})
	flights, err := client.Flights(context.Background(), FlightQuery{Origin: "BCN", Destination: "BIO", Date: bcnBioDate, ReturnDate: bcnBioDate.AddDate(0, 0, 5), Adults: 2, Children: 2, Currency: "EUR"})
	if err != nil {
		t.Fatal(err)
	}
	wantRoutes := []string{"BCN-BIO 2026-11-03T00:00:00.000Z", "BIO-BCN 2026-11-08T00:00:00.000Z"}
	if !reflect.DeepEqual(routes, wantRoutes) {
		t.Fatalf("routes = %v, want %v", routes, wantRoutes)
	}
	last := flights[len(flights)-1]
	if len(flights) != 14 || flights[0].Direction != "outbound" || last.Direction != "return" || last.Date != "2026-11-08" {
		t.Fatalf("flights = %+v", flights)
	}
}

func TestFlightsReportsAvailabilityErrors(t *testing.T) {
	tests := []struct {
		name, body, want string
		status           int
	}{
		{"graphql error", `{"errors":[{"message":"Unexpected Execution Error"}],"data":{"amsAvy":null}}`, "Unexpected Execution Error", http.StatusOK},
		{"no data", `{"data":{}}`, "no availability data", http.StatusOK},
		{"http error", `boom`, "read Vueling outbound flights", http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := availabilityServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			})
			_, err := client.Flights(context.Background(), FlightQuery{Origin: "BCN", Destination: "BIO", Date: bcnBioDate, Adults: 1, Currency: "EUR"})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestFlightsNeedsASessionToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"accessToken":""}`))
	}))
	defer server.Close()
	_, err := (&Client{HTTP: server.Client(), AMS: server.URL}).Flights(context.Background(), FlightQuery{Origin: "BCN", Destination: "BIO", Date: bcnBioDate, Adults: 1})
	if err == nil || !strings.Contains(err.Error(), "session token") {
		t.Fatalf("error = %v, want a missing-token error", err)
	}
}

func TestFlightsReportsAFailedSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	defer server.Close()
	_, err := (&Client{HTTP: server.Client(), AMS: server.URL}).Flights(context.Background(), FlightQuery{Origin: "BCN", Destination: "BIO", Date: bcnBioDate, Adults: 1})
	if err == nil || !strings.Contains(err.Error(), "anonymous Vueling session") {
		t.Fatalf("error = %v, want a session error", err)
	}
}

func TestJourneyFlightDescribesConnectionsAndSkipsSoldOutJourneys(t *testing.T) {
	operator := "Operado por Iberia"
	operated := leg{}
	operated.LegInfo.OperatedByText = &operator
	j := journey{
		Designator: designator{Origin: "AGP", Destination: "BIO", Departure: "2026-11-03T07:10:00.000Z", Arrival: "2026-11-03T12:05:00.000Z"},
		Segments: []segment{
			{Identifier: flightNumber{"VY", "2213"}, Designator: designator{Origin: "AGP", Destination: "BCN"}},
			{Identifier: flightNumber{"VY", "1464"}, Designator: designator{Origin: "BCN", Destination: "BIO"}, Legs: []leg{operated}},
		},
		Fares: []fareReference{{"a"}, {"b"}},
	}

	flight, ok := journeyFlight(j, map[string]float64{"a": 80, "b": 64.5}, "return", "2026-11-03", "EUR")
	want := vueling.Flight{Direction: "return", Date: "2026-11-03", Flight: "VY2213", Departure: "07:10", Arrival: "12:05", Fare: "64.50", Currency: "EUR", Itinerary: "07:10 AGP → 12:05 BIO VY2213+VY1464 Conexión en BCN · Operado por Iberia", Stops: 1, Flights: []string{"VY2213", "VY1464"}, Via: []string{"BCN"}}
	if !ok || !reflect.DeepEqual(flight, want) {
		t.Fatalf("journeyFlight = %+v, %v; want %+v", flight, ok, want)
	}
	if _, ok := journeyFlight(j, map[string]float64{}, "outbound", "2026-11-03", "EUR"); ok {
		t.Fatal("a journey without fares must be skipped")
	}
}

func TestRouteKindAndLocalClock(t *testing.T) {
	if got := routeKind(nil, 2); got != "Con escala" {
		t.Fatalf("routeKind = %q", got)
	}
	if got := localClock("not a time"); got != "not a time" {
		t.Fatalf("localClock = %q", got)
	}
}
