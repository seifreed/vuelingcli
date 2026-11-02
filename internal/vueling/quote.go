package vueling

// Quote is the price breakdown Vueling shows when a flight's fare is clicked, before anything is sold.
// InfantFee is the lap-infant fare and fees for every infant; Total already includes it.
type Quote struct {
	Flight      string             `json:"flight"`
	Date        string             `json:"date"`
	Origin      string             `json:"origin"`
	Destination string             `json:"destination"`
	Departure   string             `json:"departure"`
	Arrival     string             `json:"arrival"`
	ArrivalDate string             `json:"arrivalDate,omitempty"`
	Currency    string             `json:"currency"`
	Fare        float64            `json:"fare"`
	Taxes       float64            `json:"taxes"`
	TaxDetail   map[string]float64 `json:"taxDetail,omitempty"`
	Infants     int                `json:"infants,omitempty"`
	InfantFee   float64            `json:"infantFee,omitempty"`
	Total       float64            `json:"total"`
	Bundles     []Bundle           `json:"bundles"`
}

// Bundle is a fare bundle the website offers for a flight, priced per person and for every passenger.
type Bundle struct {
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	PricePerPerson float64 `json:"pricePerPerson"`
	Total          float64 `json:"total"`
}

// DefaultExtrasBundle is the fare bundle used when pricing extras without an explicit choice.
const DefaultExtrasBundle = "VYBA"

// bundleNames names the bundles the website shows; it hides Optima (VYOP) and TimeFlex (VYTF) offers.
var bundleNames = map[string]string{
	"VYBA": "Fly Light",
	"VCB2": "Fly",
	"VCB1": "Fly (25 kg bag)",
	"VPRO": "Fly Pro",
	"VCB3": "Fly Grande",
}

// BundleName names a bundle code the website offers, reporting false for codes it hides.
func BundleName(code string) (string, bool) {
	name, ok := bundleNames[code]
	return name, ok
}
