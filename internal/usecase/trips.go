// Package usecase coordinates domain operations with ports used by CLI commands.
package usecase

import (
	"context"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

// CalendarReader provides one-way fare calendars for a route and currency.
type CalendarReader interface {
	Calendar(context.Context, string, string, string) ([]vueling.CalendarDay, error)
}

// FindTrips loads calendars in both directions and ranks the matching trip dates.
func FindTrips(ctx context.Context, calendars CalendarReader, search vueling.TripSearch) ([]vueling.Trip, error) {
	outbound, err := calendars.Calendar(ctx, search.Origin, search.Destination, search.Currency)
	if err != nil {
		return nil, err
	}
	inbound, err := calendars.Calendar(ctx, search.Destination, search.Origin, search.Currency)
	if err != nil {
		return nil, err
	}
	return vueling.CheapestTrips(outbound, inbound, search)
}
