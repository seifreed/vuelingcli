package vueling

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"time"
)

// FlightFilter narrows, sorts, and limits flight offers per direction.
type FlightFilter struct {
	Cheapest bool
	Direct   bool
	Limit    int
	After    string
	Before   string
	// ReturnAfter and ReturnBefore replace After and Before for return offers when either is set.
	ReturnAfter  string
	ReturnBefore string
}

// Vueling cards say "Directo" for nonstop flights; connections mention a stop
// ("escala", "conexión") or list more than one VY flight number.
var (
	stopPattern       = regexp.MustCompile(`(?i)\b(escalas?|conexi[oó]n|stops?|connection)\b`)
	connectionPattern = regexp.MustCompile(`(?i)\bconexi[oó]n en ([A-Z]{3})\b`)
)

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
		window := f
		if direction == "return" && (f.ReturnAfter != "" || f.ReturnBefore != "") {
			window.After, window.Before = f.ReturnAfter, f.ReturnBefore
		}
		var kept []Flight
		for _, flight := range groups[direction] {
			if keepFlight(flight, window) {
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
	return readLegs(flight.Flight, flight.Itinerary).stops == 0
}

type legs struct {
	stops   int
	flights []string
	via     []string
}

// readLegs derives the connection legs that the rendered card text names: every
// VY flight number in order, the "Conexión en XXX" airports, and the stop count.
func readLegs(flightNumber, itinerary string) legs {
	flights := uniqueInOrder(append([]string{flightNumber}, flightNumberPattern.FindAllString(itinerary, -1)...))
	var airports []string
	for _, match := range connectionPattern.FindAllStringSubmatch(itinerary, -1) {
		airports = append(airports, match[1])
	}
	via := uniqueInOrder(airports)
	stops := max(len(flights)-1, len(via))
	if stops == 0 && stopPattern.MatchString(itinerary) {
		stops = 1
	}
	return legs{stops: stops, flights: flights, via: via}
}

func uniqueInOrder(values []string) []string {
	var unique []string
	for _, value := range values {
		if value != "" && !slices.Contains(unique, value) {
			unique = append(unique, value)
		}
	}
	return unique
}
