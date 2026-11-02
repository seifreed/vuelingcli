package vueling

import (
	"strings"
	"testing"
	"time"
)

func TestCheapestTripsReturnsSelectionLinkErrors(t *testing.T) {
	date := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	_, err := CheapestTrips(
		[]CalendarDay{{Date: date.Format("02/01/2006"), Fare: "10 €"}},
		[]CalendarDay{{Date: date.AddDate(0, 0, 1).Format("02/01/2006"), Fare: "20 €"}},
		TripSearch{Origin: "BC", Destination: "BIO", From: date, To: date, MinNights: 1, MaxNights: 1, Currency: "EUR"},
	)
	if err == nil || !strings.Contains(err.Error(), "build trip selection link") {
		t.Fatalf("CheapestTrips error = %v, want the invalid link error", err)
	}
}

func TestCheapestTripsPairsSortsFiltersAndLimitsFares(t *testing.T) {
	start := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	date := func(day int) string { return start.AddDate(0, 0, day-1).Format("02/01/2006") }
	outbound := []CalendarDay{{date(1), "10 €"}, {date(2), "10 €"}, {date(3), "unpriced"}}
	inbound := []CalendarDay{{date(2), "1 €"}, {date(3), "1 €"}, {date(4), "2 €"}}
	search := TripSearch{Origin: "BCN", Destination: "BIO", From: start, To: start.AddDate(0, 0, 2), MinNights: 1, MaxNights: 3, Currency: "EUR"}

	all, err := CheapestTrips(outbound, inbound, search)
	if err != nil || len(all) != 5 {
		t.Fatalf("CheapestTrips unfiltered = %d trips, %v; want five", len(all), err)
	}
	if all[0].Total != 11 || all[0].Departure != date(1) || all[0].Return != date(2) || all[1].Total != 11 || all[1].Departure != date(1) || all[1].Return != date(3) || all[2].Departure != date(2) {
		t.Fatalf("CheapestTrips order = %+v, want stable ordering for tied fares", all[:3])
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].Total > all[i].Total {
			t.Fatalf("trips are not sorted by total: %+v", all)
		}
	}

	search.MaxPrice, search.Limit = 12, 2
	limited, err := CheapestTrips(outbound, inbound, search)
	if err != nil || len(limited) != 2 || limited[0].Total != 11 || limited[1].Total != 11 {
		t.Fatalf("CheapestTrips filtered = %+v, %v; want two qualifying tied fares", limited, err)
	}
	if !strings.Contains(limited[0].URL, "tickets.vueling.com/booking") || limited[0].Currency != "EUR" {
		t.Fatalf("trip selection details = %+v", limited[0])
	}
}

func TestCheapestTripsReturnsNoMatchesForMissingFares(t *testing.T) {
	date := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	trips, err := CheapestTrips(
		[]CalendarDay{{Date: date.Format("02/01/2006"), Fare: "10 €"}},
		nil,
		TripSearch{Origin: "BCN", Destination: "BIO", From: date, To: date, MinNights: 1, MaxNights: 1, Currency: "EUR"},
	)
	if err != nil || len(trips) != 0 {
		t.Fatalf("CheapestTrips missing return = %+v, %v; want no trips", trips, err)
	}
}
