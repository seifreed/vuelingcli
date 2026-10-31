package vueling

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"time"
)

type FlightFilter struct {
	Cheapest bool
	Direct   bool
	Limit    int
	After    string
	Before   string
}

// Vueling cards say "Directo" for nonstop flights; connections mention a stop
// ("escala", "conexión") or list more than one VY flight number.
var stopPattern = regexp.MustCompile(`(?i)\b(escalas?|conexi[oó]n|stops?|connection)\b`)

// ParseClock validates an HH:MM time and returns minutes after midnight.
func ParseClock(value string) (int, error) {
	t, err := time.Parse("15:04", value)
	if err != nil {
		return 0, fmt.Errorf("time must be HH:MM, got %q", value)
	}
	return t.Hour()*60 + t.Minute(), nil
}

// FilterFlights applies f to each direction independently, keeping direction order.
func FilterFlights(flights []Flight, f FlightFilter) []Flight {
	var directions []string
	groups := map[string][]Flight{}
	for _, flight := range flights {
		if _, seen := groups[flight.Direction]; !seen {
			directions = append(directions, flight.Direction)
		}
		groups[flight.Direction] = append(groups[flight.Direction], flight)
	}
	result := []Flight{}
	for _, direction := range directions {
		var kept []Flight
		for _, flight := range groups[direction] {
			if keepFlight(flight, f) {
				kept = append(kept, flight)
			}
		}
		if f.Cheapest {
			slices.SortStableFunc(kept, compareFares)
		}
		if f.Limit > 0 && len(kept) > f.Limit {
			kept = kept[:f.Limit]
		}
		result = append(result, kept...)
	}
	return result
}

func compareFares(a, b Flight) int {
	x, errA := strconv.ParseFloat(a.Fare, 64)
	y, errB := strconv.ParseFloat(b.Fare, 64)
	switch {
	case errA != nil && errB != nil:
		return 0
	case errA != nil:
		return 1
	case errB != nil:
		return -1
	}
	return cmp.Compare(x, y)
}

func keepFlight(flight Flight, f FlightFilter) bool {
	if f.Direct && !isDirect(flight) {
		return false
	}
	if f.After == "" && f.Before == "" {
		return true
	}
	departure, err := ParseClock(flight.Departure)
	if err != nil {
		return false
	}
	if after, err := ParseClock(f.After); err == nil && departure < after {
		return false
	}
	if before, err := ParseClock(f.Before); err == nil && departure > before {
		return false
	}
	return true
}

func isDirect(flight Flight) bool {
	numbers := map[string]bool{flight.Flight: true}
	for _, number := range flightNumberPattern.FindAllString(flight.Itinerary, -1) {
		numbers[number] = true
	}
	return len(numbers) == 1 && !stopPattern.MatchString(flight.Itinerary)
}
