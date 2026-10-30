package vueling

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// CalendarURL builds Vueling's public price-calendar URL from airport codes.
func CalendarURL(origin, destination string, date time.Time) (string, error) {
	origin, destination = strings.ToUpper(strings.TrimSpace(origin)), strings.ToUpper(strings.TrimSpace(destination))
	if !validIATACode(origin) || !validIATACode(destination) || origin == destination {
		return "", fmt.Errorf("origin and destination must be different three-letter IATA codes")
	}
	if date.IsZero() {
		return "", fmt.Errorf("date is required")
	}
	query := url.Values{
		"originCalendar":      {origin},
		"destinationCalendar": {destination},
		"dateComplete":        {date.Format("02/01/2006")},
	}
	return "https://www.vueling.com/es/calendario-de-precios?" + query.Encode(), nil
}
