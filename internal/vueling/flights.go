// Package vueling contains the domain data and rules for Vueling flights and fares.
package vueling

import (
	"strconv"
	"time"
)

// Flight is one rendered Vueling offer.
type Flight struct {
	Direction string `json:"direction"`
	Date      string `json:"date"`
	Flight    string `json:"flight"`
	Departure string `json:"departure"`
	Arrival   string `json:"arrival"`
	// ArrivalDate is the local landing day (YYYY-MM-DD), set only when it is later than Date.
	ArrivalDate string `json:"arrivalDate,omitempty"`
	Fare        string `json:"fare"`
	// Fares holds the cheapest price of each fare bundle, such as Basic and Optima, when Vueling's API lists them.
	Fares     map[string]string `json:"fares,omitempty"`
	Currency  string            `json:"currency"`
	Itinerary string            `json:"itinerary"`
	Stops     int               `json:"stops"`
	Flights   []string          `json:"flights,omitempty"`
	Via       []string          `json:"via,omitempty"`
}

// ArrivalClock renders the flight's arrival time with the days it lands after departure, such as "00:45 +1".
func (f Flight) ArrivalClock() string {
	return ArrivalClock(f.Arrival, f.Date, f.ArrivalDate)
}

// ArrivalClock appends "+N" to an arrival clock time when arrivalDate is N days after departureDate;
// unreadable or missing dates leave the clock time as it is.
func ArrivalClock(arrival, departureDate, arrivalDate string) string {
	if days := DaysBetween(departureDate, arrivalDate); days > 0 {
		return arrival + " +" + strconv.Itoa(days)
	}
	return arrival
}

// DaysBetween counts the calendar days from one YYYY-MM-DD date to another, or 0 when either is unreadable.
func DaysBetween(from, to string) int {
	start, err := time.Parse(time.DateOnly, from)
	if err != nil {
		return 0
	}
	end, err := time.Parse(time.DateOnly, to)
	if err != nil {
		return 0
	}
	return int(end.Sub(start).Hours() / 24)
}
