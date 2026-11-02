package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/seifreed/vuelingcli/internal/airports"
	"github.com/seifreed/vuelingcli/internal/vueling"
)

// DealReader retrieves Vueling's fare offers for a set of destinations.
type DealReader interface {
	Deals(context.Context, string, []string, time.Time, int, string) ([]vueling.Deal, error)
}

// DealSearch contains the inputs that determine a deal query and its results.
type DealSearch struct {
	Origin       string
	Destinations []string
	From         time.Time
	Months       int
	MaxPrice     float64
	Limit        int
	Direct       bool
	Currency     string
}

// DealResult combines a fare offer with its parsed departure date for text output.
type DealResult struct {
	vueling.Deal
	Departure time.Time `json:"-"`
}

// FindDeals loads offers, applies the requested filters, and enriches their city and calendar link.
func FindDeals(ctx context.Context, reader DealReader, search DealSearch) ([]DealResult, error) {
	deals, err := reader.Deals(ctx, search.Origin, search.Destinations, search.From, search.Months, search.Currency)
	if err != nil {
		return nil, err
	}
	deals = vueling.FilterDeals(deals, vueling.DealFilter{Direct: search.Direct, MaxPrice: search.MaxPrice, Limit: search.Limit})
	enriched := make([]DealResult, 0, len(deals))
	for _, deal := range deals {
		city, ok := airports.City(deal.Destination)
		if !ok {
			city = deal.Destination
		}
		deal.City = city
		departure, err := time.Parse(time.DateOnly, deal.Date)
		if err != nil {
			return nil, fmt.Errorf("Vueling returned invalid deal date %q: %w", deal.Date, err)
		}
		deal.CalendarURL, err = vueling.CalendarURL(search.Origin, deal.Destination, departure)
		if err != nil {
			return nil, fmt.Errorf("build calendar URL for %s: %w", deal.Destination, err)
		}
		enriched = append(enriched, DealResult{Deal: deal, Departure: departure})
	}
	return enriched, nil
}
