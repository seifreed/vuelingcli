package cli

import (
	"context"
	"errors"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
	"github.com/seifreed/vuelingcli/internal/vueling/api"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

// readFlights reads offers from Vueling's availability API, or from the Chrome page with --browser.
func (cli *CLI) readFlights(reader web.PageReader, searchURL string, options flightOptions) ([]vueling.Flight, error) {
	if options.browser || cli.api == nil {
		return web.FlightsFromPage(context.Background(), reader, searchURL, options.date.Format(time.DateOnly), options.returnDate.Format(time.DateOnly))
	}
	flights, err := cli.api.Flights(context.Background(), api.FlightQuery{
		Origin: options.origin, Destination: options.destination,
		Date: options.date, ReturnDate: options.returnDate,
		Adults: options.adults, Children: options.children, Currency: options.currency,
	})
	if err != nil {
		return nil, err
	}
	return flights, requireBothDirections(flights, !options.returnDate.IsZero())
}

func requireBothDirections(flights []vueling.Flight, wantReturn bool) error {
	outbound, inbound := false, false
	for _, flight := range flights {
		outbound = outbound || flight.Direction == "outbound"
		inbound = inbound || flight.Direction == "return"
	}
	if !outbound {
		return errors.New("Vueling shows no flights for that route on the requested date")
	}
	if wantReturn && !inbound {
		return errors.New("Vueling shows no return flights for that route on the requested return date")
	}
	return nil
}
