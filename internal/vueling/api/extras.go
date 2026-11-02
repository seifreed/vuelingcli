package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

// DefaultExtrasBundle is the bundle Extras prices with unless asked otherwise: Fly Light, which
// includes no seat or checked bag, so every extra shows its full price.
const DefaultExtrasBundle = vueling.DefaultExtrasBundle

// endSessionTimeout bounds the call that ends an extras session, which runs even after the caller's context ends.
const endSessionTimeout = 15 * time.Second

// Extras reads the seat, bag and service prices of one flight. Vueling only prices extras for a flight in a
// cart, so Extras opens its own anonymous session, puts the flight's cheapest fare with bundle into that
// session's cart, reads the seat map and the service list, and ends the session before returning, even on
// error. It never adds passengers, contact details, seats or services, and never commits or pays.
func (c *Client) Extras(ctx context.Context, query FlightQuery, pick Pick, bundle string) (extras vueling.Extras, err error) {
	session, err := c.openSession(ctx)
	if err != nil {
		return vueling.Extras{}, err
	}
	defer func() {
		if endErr := c.endSession(context.WithoutCancel(ctx), session.Token); endErr != nil {
			err = errors.Join(err, endErr)
		}
	}()
	return c.extras(ctx, session.Token, query, pick, bundle)
}

func (c *Client) extras(ctx context.Context, token string, query FlightQuery, pick Pick, bundle string) (vueling.Extras, error) {
	avy, err := c.availability(ctx, token, query, "outbound", query.Origin, query.Destination, query.Date)
	if err != nil {
		return vueling.Extras{}, err
	}
	prices, err := avy.farePrices()
	if err != nil {
		return vueling.Extras{}, err
	}
	j, fare, ok := pickFare(avy.journeys(), prices, pick)
	if !ok {
		return vueling.Extras{}, fmt.Errorf("Vueling shows no bookable %s from %s to %s on %s", pickDescription(pick), query.Origin, query.Destination, query.Date.Format(time.DateOnly))
	}
	flight := strings.Join(journeyNumbers(j), "+")
	if offered := offeredBundles(avy, fare); !slices.Contains(offered, bundle) {
		return vueling.Extras{}, fmt.Errorf("Vueling does not offer bundle %s for %s; it offers %s", bundle, flight, strings.Join(offered, ", "))
	}
	if err := c.sellTrip(ctx, token, j.JourneyKey, fare.FareSellKey, bundle); err != nil {
		return vueling.Extras{}, err
	}
	var seatMap seatMapResponse
	if err := c.do(ctx, http.MethodGet, c.AMS+"/bst/v2/SmartSeatMap", ticketsOrigin, token, nil, &seatMap); err != nil {
		return vueling.Extras{}, fmt.Errorf("read Vueling's seat map: %w", err)
	}
	var services servicesResponse
	if err := c.do(ctx, http.MethodGet, c.AMS+"/bpv/v3/BookingServices/Services/BPV", ticketsOrigin, token, nil, &services); err != nil {
		return vueling.Extras{}, fmt.Errorf("read Vueling's extra services: %w", err)
	}
	currency := services.CurrencyCode
	if currency == "" {
		currency = avy.CurrencyCode
	}
	seats, err := seatMap.seatPrices(j, currency)
	if err != nil {
		return vueling.Extras{}, err
	}
	extras := vueling.Extras{
		Flight: flight, Date: query.Date.Format(time.DateOnly), Origin: j.Designator.Origin, Destination: j.Designator.Destination,
		Bundle: bundle, Currency: currency, Seats: seats,
	}
	extras.Bags, extras.Services, err = services.prices(currency)
	if err != nil {
		return vueling.Extras{}, err
	}
	return extras, nil
}

// sellTrip puts one journey's fare and bundle into the session's cart the way the website does when its
// Continue button is clicked; Vueling then prices seats and services for that journey.
func (c *Client) sellTrip(ctx context.Context, token, journeyKey, fareSellKey, bundle string) error {
	body := map[string]any{
		"journeyKeys":          []map[string]string{{"fareSellKey": fareSellKey}},
		"bundleCodesByJourney": map[string]string{journeyKey: bundle},
		"extraSeats":           0,
	}
	var sold struct {
		Success bool `json:"success"`
	}
	if err := c.postJSON(ctx, c.AMS+"/bkj/v2/Sell/selltrip/BSV", ticketsOrigin, token, body, &sold); err != nil {
		return fmt.Errorf("put the flight in the session's cart: %w", err)
	}
	if !sold.Success {
		return errors.New("Vueling did not put the flight in the session's cart")
	}
	return nil
}

// endSession ends a session on Vueling's side, which drops its cart.
func (c *Client) endSession(ctx context.Context, token string) error {
	ctx, cancel := context.WithTimeout(ctx, endSessionTimeout)
	defer cancel()
	if err := c.do(ctx, http.MethodDelete, c.AMS+"/asm/v1/Auth", ticketsOrigin, token, nil, new([]byte)); err != nil {
		return fmt.Errorf("end the Vueling session (it expires by itself within 20 minutes): %w", err)
	}
	return nil
}
