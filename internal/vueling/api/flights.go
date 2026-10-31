package api

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

// anonymousProfileID is the public profile tickets.vueling.com uses to request an anonymous session token.
const anonymousProfileID = "e8ffa738-cb67-4a02-b501-9bfd975a4b65"

const availabilityQuery = "query GetAvy($requestAVY:AvailabilityRequestGraphQLInput!){ amsAvy (amsAvailabilityRequest : $requestAVY) { currencyCode, trips { trips { journeysAvailableByMarket { key, value{ journeyKey, segments { identifier { carrierCode, identifier }, designator{ arrival, departure, destination, origin }, legs { legInfo { operatedByText, arrivalTimeUtc, departureTimeUtc } } }, designator{ arrival, departure, destination, origin }, fares{ fareAvailabilityKey, details { availableCount } }, flightType, duration, connectionTime } } } }, faresAvailable { value { fares { productClass, classOfService, fareBasisCode, passengerFares { amsFareAmount } }, fareAvailabilityKey } } } }"

// FlightQuery describes a one-way or return flight search. Infants travel on an adult's lap,
// so Vueling's availability search has no passenger type for them.
type FlightQuery struct {
	Origin, Destination string
	Date, ReturnDate    time.Time
	Adults, Children    int
	Currency            string
}

// Flights returns every bookable Vueling journey for the outbound date and, when set, the return date.
// Each fare is the cheapest price for one adult; Vueling prices every passenger type per person.
func (c *Client) Flights(ctx context.Context, query FlightQuery) ([]vueling.Flight, error) {
	token, err := c.anonymousToken(ctx)
	if err != nil {
		return nil, err
	}
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

func (c *Client) anonymousToken(ctx context.Context) (string, error) {
	var session struct {
		AccessToken string `json:"accessToken"`
	}
	if err := c.postJSON(ctx, c.AMS+"/asm/v1/Auth", ticketsOrigin, "", map[string]string{"profileId": anonymousProfileID}, &session); err != nil {
		return "", fmt.Errorf("open an anonymous Vueling session: %w", err)
	}
	if session.AccessToken == "" {
		return "", errors.New("Vueling did not return an anonymous session token")
	}
	return session.AccessToken, nil
}

type availabilityResponse struct {
	Data struct {
		AMSAvy *struct {
			CurrencyCode string `json:"currencyCode"`
			Trips        []struct {
				Trips []struct {
					JourneysAvailableByMarket []struct {
						Value []journey `json:"value"`
					} `json:"journeysAvailableByMarket"`
				} `json:"trips"`
			} `json:"trips"`
			FaresAvailable []struct {
				Value struct {
					FareAvailabilityKey string `json:"fareAvailabilityKey"`
					Fares               []struct {
						PassengerFares []struct {
							AMSFareAmount float64 `json:"amsFareAmount"`
						} `json:"passengerFares"`
					} `json:"fares"`
				} `json:"value"`
			} `json:"faresAvailable"`
		} `json:"amsAvy"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type designator struct {
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
	Departure   string `json:"departure"`
	Arrival     string `json:"arrival"`
}

type flightNumber struct {
	CarrierCode string `json:"carrierCode"`
	Identifier  string `json:"identifier"`
}

type leg struct {
	LegInfo struct {
		OperatedByText *string `json:"operatedByText"`
	} `json:"legInfo"`
}

type segment struct {
	Identifier flightNumber `json:"identifier"`
	Designator designator   `json:"designator"`
	Legs       []leg        `json:"legs"`
}

type fareReference struct {
	FareAvailabilityKey string `json:"fareAvailabilityKey"`
}

type journey struct {
	Designator designator      `json:"designator"`
	Segments   []segment       `json:"segments"`
	Fares      []fareReference `json:"fares"`
}

func (c *Client) oneWayFlights(ctx context.Context, token string, query FlightQuery, direction, origin, destination string, date time.Time) ([]vueling.Flight, error) {
	var response availabilityResponse
	if err := c.postJSON(ctx, c.AMS+"/avy/v1/graphql", ticketsOrigin, token, availabilityRequest(query, origin, destination, date), &response); err != nil {
		return nil, fmt.Errorf("read Vueling %s flights: %w", direction, err)
	}
	avy := response.Data.AMSAvy
	if avy == nil {
		return nil, availabilityError(response)
	}
	prices := map[string]float64{}
	for _, entry := range avy.FaresAvailable {
		for _, fare := range entry.Value.Fares {
			if len(fare.PassengerFares) > 0 {
				prices[entry.Value.FareAvailabilityKey] = fare.PassengerFares[0].AMSFareAmount
			}
		}
	}
	currency := avy.CurrencyCode
	if currency == "" {
		currency = query.Currency
	}
	day := date.Format(time.DateOnly)
	var flights []vueling.Flight
	for _, trip := range avy.Trips {
		for _, inner := range trip.Trips {
			for _, market := range inner.JourneysAvailableByMarket {
				for _, j := range market.Value {
					if flight, ok := journeyFlight(j, prices, direction, day, currency); ok {
						flights = append(flights, flight)
					}
				}
			}
		}
	}
	return flights, nil
}

func availabilityRequest(query FlightQuery, origin, destination string, date time.Time) map[string]any {
	passengers := []map[string]any{{"count": query.Adults, "type": "Adult"}}
	if query.Children > 0 {
		passengers = append(passengers, map[string]any{"count": query.Children, "type": "Child"})
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
					"servicesToRequest":   []string{},
					"bundleControlFilter": 2,
				},
				"trackingPoint": "BBV",
			},
		},
	}
}

func availabilityError(response availabilityResponse) error {
	var messages []string
	for _, e := range response.Errors {
		if e.Message != "" {
			messages = append(messages, e.Message)
		}
	}
	if len(messages) == 0 {
		return errors.New("Vueling returned no availability data")
	}
	return fmt.Errorf("Vueling availability: %s", strings.Join(messages, "; "))
}

// journeyFlight maps a journey to a flight priced at its cheapest fare; journeys without a price are sold out.
func journeyFlight(j journey, prices map[string]float64, direction, day, currency string) (vueling.Flight, bool) {
	fare := math.Inf(1)
	for _, f := range j.Fares {
		if price, ok := prices[f.FareAvailabilityKey]; ok && price < fare {
			fare = price
		}
	}
	if math.IsInf(fare, 1) || len(j.Segments) == 0 {
		return vueling.Flight{}, false
	}
	var numbers, via, operators []string
	legs := 0
	for i, segment := range j.Segments {
		numbers = append(numbers, segment.Identifier.CarrierCode+segment.Identifier.Identifier)
		if i < len(j.Segments)-1 {
			via = append(via, segment.Designator.Destination)
		}
		legs += max(len(segment.Legs), 1)
		for _, leg := range segment.Legs {
			if text := leg.LegInfo.OperatedByText; text != nil && *text != "" {
				operators = append(operators, *text)
			}
		}
	}
	departure, arrival := localClock(j.Designator.Departure), localClock(j.Designator.Arrival)
	itinerary := fmt.Sprintf("%s %s → %s %s %s %s", departure, j.Designator.Origin, arrival, j.Designator.Destination, strings.Join(numbers, "+"), routeKind(via, legs))
	if len(operators) > 0 {
		itinerary += " · " + strings.Join(operators, ", ")
	}
	return vueling.Flight{
		Direction: direction,
		Date:      day,
		Flight:    numbers[0],
		Departure: departure,
		Arrival:   arrival,
		Fare:      strconv.FormatFloat(fare, 'f', 2, 64),
		Currency:  currency,
		Itinerary: itinerary,
		Stops:     max(legs-1, len(via)),
		Flights:   numbers,
		Via:       via,
	}, true
}

func routeKind(via []string, legs int) string {
	switch {
	case len(via) > 0:
		return "Conexión en " + strings.Join(via, ", ")
	case legs > 1:
		return "Con escala"
	default:
		return "Directo"
	}
}

// localClock reads HH:MM from Vueling's local timestamps, which carry a misleading Z suffix.
func localClock(timestamp string) string {
	if t, err := time.Parse("2006-01-02T15:04:05", strings.TrimSuffix(strings.SplitN(timestamp, ".", 2)[0], "Z")); err == nil {
		return t.Format("15:04")
	}
	return timestamp
}
