package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/usecase"
	"github.com/seifreed/vuelingcli/internal/vueling"
)

// Quote prices one outbound flight the way the website does when its fare is clicked: it reads the
// fare breakdown and bundle prices of the flight's cheapest fare without selling or booking anything.
// flight is the first flight number, such as VY1460, or every number of a connection, such as VY1383+VY1050.
func (c *Client) Quote(ctx context.Context, query FlightQuery, flight string) (vueling.Quote, error) {
	return withSession(ctx, c, func(token string) (vueling.Quote, error) {
		return c.quote(ctx, token, query, "outbound", Pick{Flight: flight})
	})
}

// Pick aliases the application-level flight selection type.
type Pick = usecase.FlightPick

// QuoteTrip prices the outbound pick and, when query has a return date, the return pick. It reads each
// direction's breakdown separately: Vueling rejects fare sell keys from two one-way searches in one request.
func (c *Client) QuoteTrip(ctx context.Context, query FlightQuery, outbound, inbound Pick) ([]vueling.Quote, error) {
	return withSession(ctx, c, func(token string) ([]vueling.Quote, error) {
		quote, err := c.quote(ctx, token, query, "outbound", outbound)
		if err != nil || query.ReturnDate.IsZero() {
			return []vueling.Quote{quote}, err
		}
		returnQuote, err := c.quote(ctx, token, query, "return", inbound)
		return []vueling.Quote{quote, returnQuote}, err
	})
}

func (c *Client) quote(ctx context.Context, token string, query FlightQuery, direction string, pick Pick) (vueling.Quote, error) {
	origin, destination, date := query.Origin, query.Destination, query.Date
	if direction == "return" {
		origin, destination, date = query.Destination, query.Origin, query.ReturnDate
	}
	avy, err := c.availability(ctx, token, query, direction, origin, destination, date)
	if err != nil {
		return vueling.Quote{}, err
	}
	prices, err := avy.farePrices()
	if err != nil {
		return vueling.Quote{}, err
	}
	j, fare, ok := pickFare(avy.journeys(), prices, pick)
	if !ok {
		return vueling.Quote{}, fmt.Errorf("Vueling shows no bookable %s from %s to %s on %s", pickDescription(pick), origin, destination, date.Format(time.DateOnly))
	}
	flight := strings.Join(journeyNumbers(j), "+")
	var breakdown breakdownResponse
	if err := c.postJSON(ctx, c.AMS+"/bkd/v4/breakdown/Availability", ticketsOrigin, token, map[string][]string{"fareSellKeyList": {fare.FareSellKey}}, &breakdown); err != nil {
		return vueling.Quote{}, fmt.Errorf("read Vueling's price breakdown: %w", err)
	}
	if len(breakdown.Flights) == 0 || len(breakdown.Flights[0].FlightInfo) == 0 {
		return vueling.Quote{}, fmt.Errorf("Vueling returned no price breakdown for %s", flight)
	}
	passengers, err := breakdownPassengers(breakdown)
	if err != nil {
		return vueling.Quote{}, err
	}
	if passengers["ADT"] != query.Adults || passengers["CHD"] != query.Children {
		return vueling.Quote{}, fmt.Errorf("Vueling returned adults=%d and children=%d; requested adults=%d and children=%d", passengers["ADT"], passengers["CHD"], query.Adults, query.Children)
	}
	quote, err := breakdownQuote(breakdown, journeyNumbers(j), date)
	if err != nil {
		return vueling.Quote{}, err
	}
	if quote.Infants != query.Infants {
		return vueling.Quote{}, fmt.Errorf("Vueling returned infants=%d; requested infants=%d", quote.Infants, query.Infants)
	}
	quote.Bundles, err = bundlePrices(avy, fare, passengers, quote.Total)
	if err != nil {
		return vueling.Quote{}, fmt.Errorf("price Vueling bundles: %w", err)
	}
	return quote, nil
}
