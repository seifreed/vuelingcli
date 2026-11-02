package vueling

// Trip is a return trip priced from the cheapest calendar fare of each leg's day, per adult.
type Trip struct {
	Departure  string  `json:"departure"`
	Return     string  `json:"return"`
	Nights     int     `json:"nights"`
	Outbound   float64 `json:"outbound"`
	ReturnFare float64 `json:"returnFare"`
	Total      float64 `json:"total"`
	Currency   string  `json:"currency"`
	URL        string  `json:"url,omitempty"`
}
