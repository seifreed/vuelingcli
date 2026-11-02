package api

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

type farePrice struct {
	amount float64
	bundle string
}

// bundleName names Vueling's fare bundles: Basic is the seat alone, Optima adds a checked bag, seat choice and free changes.
func bundleName(productClass string) string {
	switch productClass {
	case "BA":
		return "Basic"
	case "OP":
		return "Optima"
	case "TF":
		return "TimeFlex"
	default:
		return productClass
	}
}

// journeyFlight maps a journey to a flight priced at its cheapest sellable fare; journeys without one are sold out.
func journeyFlight(j journey, prices map[string]farePrice, direction, day, currency string) (vueling.Flight, bool) {
	fare := math.Inf(1)
	bundles := map[string]float64{}
	for _, f := range j.Fares {
		if f.FareSellKey == "" {
			continue
		}
		price, ok := prices[f.FareAvailabilityKey]
		if !ok {
			continue
		}
		fare = math.Min(fare, price.amount)
		if cheapest, seen := bundles[price.bundle]; price.bundle != "" && (!seen || price.amount < cheapest) {
			bundles[price.bundle] = price.amount
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
	arrivalDate := laterArrivalDate(j.Designator.Departure, j.Designator.Arrival)
	arrivalClock := vueling.ArrivalClock(arrival, localDate(j.Designator.Departure), arrivalDate)
	itinerary := fmt.Sprintf("%s %s → %s %s %s %s", departure, j.Designator.Origin, arrivalClock, j.Designator.Destination, strings.Join(numbers, "+"), routeKind(via, legs))
	if len(operators) > 0 {
		itinerary += " · " + strings.Join(operators, ", ")
	}
	return vueling.Flight{
		Direction:   direction,
		Date:        day,
		Flight:      numbers[0],
		Departure:   departure,
		Arrival:     arrival,
		ArrivalDate: arrivalDate,
		Fare:        vueling.FormatAmount(fare),
		Fares:       bundleFares(bundles),
		Currency:    currency,
		Itinerary:   itinerary,
		Stops:       max(legs-1, len(via)),
		Flights:     numbers,
		Via:         via,
	}, true
}

func bundleFares(bundles map[string]float64) map[string]string {
	if len(bundles) == 0 {
		return nil
	}
	fares := make(map[string]string, len(bundles))
	for bundle, amount := range bundles {
		fares[bundle] = vueling.FormatAmount(amount)
	}
	return fares
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
	if t, ok := localTime(timestamp); ok {
		return t.Format("15:04")
	}
	return timestamp
}

// localDate reads the YYYY-MM-DD day of one of Vueling's local timestamps, or "" when it is unreadable.
func localDate(timestamp string) string {
	if t, ok := localTime(timestamp); ok {
		return t.Format(time.DateOnly)
	}
	return ""
}

// laterArrivalDate returns the local arrival day when a flight lands on a later day than it departs, and "" otherwise.
func laterArrivalDate(departure, arrival string) string {
	arrivalDate := localDate(arrival)
	if vueling.DaysBetween(localDate(departure), arrivalDate) > 0 {
		return arrivalDate
	}
	return ""
}

func localTime(timestamp string) (time.Time, bool) {
	t, err := parseAPITime(strings.TrimSuffix(strings.SplitN(timestamp, ".", 2)[0], "Z"))
	return t, err == nil
}
