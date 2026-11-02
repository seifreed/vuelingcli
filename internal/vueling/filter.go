package vueling

import (
	"cmp"
	"fmt"
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

// ParseClock validates an HH:MM time and returns minutes after midnight.
func ParseClock(value string) (int, error) {
	t, err := time.Parse("15:04", value)
	if err != nil {
		return 0, fmt.Errorf("time must be HH:MM, got %q", value)
	}
	return t.Hour()*60 + t.Minute(), nil
}

type clockWindow struct {
	after, before       int
	hasAfter, hasBefore bool
}

func parseClockWindow(afterValue, beforeValue string) (clockWindow, error) {
	window := clockWindow{hasAfter: afterValue != "", hasBefore: beforeValue != ""}
	var err error
	if window.hasAfter {
		window.after, err = ParseClock(afterValue)
		if err != nil {
			return clockWindow{}, fmt.Errorf("after filter: %w", err)
		}
	}
	if window.hasBefore {
		window.before, err = ParseClock(beforeValue)
		if err != nil {
			return clockWindow{}, fmt.Errorf("before filter: %w", err)
		}
	}
	return window, nil
}

// FilterFlights applies f to each direction independently, keeping direction order.
func FilterFlights(flights []Flight, f FlightFilter) ([]Flight, error) {
	window, err := parseClockWindow(f.After, f.Before)
	if err != nil {
		return nil, err
	}
	returnWindow, err := parseClockWindow(f.ReturnAfter, f.ReturnBefore)
	if err != nil {
		return nil, fmt.Errorf("return %w", err)
	}
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
		directionWindow := window
		if direction == "return" && (f.ReturnAfter != "" || f.ReturnBefore != "") {
			directionWindow = returnWindow
		}
		var kept []Flight
		for _, flight := range groups[direction] {
			keep, err := keepFlight(flight, f.Direct, directionWindow)
			if err != nil {
				return nil, fmt.Errorf("flight %s: %w", flight.Flight, err)
			}
			if keep {
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
	return result, nil
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

func keepFlight(flight Flight, direct bool, window clockWindow) (bool, error) {
	if direct && !isDirect(flight) {
		return false, nil
	}
	if !window.hasAfter && !window.hasBefore {
		return true, nil
	}
	departure, err := ParseClock(flight.Departure)
	if err != nil {
		return false, fmt.Errorf("departure time: %w", err)
	}
	if window.hasAfter && departure < window.after {
		return false, nil
	}
	if window.hasBefore && departure > window.before {
		return false, nil
	}
	return true, nil
}

func isDirect(flight Flight) bool {
	return flight.Stops == 0
}
