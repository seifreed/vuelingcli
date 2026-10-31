// Package vueling contains the domain data and rules for Vueling flights and fares.
package vueling

// Flight is one rendered Vueling offer.
type Flight struct {
	Direction string   `json:"direction"`
	Date      string   `json:"date"`
	Flight    string   `json:"flight"`
	Departure string   `json:"departure"`
	Arrival   string   `json:"arrival"`
	Fare      string   `json:"fare"`
	Currency  string   `json:"currency"`
	Itinerary string   `json:"itinerary"`
	Stops     int      `json:"stops"`
	Flights   []string `json:"flights,omitempty"`
	Via       []string `json:"via,omitempty"`
}
