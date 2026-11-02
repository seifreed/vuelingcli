package api

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

type allFlightsRequest struct {
	OriginCode      string `json:"originCode"`
	DestinationCode string `json:"destinationCode"`
	Year            int    `json:"year"`
	Month           int    `json:"month"`
	CurrencyCode    string `json:"currencyCode"`
	MonthsRange     int    `json:"monthsRange"`
	FlightType      string `json:"flightType"`
}

type allFlightsDay struct {
	DepartureDate      string  `json:"departureDate"`
	FlightID           string  `json:"flightID"`
	CarrierCode        string  `json:"carrierCode"`
	Price              float64 `json:"price"`
	Currency           string  `json:"currency"`
	IsConnectionFlight bool    `json:"isConnectionFlight"`
	IsInvalidPrice     bool    `json:"isInvalidPrice"`
	IsAvailableDay     bool    `json:"isAvailableDay"`
}

// Schedule returns each day Vueling sells a one-way flight on a route from the start of from's
// month through the given number of months, with that day's cheapest flight. It reads the
// availability the website's date picker uses, which knows each route about eleven months ahead.
func (c *Client) Schedule(ctx context.Context, origin, destination, currency string, from time.Time, months int) ([]vueling.ScheduledDay, error) {
	request := allFlightsRequest{
		OriginCode: origin, DestinationCode: destination,
		Year: from.Year(), Month: int(from.Month()), CurrencyCode: currency,
		MonthsRange: months, FlightType: "OW",
	}
	return withSession(ctx, c, func(token string) ([]vueling.ScheduledDay, error) {
		var body []byte
		if err := c.postJSON(ctx, c.AMS+"/avy/v3/AvailabilityServices/allFlights", ticketsOrigin, token, request, &body); err != nil {
			return nil, err
		}
		return scheduledDays(body)
	})
}

// scheduledDays keeps the days Vueling marks as available; it answers an empty body for a route it does not fly.
func scheduledDays(body []byte) ([]vueling.ScheduledDay, error) {
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil, nil
	}
	var rows []allFlightsDay
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("decode Vueling's flight days: %w", err)
	}
	days := map[string]vueling.ScheduledDay{}
	prices, priced := map[string]float64{}, map[string]bool{}
	for i, row := range rows {
		if !row.IsAvailableDay {
			continue
		}
		if !row.IsInvalidPrice && row.Price < 0 {
			return nil, fmt.Errorf("Vueling returned a negative price for available flight day %d", i+1)
		}
		departure, err := parseAPITime(row.DepartureDate)
		if err != nil {
			return nil, fmt.Errorf("decode Vueling flight day %d departure date %q: %w", i+1, row.DepartureDate, err)
		}
		date := departure.Format(time.DateOnly)
		hasPrice := !row.IsInvalidPrice && row.Price > 0
		if _, duplicate := days[date]; duplicate && (!hasPrice || priced[date] && row.Price >= prices[date]) {
			continue
		}
		day := vueling.ScheduledDay{Date: date, Departure: departure.Format("15:04"), Connection: row.IsConnectionFlight}
		if row.FlightID != "" {
			day.Flight = row.CarrierCode + row.FlightID
		}
		if hasPrice {
			day.Fare = vueling.FormatFare(row.Price, row.Currency)
		}
		days[date] = day
		prices[date], priced[date] = row.Price, hasPrice
	}
	sorted := make([]vueling.ScheduledDay, 0, len(days))
	for _, day := range days {
		sorted = append(sorted, day)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Date < sorted[j].Date })
	return sorted, nil
}
