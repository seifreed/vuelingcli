package vueling

import (
	"regexp"
	"strings"
)

var flightNumberPattern = regexp.MustCompile(`^VY\d{2,4}[A-Z]?$`)

// IsFlightNumber reports whether s is a Vueling flight number such as VY1158 or VY392D.
func IsFlightNumber(s string) bool {
	return flightNumberPattern.MatchString(s)
}

// FlightStatus is one public flight-status card.
type FlightStatus struct {
	Flight      string `json:"flight"`
	Date        string `json:"date"`
	Status      string `json:"status"`
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
	Departure   string `json:"departure"`
	Arrival     string `json:"arrival"`
	Details     string `json:"details"`
}

// ChangedStatuses returns the cards in next that are new or whose status, departure or arrival differ from previous.
func ChangedStatuses(previous, next []FlightStatus) []FlightStatus {
	seen := make(map[string]FlightStatus, len(previous))
	for _, status := range previous {
		seen[status.Flight+" "+status.Date] = status
	}
	var changed []FlightStatus
	for _, status := range next {
		old, ok := seen[status.Flight+" "+status.Date]
		if !ok || old.Status != status.Status || old.Departure != status.Departure || old.Arrival != status.Arrival {
			changed = append(changed, status)
		}
	}
	return changed
}

// AllFinal reports whether every card has a status that will not change again, such as landed or cancelled.
func AllFinal(statuses []FlightStatus) bool {
	for _, status := range statuses {
		if !isFinal(status.Status) {
			return false
		}
	}
	return len(statuses) > 0
}

// isFinal also accepts Vueling's "No operado" for cancelled flights and "Aterrizado en …" for diverted landings.
func isFinal(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "cancelado", "no operado", "landed", "cancelled":
		return true
	}
	return strings.HasPrefix(status, "aterrizado")
}
