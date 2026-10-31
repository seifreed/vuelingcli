package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

// ErrNoFlightStatus reports that Vueling returned no flights for a status search.
var ErrNoFlightStatus = errors.New("Vueling found no flight status for that search and date")

// statusLabels maps the API's status names to the Spanish labels Vueling's flight-status page shows.
// Names come from the site's FlightStatus enum; labels from its flightStatusList settings ({0} is the destination).
var statusLabels = map[string]string{
	"Unknown":                        "Desconocido",
	"OnTime":                         "En Hora",
	"Delayed":                        "Nueva hora",
	"Flying":                         "En vuelo",
	"Derived":                        "Desviado a {0}",
	"Landed":                         "Aterrizado",
	"Cancelled":                      "No operado",
	"DerivedLanded":                  "Aterrizado en {0}",
	"NoInfo":                         "Información no disponible",
	"Incident":                       "Incidente",
	"DerivedFrom":                    "Salida desde",
	"DerivedFromLanded":              "Aterrizado en",
	"CancelledRescheduled":           "Reprogramado",
	"CodeShareStatus":                "Estado de código compartido",
	"ProvisionalDelayedByMeteo":      "Nueva hora provisional",
	"ProvisionalDelayedByRegulation": "Provisional retrasada por reglamento",
}

const apiTimeLayout = "2006-01-02T15:04:05"

type statusResponse struct {
	Flights []statusFlight `json:"flights"`
}

type statusFlight struct {
	FlightCode string `json:"flightCode"`
	Status     string `json:"status"`
	Segment    struct {
		Origin      statusStop `json:"origin"`
		Destination statusStop `json:"destination"`
	} `json:"segment"`
	Connection *struct {
		DurationMinutes int    `json:"durationMinutes"`
		Location        string `json:"location"`
	} `json:"connection"`
}

type statusStop struct {
	IATA         string `json:"iata"`
	Gate         string `json:"gateDescription"`
	Terminal     string `json:"terminalDescription"`
	BaggageClaim string `json:"baggageClaimDescription"`
	Scheduled    string `json:"timeScheduled"`
	ToShow       string `json:"timeUpdatedToShow"`
}

// FlightStatus reads Vueling's public flight status for one VY flight, or for every flight on a route when flight is empty.
func (c *Client) FlightStatus(ctx context.Context, flight, origin, destination string, date time.Time) ([]vueling.FlightStatus, error) {
	day := date.Format(time.DateOnly)
	var endpoint string
	switch {
	case vueling.IsFlightNumber(flight):
		endpoint = c.APIW + "/api/v1/flights/" + flight + "?" + url.Values{"date": {day}}.Encode()
	case flight != "":
		return nil, fmt.Errorf("flight must be VY followed by its number, such as VY1158 or VY392D")
	case vueling.IsIATACode(origin) && vueling.IsIATACode(destination) && origin != destination:
		endpoint = c.APIW + "/api/v1/flights?" + url.Values{"date": {day}, "originCode": {origin}, "destinationCode": {destination}}.Encode()
	default:
		return nil, fmt.Errorf("route needs two different three-letter IATA codes")
	}
	var response statusResponse
	if err := c.getJSON(ctx, endpoint, wwwOrigin, &response); err != nil {
		return nil, err
	}
	if len(response.Flights) == 0 {
		return nil, ErrNoFlightStatus
	}
	statuses := make([]vueling.FlightStatus, 0, len(response.Flights))
	for _, f := range response.Flights {
		statuses = append(statuses, f.toStatus(day))
	}
	return statuses, nil
}

func (f statusFlight) toStatus(requestedDay string) vueling.FlightStatus {
	from, to := f.Segment.Origin, f.Segment.Destination
	day := requestedDay
	if scheduled, err := time.Parse(apiTimeLayout, from.Scheduled); err == nil {
		day = scheduled.Format(time.DateOnly)
	}
	return vueling.FlightStatus{
		Flight:      f.FlightCode,
		Date:        day,
		Status:      statusLabel(f.Status, to.IATA),
		Origin:      from.IATA,
		Destination: to.IATA,
		Departure:   from.shownClock(),
		Arrival:     to.shownClock(),
		Details:     f.details(),
	}
}

func statusLabel(status, destination string) string {
	label, ok := statusLabels[status]
	if !ok {
		return status
	}
	return strings.TrimSpace(strings.ReplaceAll(label, "{0}", destination))
}

func (s statusStop) shownClock() string {
	if shown := clock(s.ToShow); shown != "" {
		return shown
	}
	return clock(s.Scheduled)
}

func clock(value string) string {
	parsed, err := time.Parse(apiTimeLayout, value)
	if err != nil || parsed.Year() < 1900 {
		return ""
	}
	return parsed.Format("15:04")
}

func (f statusFlight) details() string {
	from, to := f.Segment.Origin, f.Segment.Destination
	parts := []string{fmt.Sprintf("scheduled %s–%s", clock(from.Scheduled), clock(to.Scheduled))}
	if facilities := from.facilities(); facilities != "" {
		parts = append(parts, from.IATA+" "+facilities)
	}
	if facilities := to.facilities(); facilities != "" {
		parts = append(parts, to.IATA+" "+facilities)
	}
	if f.Connection != nil && f.Connection.Location != "" {
		parts = append(parts, fmt.Sprintf("connection in %s (%d min)", f.Connection.Location, f.Connection.DurationMinutes))
	}
	return strings.Join(parts, "; ")
}

func (s statusStop) facilities() string {
	var parts []string
	for _, field := range []struct{ label, value string }{{"terminal", s.Terminal}, {"gate", s.Gate}, {"belt", s.BaggageClaim}} {
		if value := strings.TrimSpace(field.value); value != "" {
			parts = append(parts, field.label+" "+value)
		}
	}
	return strings.Join(parts, ", ")
}
