package api

import (
	"math"
	"strings"
)

func pickDescription(pick Pick) string {
	switch {
	case pick.Flight != "":
		return "flight " + strings.ToUpper(pick.Flight)
	case pick.Direct:
		return "direct flight"
	default:
		return "flight"
	}
}

// pickFare finds the journey pick chooses and its cheapest fare that the website can sell.
func pickFare(journeys []journey, prices map[string]farePrice, pick Pick) (journey, fareReference, bool) {
	want := strings.ToUpper(strings.ReplaceAll(pick.Flight, " ", ""))
	var chosen journey
	var chosenFare fareReference
	best := math.Inf(1)
	for _, j := range journeys {
		if !pickMatches(j, want, pick.Direct) {
			continue
		}
		fare, amount, ok := cheapestFare(j, prices)
		if ok && want != "" {
			return j, fare, true
		}
		if ok && amount < best {
			chosen, chosenFare, best = j, fare, amount
		}
	}
	return chosen, chosenFare, !math.IsInf(best, 1)
}

// pickMatches reports whether j flies the wanted flight or, when none is wanted, may be the cheapest pick.
func pickMatches(j journey, want string, direct bool) bool {
	numbers := journeyNumbers(j)
	switch {
	case len(numbers) == 0:
		return false
	case want != "":
		return want == numbers[0] || want == strings.Join(numbers, "+")
	default:
		return !direct || (len(j.Segments) == 1 && len(j.Segments[0].Legs) <= 1)
	}
}

// cheapestFare finds a journey's cheapest fare that the website can sell.
func cheapestFare(j journey, prices map[string]farePrice) (fareReference, float64, bool) {
	var cheapest fareReference
	amount := math.Inf(1)
	for _, f := range j.Fares {
		if price, ok := prices[f.FareAvailabilityKey]; ok && f.FareSellKey != "" && price.amount < amount {
			cheapest, amount = f, price.amount
		}
	}
	return cheapest, amount, !math.IsInf(amount, 1)
}

func journeyNumbers(j journey) []string {
	numbers := make([]string, 0, len(j.Segments))
	for _, segment := range j.Segments {
		numbers = append(numbers, segment.Identifier.CarrierCode+segment.Identifier.Identifier)
	}
	return numbers
}
