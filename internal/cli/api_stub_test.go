package cli

import (
	"context"
	"time"

	"github.com/seifreed/vuelingcli/internal/usecase"
	"github.com/seifreed/vuelingcli/internal/vueling"
)

type stubAPI struct {
	alertFn        func(context.Context, string) (vueling.Alert, error)
	calendarFn     func(context.Context, string, string, string) ([]vueling.CalendarDay, error)
	dealsFn        func(context.Context, string, []string, time.Time, int, string) ([]vueling.Deal, error)
	extrasFn       func(context.Context, usecase.FlightQuery, usecase.FlightPick, string) (vueling.Extras, error)
	feesFn         func(context.Context) ([]vueling.Fee, error)
	flightsFn      func(context.Context, usecase.FlightQuery) ([]vueling.Flight, error)
	statusFn       func(context.Context, string, string, string, time.Time) ([]vueling.FlightStatus, error)
	quoteTripFn    func(context.Context, usecase.FlightQuery, usecase.FlightPick, usecase.FlightPick) ([]vueling.Quote, error)
	routesFn       func(context.Context, string) ([]vueling.Route, error)
	scheduleFn     func(context.Context, string, string, string, time.Time, int) ([]vueling.ScheduledDay, error)
	storedFn       func() (vueling.Session, error)
	refreshFn      func(context.Context) (vueling.Session, error)
	clearSessionFn func(context.Context) error
}

func (api *stubAPI) Alert(ctx context.Context, origin string) (vueling.Alert, error) {
	if api.alertFn != nil {
		return api.alertFn(ctx, origin)
	}
	return vueling.Alert{}, nil
}
func (api *stubAPI) Calendar(ctx context.Context, origin, destination, currency string) ([]vueling.CalendarDay, error) {
	if api.calendarFn != nil {
		return api.calendarFn(ctx, origin, destination, currency)
	}
	return nil, nil
}
func (api *stubAPI) Deals(ctx context.Context, origin string, destinations []string, from time.Time, months int, currency string) ([]vueling.Deal, error) {
	if api.dealsFn != nil {
		return api.dealsFn(ctx, origin, destinations, from, months, currency)
	}
	return nil, nil
}
func (api *stubAPI) Extras(ctx context.Context, query usecase.FlightQuery, pick usecase.FlightPick, bundle string) (vueling.Extras, error) {
	if api.extrasFn != nil {
		return api.extrasFn(ctx, query, pick, bundle)
	}
	return vueling.Extras{}, nil
}
func (api *stubAPI) Fees(ctx context.Context) ([]vueling.Fee, error) {
	if api.feesFn != nil {
		return api.feesFn(ctx)
	}
	return nil, nil
}
func (api *stubAPI) Flights(ctx context.Context, query usecase.FlightQuery) ([]vueling.Flight, error) {
	if api.flightsFn != nil {
		return api.flightsFn(ctx, query)
	}
	return nil, nil
}
func (api *stubAPI) FlightStatus(ctx context.Context, flight, origin, destination string, date time.Time) ([]vueling.FlightStatus, error) {
	if api.statusFn != nil {
		return api.statusFn(ctx, flight, origin, destination, date)
	}
	return nil, nil
}
func (api *stubAPI) QuoteTrip(ctx context.Context, query usecase.FlightQuery, outbound, inbound usecase.FlightPick) ([]vueling.Quote, error) {
	if api.quoteTripFn != nil {
		return api.quoteTripFn(ctx, query, outbound, inbound)
	}
	return nil, nil
}
func (api *stubAPI) Routes(ctx context.Context, origin string) ([]vueling.Route, error) {
	if api.routesFn != nil {
		return api.routesFn(ctx, origin)
	}
	return nil, nil
}
func (api *stubAPI) Schedule(ctx context.Context, origin, destination, currency string, date time.Time, days int) ([]vueling.ScheduledDay, error) {
	if api.scheduleFn != nil {
		return api.scheduleFn(ctx, origin, destination, currency, date, days)
	}
	return nil, nil
}
func (api *stubAPI) StoredSession() (vueling.Session, error) {
	if api.storedFn != nil {
		return api.storedFn()
	}
	return vueling.Session{}, nil
}
func (api *stubAPI) RefreshSession(ctx context.Context) (vueling.Session, error) {
	if api.refreshFn != nil {
		return api.refreshFn(ctx)
	}
	return vueling.Session{}, nil
}
func (api *stubAPI) ClearSession(ctx context.Context) error {
	if api.clearSessionFn != nil {
		return api.clearSessionFn(ctx)
	}
	return nil
}
