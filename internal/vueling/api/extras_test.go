package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const extrasAvailability = `{"data":{"amsAvy":{"currencyCode":"EUR","trips":[{"trips":[{"journeysAvailableByMarket":[{"key":"BCN|BIO","value":[
{"journeyKey":"jk-1460","designator":{"origin":"BCN","destination":"BIO"},"segments":[{"identifier":{"carrierCode":"VY","identifier":"1460"},"designator":{"origin":"BCN","destination":"BIO"}}],"fares":[{"fareAvailabilityKey":"ba","fareSellKey":"sell-ba","details":[{"bundleReferences":["VllCQSEx","VPRO!5"]}]}]}]}]}]}],
"faresAvailable":[{"value":{"fareAvailabilityKey":"ba","fares":[{"productClass":"BA","passengerFares":[{"amsFareAmount":64.99}]}]}}],
"bundleOffers":[{"key":"VllCQSEx","value":{"bundleCode":"VYBA","bundlePrices":[{"totalPrice":0,"passengerType":"ADT"}]}},{"key":"VPRO!5","value":{"bundleCode":"VPRO","bundlePrices":[{"totalPrice":68,"passengerType":"ADT"}]}}]}}}`

const extrasSeatMap = `{"seatMaps":[{"passengerFees":[{"passengerKey":"MCFBRFQ-","value":{"groups":[{"groupNumber":4,"amount":7.0},{"groupNumber":21,"amount":11.0},{"groupNumber":27,"amount":9.0},{"groupNumber":51,"amount":14.0},{"groupNumber":77,"amount":45.0}]}}],
"seatMap":{"decks":[{"value":{"compartments":[{"value":{"sections":[
{"seatFam":{"name":"fake"},"units":[{"type":"LabelRuler","group":0,"assignable":false}]},
{"seatFam":{"name":"spaceone"},"units":[{"type":"NormalSeat","group":77,"assignable":true,"isSeatAvailable":false}]},
{"seatFam":{"name":"front"},"units":[{"type":"NormalSeat","group":27,"assignable":true,"isSeatAvailable":true},{"type":"NormalSeat","group":51,"assignable":true,"isSeatAvailable":true},{"type":"Exit","group":0}]},
{"seatFam":{"name":"back"},"units":[{"type":"NormalSeat","group":4,"assignable":true,"isSeatAvailable":true},{"type":"NormalSeat","group":99,"assignable":true,"isSeatAvailable":true}]},
{"seatFam":{"name":"back"},"units":[{"type":"NormalSeat","group":21,"assignable":true,"isSeatAvailable":false}]}]}}]}}]}}]}`

const extrasServices = `{"currencyCode":"EUR","paxSegmentsService":[
{"serviceCode":"WEAP","applyPax":"SpecifyPax","paxJourneySegments":[{"paxPrice":50,"journeySegments":[{}]}]},
{"serviceCode":"WCHR","applyPax":"SpecifyPax","paxJourneySegments":[{"journeySegments":[{"journeyPrice":0.0}]}]},
{"serviceCode":"RFCS","applyPax":"AllPax","price":0,"paxJourneySegments":[]},
{"serviceCode":"PETC","applyPax":"SpecifyPax","paxJourneySegments":[{"paxPrice":50,"journeySegments":[{}]}]},
{"serviceCode":"HBBG","applyPax":"SpecifyPax","paxJourneySegments":[{"journeySegments":[{"journeyPrice":140.0}]}]},
{"serviceCode":"GCBG","applyPax":"SpecifyPax","paxJourneySegments":[{"journeySegments":[{"journeyPrice":30.0}]}]},
{"serviceCode":"FXPK","applyPax":"AllPax","price":14},
{"serviceCode":"CARR","applyPax":"SpecifyPax","paxJourneySegments":[{"journeySegments":[{"segmentPrice":2.5}]}]},
{"serviceCode":"BG25","applyPax":"SpecifyPax","weight":25,"paxJourneySegments":[{"journeySegments":[{"journeyPrice":40.0}]}]},
{"serviceCode":"BG15","applyPax":"SpecifyPax","weight":15,"paxJourneySegments":[{"journeySegments":[{"journeyPrice":33.0}]}]}]}`

type extrasServer struct {
	mu           sync.Mutex
	calls        []string
	sell         map[string]any
	fail         string
	failDelete   bool
	failSale     bool
	availability string
	seatMap      string
	services     string
}

func newExtrasServer(t *testing.T, fail string) (*Client, *extrasServer) {
	t.Helper()
	recorded := &extrasServer{fail: fail}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorded.mu.Lock()
		recorded.calls = append(recorded.calls, r.Method+" "+r.URL.Path)
		recorded.mu.Unlock()
		if r.URL.Path != "/asm/v1/Auth" && r.Header.Get("Authorization") != "Bearer extras-jwt" {
			t.Errorf("%s %s sent %q, want the extras session's token", r.Method, r.URL.Path, r.Header.Get("Authorization"))
		}
		if r.URL.Path == recorded.fail || r.Method+" "+r.URL.Path == recorded.fail || (recorded.failDelete && r.Method == http.MethodDelete) {
			http.Error(w, `{"errors":[{"code":1}]}`, http.StatusInternalServerError)
			return
		}
		switch r.Method + " " + r.URL.Path {
		case "POST /asm/v1/Auth":
			_ = json.NewEncoder(w).Encode(map[string]any{"accessToken": "extras-jwt", "expiration": 1200})
		case "DELETE /asm/v1/Auth":
			if r.Header.Get("Authorization") != "Bearer extras-jwt" {
				t.Errorf("DELETE Auth sent %q, want the extras session's token", r.Header.Get("Authorization"))
			}
		case "POST /avy/v1/graphql":
			body := recorded.availability
			if body == "" {
				body = extrasAvailability
			}
			_, _ = w.Write([]byte(body))
		case "POST /bkj/v2/Sell/selltrip/BSV":
			_ = json.NewDecoder(r.Body).Decode(&recorded.sell)
			if recorded.failSale {
				_, _ = w.Write([]byte(`{"success":false}`))
			} else {
				_, _ = w.Write([]byte(`{"success":true}`))
			}
		case "GET /bst/v2/SmartSeatMap":
			body := recorded.seatMap
			if body == "" {
				body = extrasSeatMap
			}
			_, _ = w.Write([]byte(body))
		case "GET /bpv/v3/BookingServices/Services/BPV":
			body := recorded.services
			if body == "" {
				body = extrasServices
			}
			_, _ = w.Write([]byte(body))
		default:
			t.Errorf("extras called %s %s, which is not on its allow-list", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return &Client{HTTP: server.Client(), AMS: server.URL}, recorded
}

func extrasQuery() FlightQuery {
	return FlightQuery{Origin: "BCN", Destination: "BIO", Date: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC), Adults: 1, Currency: "EUR"}
}

func TestServicesPricesRejectNegativePrices(t *testing.T) {
	var response servicesResponse
	if err := json.Unmarshal([]byte(`{"paxSegmentsService":[{"serviceCode":"PETC","price":-1}]}`), &response); err != nil {
		t.Fatal(err)
	}
	_, _, err := response.prices("EUR")
	if err == nil || !strings.Contains(err.Error(), "negative price for service PETC") {
		t.Fatalf("prices error = %v, want negative PETC price error", err)
	}
}

func TestExtrasSellsTheFareReadsPricesAndEndsTheSession(t *testing.T) {
	client, recorded := newExtrasServer(t, "")
	extras, err := client.Extras(context.Background(), extrasQuery(), Pick{Flight: "VY1460"}, DefaultExtrasBundle)
	if err != nil {
		t.Fatal(err)
	}
	want := "POST /asm/v1/Auth,POST /avy/v1/graphql,POST /bkj/v2/Sell/selltrip/BSV,GET /bst/v2/SmartSeatMap,GET /bpv/v3/BookingServices/Services/BPV,DELETE /asm/v1/Auth"
	if got := strings.Join(recorded.calls, ","); got != want {
		t.Fatalf("calls = %s, want %s", got, want)
	}
	sell, _ := json.Marshal(recorded.sell)
	if string(sell) != `{"bundleCodesByJourney":{"jk-1460":"VYBA"},"extraSeats":0,"journeyKeys":[{"fareSellKey":"sell-ba"}]}` {
		t.Fatalf("sell body = %s", sell)
	}
	if extras.Flight != "VY1460" || extras.Origin != "BCN" || extras.Destination != "BIO" || extras.Bundle != "VYBA" || extras.Currency != "EUR" || extras.Date != "2026-11-10" {
		t.Fatalf("extras = %+v", extras)
	}
	seats, _ := json.Marshal(extras.Seats)
	wantSeats := `[{"code":"spaceone","name":"Space One (first row)","price":45,"maxPrice":45,"available":0,"currency":"EUR","perPassenger":true},` +
		`{"code":"front","name":"Front","price":9,"maxPrice":14,"available":2,"currency":"EUR","perPassenger":true},` +
		`{"code":"back","name":"Back","price":7,"maxPrice":11,"available":1,"currency":"EUR","perPassenger":true}]`
	if string(seats) != wantSeats {
		t.Fatalf("seats = %s\nwant %s", seats, wantSeats)
	}
	bags, _ := json.Marshal(extras.Bags)
	wantBags := `[{"code":"GCBG","name":"Overhead cabin bag","price":30,"currency":"EUR","perPassenger":true},` +
		`{"code":"BG15","name":"Checked bag (15 kg)","price":33,"currency":"EUR","perPassenger":true},` +
		`{"code":"BG25","name":"Checked bag (25 kg)","price":40,"currency":"EUR","perPassenger":true},` +
		`{"code":"HBBG","name":"Heavy bag at the boarding gate","price":140,"currency":"EUR","perPassenger":true}]`
	if string(bags) != wantBags {
		t.Fatalf("bags = %s\nwant %s", bags, wantBags)
	}
	services, _ := json.Marshal(extras.Services)
	wantServices := `[{"code":"WEAP","name":"Sports firearm","price":50,"currency":"EUR","perPassenger":true},` +
		`{"code":"PETC","name":"Pet in the cabin","price":50,"currency":"EUR","perPassenger":true},` +
		`{"code":"FXPK","name":"Flex Pack (cancellation + flight change)","price":14,"currency":"EUR"},` +
		`{"code":"CARR","name":"CARR","price":2.5,"currency":"EUR","perPassenger":true}]`
	if string(services) != wantServices {
		t.Fatalf("services = %s\nwant %s", services, wantServices)
	}
}

func TestExtrasEndsTheSessionWhenAStepFails(t *testing.T) {
	for _, failing := range []string{"/bkj/v2/Sell/selltrip/BSV", "/bst/v2/SmartSeatMap", "/bpv/v3/BookingServices/Services/BPV", "/avy/v1/graphql"} {
		client, recorded := newExtrasServer(t, failing)
		if _, err := client.Extras(context.Background(), extrasQuery(), Pick{Flight: "VY1460"}, DefaultExtrasBundle); err == nil {
			t.Fatalf("failing %s: Extras returned no error", failing)
		}
		if last := recorded.calls[len(recorded.calls)-1]; last != "DELETE /asm/v1/Auth" {
			t.Fatalf("failing %s: last call = %s, want the session ended", failing, last)
		}
	}
}

func TestExtrasRefusesUnofferedBundlesAndUnknownFlightsBeforeSelling(t *testing.T) {
	for pick, bundle := range map[string]string{"VY1460": "VCB2", "VY9999": "VYBA"} {
		client, recorded := newExtrasServer(t, "")
		_, err := client.Extras(context.Background(), extrasQuery(), Pick{Flight: pick}, bundle)
		if err == nil {
			t.Fatalf("Extras(%s, %s) returned no error", pick, bundle)
		}
		for _, call := range recorded.calls {
			if strings.Contains(call, "Sell") {
				t.Fatalf("Extras(%s, %s) sold the fare: %v", pick, bundle, recorded.calls)
			}
		}
		if recorded.calls[len(recorded.calls)-1] != "DELETE /asm/v1/Auth" {
			t.Fatalf("Extras(%s, %s) did not end the session: %v", pick, bundle, recorded.calls)
		}
	}
}

func TestExtrasReportsASessionThatCannotBeEnded(t *testing.T) {
	client, _ := newExtrasServer(t, "DELETE /asm/v1/Auth")
	if _, err := client.Extras(context.Background(), extrasQuery(), Pick{Flight: "VY1460"}, DefaultExtrasBundle); err == nil || !strings.Contains(err.Error(), "end the Vueling session") {
		t.Fatalf("Extras error = %v, want the failed session end", err)
	}
}

func TestExtrasReportsSessionCleanupFailureAlongsideOperationFailure(t *testing.T) {
	client, recorded := newExtrasServer(t, "/avy/v1/graphql")
	recorded.failDelete = true
	_, err := client.Extras(context.Background(), extrasQuery(), Pick{Flight: "VY1460"}, DefaultExtrasBundle)
	if err == nil || strings.Count(err.Error(), "HTTP 500") != 2 || !strings.Contains(err.Error(), "read Vueling outbound flights") || !strings.Contains(err.Error(), "end the Vueling session") {
		t.Fatalf("Extras error = %v; want both failures with their operation context", err)
	}
}

func TestExtrasCoversSessionAvailabilityPricingAndSaleFailures(t *testing.T) {
	t.Run("open session", func(t *testing.T) {
		client, _ := newExtrasServer(t, "POST /asm/v1/Auth")
		if _, err := client.Extras(context.Background(), extrasQuery(), Pick{Flight: "VY1460"}, DefaultExtrasBundle); err == nil {
			t.Fatal("Extras ignored an open-session failure")
		}
	})
	t.Run("availability response errors", func(t *testing.T) {
		client, recorded := newExtrasServer(t, "")
		recorded.availability = `{"data":{"amsAvy":null},"errors":[{"message":"unavailable"}]}`
		if _, err := client.Extras(context.Background(), extrasQuery(), Pick{Flight: "VY1460"}, DefaultExtrasBundle); err == nil {
			t.Fatal("Extras accepted an availability response with GraphQL errors")
		}
	})
	t.Run("negative fare", func(t *testing.T) {
		client, recorded := newExtrasServer(t, "")
		recorded.availability = strings.Replace(extrasAvailability, `"amsFareAmount":64.99`, `"amsFareAmount":-1`, 1)
		if _, err := client.Extras(context.Background(), extrasQuery(), Pick{Flight: "VY1460"}, DefaultExtrasBundle); err == nil {
			t.Fatal("Extras accepted a negative fare")
		}
	})
	t.Run("unsuccessful sale", func(t *testing.T) {
		client, recorded := newExtrasServer(t, "")
		recorded.failSale = true
		if _, err := client.Extras(context.Background(), extrasQuery(), Pick{Flight: "VY1460"}, DefaultExtrasBundle); err == nil || !strings.Contains(err.Error(), "did not put the flight") {
			t.Fatalf("Extras sale error = %v", err)
		}
	})
	t.Run("negative seat price", func(t *testing.T) {
		client, recorded := newExtrasServer(t, "")
		recorded.seatMap = strings.Replace(extrasSeatMap, `"groupNumber":4,"amount":7.0`, `"groupNumber":4,"amount":-7.0`, 1)
		if _, err := client.Extras(context.Background(), extrasQuery(), Pick{Flight: "VY1460"}, DefaultExtrasBundle); err == nil {
			t.Fatal("Extras accepted a negative seat price")
		}
	})
	t.Run("negative service price", func(t *testing.T) {
		client, recorded := newExtrasServer(t, "")
		recorded.services = strings.Replace(extrasServices, `"journeyPrice":140.0`, `"journeyPrice":-140.0`, 1)
		if _, err := client.Extras(context.Background(), extrasQuery(), Pick{Flight: "VY1460"}, DefaultExtrasBundle); err == nil {
			t.Fatal("Extras accepted a negative service price")
		}
	})
	t.Run("availability currency fallback and empty passenger fees", func(t *testing.T) {
		client, recorded := newExtrasServer(t, "")
		recorded.services = strings.Replace(extrasServices, `"currencyCode":"EUR"`, `"currencyCode":""`, 1)
		recorded.seatMap = `{"seatMaps":[{"passengerFees":[]}]}`
		got, err := client.Extras(context.Background(), extrasQuery(), Pick{Flight: "VY1460"}, DefaultExtrasBundle)
		if err != nil || got.Currency != "EUR" || len(got.Seats) != 0 {
			t.Fatalf("Extras fallback = %+v, %v", got, err)
		}
	})
}

func TestEndSessionReportsAFailedDelete(t *testing.T) {
	client, _ := newExtrasServer(t, "/asm/v1/Auth")
	if err := client.endSession(context.Background(), "extras-jwt"); err == nil || !strings.Contains(err.Error(), "expires by itself") {
		t.Fatalf("endSession error = %v", err)
	}
}

func TestSeatPricesRejectNegativeGroupPrices(t *testing.T) {
	data := strings.Replace(extrasSeatMap, `"groupNumber":4,"amount":7.0`, `"groupNumber":4,"amount":-7.0`, 1)
	if data == extrasSeatMap {
		t.Fatal("test fixture did not replace the seat price")
	}
	var seatMap seatMapResponse
	if err := json.Unmarshal([]byte(data), &seatMap); err != nil {
		t.Fatal(err)
	}
	if _, err := seatMap.seatPrices(journey{}, "EUR"); err == nil || !strings.Contains(err.Error(), "negative seat price") {
		t.Fatalf("seatPrices error = %v, want a negative group price error", err)
	}
}

func TestSeatPricesNameTheLegOfAConnection(t *testing.T) {
	var seatMap seatMapResponse
	single := strings.TrimSuffix(strings.TrimPrefix(extrasSeatMap, `{"seatMaps":[`), `]}`)
	if err := json.Unmarshal([]byte(`{"seatMaps":[`+single+`,`+single+`]}`), &seatMap); err != nil {
		t.Fatal(err)
	}
	j := journey{Segments: []segment{
		{Identifier: flightNumber{CarrierCode: "VY", Identifier: "1383"}, Designator: designator{Origin: "BCN", Destination: "AMS"}},
		{Identifier: flightNumber{CarrierCode: "VY", Identifier: "1050"}, Designator: designator{Origin: "AMS", Destination: "BIO"}},
	}}
	prices, err := seatMap.seatPrices(j, "EUR")
	if err != nil {
		t.Fatal(err)
	}
	if len(prices) != 6 || prices[0].Segment != "VY1383 BCN-AMS" || prices[3].Segment != "VY1050 AMS-BIO" {
		t.Fatalf("seat prices = %+v", prices)
	}
}
