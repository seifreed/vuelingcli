package vueling

// Route is a destination Vueling sells from an origin, as listed in the website's route network.
type Route struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	CountryCode string `json:"countryCode"`
	Country     string `json:"country"`
	Direct      bool   `json:"direct"`
	Residents   bool   `json:"residentDiscount"`
	LargeFamily bool   `json:"largeFamilyDiscount"`
}
