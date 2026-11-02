package usecase

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

type dealReaderFunc func(context.Context, string, []string, time.Time, int, string) ([]vueling.Deal, error)

func (read dealReaderFunc) Deals(ctx context.Context, origin string, destinations []string, from time.Time, months int, currency string) ([]vueling.Deal, error) {
	return read(ctx, origin, destinations, from, months, currency)
}

func TestFindDealsReadsFiltersAndEnrichesOffers(t *testing.T) {
	from := time.Date(2026, time.December, 1, 0, 0, 0, 0, time.UTC)
	destinations := []string{"BIO", "MAD", "PMI"}
	var gotQuery DealSearch
	read := dealReaderFunc(func(_ context.Context, origin string, gotDestinations []string, gotFrom time.Time, months int, currency string) ([]vueling.Deal, error) {
		gotQuery = DealSearch{Origin: origin, Destinations: gotDestinations, From: gotFrom, Months: months, Currency: currency}
		return []vueling.Deal{
			{Destination: "MAD", Date: "2026-12-02", Price: 80},
			{Destination: "BIO", Date: "2026-12-03", Price: 100, Direct: true},
			{Destination: "PMI", Date: "2026-12-04", Price: 200, Direct: true},
		}, nil
	})
	search := DealSearch{Origin: "BCN", Destinations: destinations, From: from, Months: 3, MaxPrice: 100, Limit: 1, Direct: true, Currency: "EUR"}

	deals, err := FindDeals(context.Background(), read, search)
	if err != nil || len(deals) != 1 {
		t.Fatalf("FindDeals = %+v, %v; want one deal", deals, err)
	}
	if deals[0].Destination != "BIO" || deals[0].City == "" || deals[0].CalendarURL == "" || deals[0].Departure.Format(time.DateOnly) != "2026-12-03" {
		t.Fatalf("deal was not enriched: %+v", deals[0])
	}
	if gotQuery.Origin != "BCN" || !reflect.DeepEqual(gotQuery.Destinations, destinations) || !gotQuery.From.Equal(from) || gotQuery.Months != 3 || gotQuery.Currency != "EUR" {
		t.Fatalf("query = %+v", gotQuery)
	}
}

func TestFindDealsUsesDestinationCodeWhenCityIsUnknown(t *testing.T) {
	read := dealReaderFunc(func(context.Context, string, []string, time.Time, int, string) ([]vueling.Deal, error) {
		return []vueling.Deal{{Destination: "ZZZ", Date: "2026-12-03"}}, nil
	})
	deals, err := FindDeals(context.Background(), read, DealSearch{Origin: "BCN"})
	if err != nil || len(deals) != 1 || deals[0].City != "ZZZ" {
		t.Fatalf("FindDeals unknown destination = %+v, %v", deals, err)
	}
}

func TestFindDealsPropagatesReaderAndEnrichmentErrors(t *testing.T) {
	readerErr := errors.New("deals unavailable")
	if _, err := FindDeals(context.Background(), dealReaderFunc(func(context.Context, string, []string, time.Time, int, string) ([]vueling.Deal, error) {
		return nil, readerErr
	}), DealSearch{}); !errors.Is(err, readerErr) {
		t.Fatalf("reader error = %v", err)
	}
	for _, tc := range []struct {
		name, destination, date, want string
	}{
		{"invalid date", "BIO", "tomorrow", "invalid deal date"},
		{"invalid calendar route", "BCN", "2026-12-03", "build calendar URL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			read := dealReaderFunc(func(context.Context, string, []string, time.Time, int, string) ([]vueling.Deal, error) {
				return []vueling.Deal{{Destination: tc.destination, Date: tc.date}}, nil
			})
			if _, err := FindDeals(context.Background(), read, DealSearch{Origin: "BCN"}); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("FindDeals error = %v; want %q", err, tc.want)
			}
		})
	}
}
