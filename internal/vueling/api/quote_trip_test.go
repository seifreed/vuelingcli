package api

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestQuoteTripPricesEachDirectionWithItsOwnBreakdown(t *testing.T) {
	var sold []string
	client := quoteServer(t, quoteAvailability, quoteBreakdown, &sold)
	query := quoteQuery
	query.ReturnDate = time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	quotes, err := client.QuoteTrip(context.Background(), query, Pick{Flight: "VY1460"}, Pick{})
	if err != nil {
		t.Fatal(err)
	}
	if len(quotes) != 2 || quotes[0].Date != "2026-11-10" || quotes[1].Date != "2026-11-15" || quotes[1].Flight != "VY1460" || quotes[1].Total != 194.97 {
		t.Fatalf("quotes = %+v", quotes)
	}
	if !reflect.DeepEqual(sold, []string{"sell-ba", "sell-ba"}) {
		t.Fatalf("breakdown fare sell keys = %v, want one request per direction", sold)
	}
	oneWay, err := client.QuoteTrip(context.Background(), quoteQuery, Pick{Flight: "VY1383"}, Pick{Flight: "VY9999"})
	if err != nil || len(oneWay) != 1 || oneWay[0].Flight != "VY1383+VY1050" {
		t.Fatalf("one-way quotes = %+v, %v", oneWay, err)
	}
	if _, err := client.QuoteTrip(context.Background(), query, Pick{}, Pick{Flight: "VY9999"}); err == nil || !strings.Contains(err.Error(), "no bookable flight VY9999 from BIO to BCN on 2026-11-15") {
		t.Fatalf("missing return flight error = %v", err)
	}
}

func TestPickFareChoosesTheCheapestJourneyAndHonoursDirect(t *testing.T) {
	build := func(fareKey string, numbers ...string) journey {
		var j journey
		for _, number := range numbers {
			j.Segments = append(j.Segments, segment{Identifier: flightNumber{CarrierCode: "VY", Identifier: number}, Legs: []leg{{}}})
		}
		j.Fares = []fareReference{{FareAvailabilityKey: fareKey, FareSellKey: "sell-" + fareKey}}
		return j
	}
	stopover := build("stop", "3000")
	stopover.Segments[0].Legs = append(stopover.Segments[0].Legs, leg{})
	journeys := []journey{build("direct", "1460"), build("connection", "1383", "1050"), stopover, build("soldout", "2000")}
	prices := map[string]farePrice{"direct": {amount: 80}, "connection": {amount: 40}, "stop": {amount: 30}}
	tests := []struct {
		pick Pick
		want string
	}{
		{Pick{}, "sell-stop"},
		{Pick{Direct: true}, "sell-direct"},
		{Pick{Flight: "vy1383"}, "sell-connection"},
		{Pick{Flight: "VY1383+VY1050", Direct: true}, "sell-connection"},
	}
	for _, test := range tests {
		if _, fare, ok := pickFare(journeys, prices, test.pick); !ok || fare.FareSellKey != test.want {
			t.Errorf("pickFare(%+v) = %q, %v, want %q", test.pick, fare.FareSellKey, ok, test.want)
		}
	}
	for _, pick := range []Pick{{Flight: "VY2000"}, {Flight: "VY1050"}} {
		if _, _, ok := pickFare(journeys, prices, pick); ok {
			t.Errorf("pickFare(%+v) found a fare", pick)
		}
	}
	if _, _, ok := pickFare(journeys[1:], prices, Pick{Direct: true}); ok {
		t.Error("pickFare found a direct fare among connections")
	}
	for pick, want := range map[Pick]string{{}: "flight", {Direct: true}: "direct flight", {Flight: "vy1460"}: "flight VY1460"} {
		if got := pickDescription(pick); got != want {
			t.Errorf("pickDescription(%+v) = %q, want %q", pick, got, want)
		}
	}
}
