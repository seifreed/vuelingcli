package vueling

// Alert is the disruption notice (strikes, weather, outages) Vueling shows across its website.
type Alert struct {
	Active bool   `json:"active"`
	Text   string `json:"text,omitempty"`
	URL    string `json:"url,omitempty"`
}
