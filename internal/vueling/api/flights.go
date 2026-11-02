package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/usecase"
	"github.com/seifreed/vuelingcli/internal/vueling"
)

// anonymousProfileID is the public profile tickets.vueling.com uses to request an anonymous session token.
const anonymousProfileID = "e8ffa738-cb67-4a02-b501-9bfd975a4b65"

const availabilityQuery = "query GetAvy($requestAVY:AvailabilityRequestGraphQLInput!){ amsAvy (amsAvailabilityRequest : $requestAVY) { currencyCode, trips { trips { journeysAvailableByMarket { key, value{ journeyKey, segments { identifier { carrierCode, identifier }, designator{ arrival, departure, destination, origin }, legs { legInfo { operatedByText, arrivalTimeUtc, departureTimeUtc } } }, designator{ arrival, departure, destination, origin }, fares{ fareAvailabilityKey, fareSellKey, details { bundleReferences, availableCount } }, flightType, duration, connectionTime } } } }, faresAvailable { value { fares { productClass, classOfService, fareBasisCode, passengerFares { amsFareAmount } }, fareAvailabilityKey } }, bundleOffers { key, value { bundleCode, bundlePrices { totalPrice, passengerType } } } } }"

// FlightQuery aliases the application-level query type.
type FlightQuery = usecase.FlightQuery

// discountServices are the availability services the website requests for each dt discount.
var discountServices = map[int][]string{
	1: {"RESO"},
	2: {"FAMI"},
	3: {"FAMIESP"},
	4: {"FAMIRESO", "FAMRES"},
	5: {"FAMIRESOESP", "FAMRESESP"},
}

// Flights returns every bookable Vueling journey for the outbound date and, when set, the return date.
// Each fare is the cheapest price for one adult; Vueling prices every passenger type per person.
func (c *Client) Flights(ctx context.Context, query FlightQuery) ([]vueling.Flight, error) {
	return withSession(ctx, c, func(token string) ([]vueling.Flight, error) { return c.flights(ctx, token, query) })
}

func (c *Client) flights(ctx context.Context, token string, query FlightQuery) ([]vueling.Flight, error) {
	flights, err := c.oneWayFlights(ctx, token, query, "outbound", query.Origin, query.Destination, query.Date)
	if err != nil {
		return nil, err
	}
	if query.ReturnDate.IsZero() {
		return flights, nil
	}
	returns, err := c.oneWayFlights(ctx, token, query, "return", query.Destination, query.Origin, query.ReturnDate)
	if err != nil {
		return nil, err
	}
	return append(flights, returns...), nil
}

func (c *Client) oneWayFlights(ctx context.Context, token string, query FlightQuery, direction, origin, destination string, date time.Time) ([]vueling.Flight, error) {
	avy, err := c.availability(ctx, token, query, direction, origin, destination, date)
	if err != nil {
		return nil, err
	}
	prices, err := avy.farePrices()
	if err != nil {
		return nil, err
	}
	currency := avy.CurrencyCode
	if currency == "" {
		currency = query.Currency
	}
	day := date.Format(time.DateOnly)
	var flights []vueling.Flight
	for _, j := range avy.journeys() {
		flight, ok := journeyFlight(j, prices, direction, day, currency)
		if !ok {
			continue
		}
		if !strings.EqualFold(j.Designator.Origin, origin) || !strings.EqualFold(j.Designator.Destination, destination) {
			return nil, fmt.Errorf("Vueling returned a flight from %s to %s for requested route %s to %s", j.Designator.Origin, j.Designator.Destination, origin, destination)
		}
		for _, segment := range j.Segments {
			if segment.Identifier.CarrierCode == "" || segment.Identifier.Identifier == "" {
				return nil, fmt.Errorf("Vueling returned an incomplete flight number for %s to %s on %s", origin, destination, day)
			}
		}
		flights = append(flights, flight)
	}
	return flights, nil
}

func (c *Client) availability(ctx context.Context, token string, query FlightQuery, direction, origin, destination string, date time.Time) (*amsAvailability, error) {
	var response journeyAvailabilityResponse
	if err := c.postJSON(ctx, c.AMS+"/avy/v1/graphql", ticketsOrigin, token, availabilityRequest(query, origin, destination, date), &response); err != nil {
		return nil, fmt.Errorf("read Vueling %s flights: %w", direction, err)
	}
	if response.Data.AMSAvy == nil || len(response.Errors) > 0 {
		return nil, availabilityError(response)
	}
	return response.Data.AMSAvy, nil
}

func availabilityRequest(query FlightQuery, origin, destination string, date time.Time) map[string]any {
	passengers := []map[string]any{{"count": query.Adults, "type": "Adult"}}
	if query.Children > 0 {
		passengers = append(passengers, map[string]any{"count": query.Children, "type": "Child"})
	}
	services := []map[string]any{}
	if query.Infants > 0 {
		services = append(services, map[string]any{"count": query.Infants, "serviceCode": "INFANT"})
	}
	for _, code := range discountServices[query.Discount] {
		services = append(services, map[string]any{"count": 1, "serviceCode": code})
	}
	return map[string]any{
		"operationName": "GetAvy",
		"query":         availabilityQuery,
		"variables": map[string]any{
			"requestAVY": map[string]any{
				"request": map[string]any{
					"criteria":            []map[string]string{{"origin": origin, "destination": destination, "date": date.Format(time.DateOnly) + "T00:00:00.000Z"}},
					"cultureCode":         "es-ES",
					"currencyCode":        query.Currency,
					"flightType":          "ALL",
					"itineraryType":       "ONE_WAY",
					"maxConnections":      10,
					"passengers":          passengers,
					"serviceCode":         "Search1Day",
					"servicesToRequest":   services,
					"bundleControlFilter": 2,
				},
				"trackingPoint": "BBV",
			},
		},
	}
}
