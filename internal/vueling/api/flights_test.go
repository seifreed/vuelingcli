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
	wantFlight := vueling.Flight{Direction: "outbound", Date: "2026-11-03", Flight: "VY1464", Departure: "13:30", Arrival: "14:50", Fare: "47.99", Fares: map[string]string{"Basic": "47.99", "Optima": "100.99"}, Currency: "EUR", Itinerary: "13:30 BCN → 14:50 BIO VY1464 Directo", Flights: []string{"VY1464"}}
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
		answer := response
		if request.Criteria[0]["origin"] == "BIO" {
			answer = []byte(strings.NewReplacer(`"origin":"BCN"`, `"origin":"BIO"`, `"destination":"BIO"`, `"destination":"BCN"`).Replace(string(response)))
		}
		_, _ = w.Write(answer)
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
		{"partial graphql error", `{"errors":[{"message":"Partial Execution Error"}],"data":{"amsAvy":{"currencyCode":"EUR"}}}`, "Partial Execution Error", http.StatusOK},
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

func TestFlightsRejectsBookableJourneysWithoutFlightNumbers(t *testing.T) {
	response := `{"data":{"amsAvy":{"trips":[{"trips":[{"journeysAvailableByMarket":[{"value":[{"designator":{"origin":"BCN","destination":"BIO","departure":"2026-11-03T07:00:00Z","arrival":"2026-11-03T08:20:00Z"},"segments":[{"identifier":{"carrierCode":"VY","identifier":""}}],"fares":[{"fareAvailabilityKey":"a","fareSellKey":"sell-a"}]}]}]}]}],"faresAvailable":[{"value":{"fareAvailabilityKey":"a","fares":[{"productClass":"BA","passengerFares":[{"amsFareAmount":20}]}]}}]}}}`
	client := availabilityServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(response)) })
	_, err := client.Flights(context.Background(), FlightQuery{Origin: "BCN", Destination: "BIO", Date: bcnBioDate, Adults: 1, Currency: "EUR"})
	if err == nil || !strings.Contains(err.Error(), "incomplete flight number") {
		t.Fatalf("Flights error = %v, want incomplete flight number error", err)
	}
}

func TestFlightsRejectsBookableJourneysOnDifferentRoutes(t *testing.T) {
	response := `{"data":{"amsAvy":{"trips":[{"trips":[{"journeysAvailableByMarket":[{"value":[{"designator":{"origin":"MAD","destination":"BIO","departure":"2026-11-03T07:00:00Z","arrival":"2026-11-03T08:20:00Z"},"segments":[{"identifier":{"carrierCode":"VY","identifier":"1460"}}],"fares":[{"fareAvailabilityKey":"a","fareSellKey":"sell-a"}]}]}]}]}],"faresAvailable":[{"value":{"fareAvailabilityKey":"a","fares":[{"productClass":"BA","passengerFares":[{"amsFareAmount":20}]}]}}]}}}`
	client := availabilityServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(response)) })
	_, err := client.Flights(context.Background(), FlightQuery{Origin: "BCN", Destination: "BIO", Date: bcnBioDate, Adults: 1, Currency: "EUR"})
	if err == nil || !strings.Contains(err.Error(), "requested route BCN to BIO") {
		t.Fatalf("Flights error = %v, want route mismatch error", err)
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
		Fares: []fareReference{{FareAvailabilityKey: "a", FareSellKey: "sell-a"}, {FareAvailabilityKey: "b", FareSellKey: "sell-b"}},
	}

	flight, ok := journeyFlight(j, map[string]farePrice{"a": {80, "Optima"}, "b": {64.5, "Basic"}}, "return", "2026-11-03", "EUR")
	want := vueling.Flight{Direction: "return", Date: "2026-11-03", Flight: "VY2213", Departure: "07:10", Arrival: "12:05", Fare: "64.50", Fares: map[string]string{"Basic": "64.50", "Optima": "80.00"}, Currency: "EUR", Itinerary: "07:10 AGP → 12:05 BIO VY2213+VY1464 Conexión en BCN · Operado por Iberia", Stops: 1, Flights: []string{"VY2213", "VY1464"}, Via: []string{"BCN"}}
	if !ok || !reflect.DeepEqual(flight, want) {
		t.Fatalf("journeyFlight = %+v, %v; want %+v", flight, ok, want)
	}
	if _, ok := journeyFlight(j, map[string]farePrice{}, "outbound", "2026-11-03", "EUR"); ok {
		t.Fatal("a journey without fares must be skipped")
	}
	j.Fares[0].FareSellKey = ""
	flight, ok = journeyFlight(j, map[string]farePrice{"a": {30, "Basic"}, "b": {64.5, "Optima"}}, "outbound", "2026-11-03", "EUR")
	if !ok || flight.Fare != "64.50" || !reflect.DeepEqual(flight.Fares, map[string]string{"Optima": "64.50"}) {
		t.Fatalf("journeyFlight with an unsellable cheapest fare = %+v, %v; want only the sellable fare", flight, ok)
	}
}

func TestRouteKindAndLocalClock(t *testing.T) {
	if got := routeKind(nil, 2); got != "Con escala" {
		t.Fatalf("routeKind = %q", got)
	}
	if got := localClock("not a time"); got != "not a time" {
		t.Fatalf("localClock = %q", got)
	}
	if got := laterArrivalDate("not a time", "2026-11-11T00:45:00Z"); got != "" {
		t.Fatalf("laterArrivalDate with an unreadable departure = %q", got)
	}
}

func TestJourneyFlightMarksOvernightArrivals(t *testing.T) {
	j := journey{
		Designator: designator{Origin: "BCN", Destination: "AMS", Departure: "2026-11-10T23:30:00Z", Arrival: "2026-11-11T00:45:00Z"},
		Segments:   []segment{{Identifier: flightNumber{"VY", "8318"}, Designator: designator{Origin: "BCN", Destination: "AMS"}}},
		Fares:      []fareReference{{FareAvailabilityKey: "a", FareSellKey: "sell-a"}},
	}
	flight, ok := journeyFlight(j, map[string]farePrice{"a": {59.99, "Basic"}}, "outbound", "2026-11-10", "EUR")
	if !ok || flight.Arrival != "00:45" || flight.ArrivalDate != "2026-11-11" || flight.Itinerary != "23:30 BCN → 00:45 +1 AMS VY8318 Directo" {
		t.Fatalf("journeyFlight = %+v, %v; want an arrival marked +1 on 2026-11-11", flight, ok)
	}
	j.Designator.Arrival = "2026-11-10T23:55:00Z"
	if flight, _ := journeyFlight(j, map[string]farePrice{"a": {59.99, "Basic"}}, "outbound", "2026-11-10", "EUR"); flight.ArrivalDate != "" {
		t.Fatalf("same-day ArrivalDate = %q, want none", flight.ArrivalDate)
	}
}

func TestFlightsSumsSegmentFaresOfConnections(t *testing.T) {
	response := readFixture(t, "flights-svq-ams-2026-11-12.json")
	client := availabilityServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(response) })
	flights, err := client.Flights(context.Background(), FlightQuery{Origin: "SVQ", Destination: "AMS", Date: time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC), Adults: 1, Currency: "EUR"})
	if err != nil {
		t.Fatal(err)
	}
	fares := map[string]string{}
	for _, flight := range flights {
		fares[strings.Join(flight.Flights, "+")] = flight.Fare
	}
	// The website shows 274 € and 282 € for these connections: 170.23 + 103.39 and 170.23 + 111.39.
	if fares["VY1393+VY1050"] != "273.62" || fares["VY1383+VY1050"] != "281.62" {
		t.Fatalf("connection fares = %v, want summed segment fares", fares)
	}
}

func TestAvailabilityRequestAddsInfantsAndDiscountServices(t *testing.T) {
	request := availabilityRequest(FlightQuery{Adults: 2, Infants: 1, Discount: 4, Currency: "EUR"}, "BCN", "PMI", bcnBioDate)
	services := request["variables"].(map[string]any)["requestAVY"].(map[string]any)["request"].(map[string]any)["servicesToRequest"]
	want := []map[string]any{{"count": 1, "serviceCode": "INFANT"}, {"count": 1, "serviceCode": "FAMIRESO"}, {"count": 1, "serviceCode": "FAMRES"}}
	if !reflect.DeepEqual(services, want) {
		t.Fatalf("servicesToRequest = %v, want %v", services, want)
	}
}
