package vueling

// Extras are the seat, bag and other service prices Vueling offers for one flight once its fare
// is in a cart; Bundle is the fare bundle the prices assume, since bundles include some extras.
type Extras struct {
	Flight      string       `json:"flight"`
	Date        string       `json:"date"`
	Origin      string       `json:"origin"`
	Destination string       `json:"destination"`
	Bundle      string       `json:"bundle"`
	Currency    string       `json:"currency"`
	Seats       []ExtraPrice `json:"seats"`
	Bags        []ExtraPrice `json:"bags"`
	Services    []ExtraPrice `json:"services"`
}

// ExtraPrice is one priced extra. A seat type spans Price to MaxPrice and counts the seats still
// Available; Segment names the flight leg of a connection a seat price belongs to.
type ExtraPrice struct {
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Segment      string  `json:"segment,omitempty"`
	Price        float64 `json:"price"`
	MaxPrice     float64 `json:"maxPrice,omitempty"`
	Available    *int    `json:"available,omitempty"`
	Currency     string  `json:"currency"`
	PerPassenger bool    `json:"perPassenger,omitempty"`
}

// seatTypeNames names the website's seat map sections.
var seatTypeNames = map[string]string{
	"spaceone":   "Space One (first row)",
	"spaceplus":  "Space Plus",
	"space":      "Space (extra legroom)",
	"front":      "Front",
	"back":       "Back",
	"excellence": "Excellence",
}

// SeatTypeName names a seat map section, reporting false for sections the website hides.
func SeatTypeName(code string) (string, bool) {
	name, ok := seatTypeNames[code]
	return name, ok
}

// serviceNames names the services the website sells per flight, in the words of its basket.
var serviceNames = map[string]string{
	"GCBG": "Overhead cabin bag",
	"CBHG": "Cabin bag put in the hold at the gate",
	"HBBG": "Heavy bag at the boarding gate",
	"PETC": "Pet in the cabin",
	"FXPK": "Flex Pack (cancellation + flight change)",
	"BIKE": "Bicycle",
	"SPBG": "Golf equipment",
	"SPEQ": "Other special luggage",
	"SPIM": "Musical instrument",
	"SPKB": "Kitesurfing equipment",
	"SPSK": "Skiing equipment",
	"SPSW": "Snowboard",
	"SPTS": "Surfboard",
	"SPWS": "Windsurfing equipment",
	"WEAP": "Sports firearm",
}

// ServiceName names a service code, or returns the code itself for services the CLI does not know.
func ServiceName(code string) string {
	if name, ok := serviceNames[code]; ok {
		return name
	}
	return code
}
