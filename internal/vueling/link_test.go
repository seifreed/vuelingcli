package vueling

import (
	"net/url"
	"testing"
	"time"
)

func TestFlightURL(t *testing.T) {
	u, err := FlightURL(LinkOptions{Origin: "bcn", Destination: "AGP", Date: time.Date(2027, 2, 3, 0, 0, 0, 0, time.UTC), ReturnDate: time.Date(2027, 2, 8, 0, 0, 0, 0, time.UTC), Adults: 2, Children: 1, Infants: 1, Discount: 2, OutboundFlight: "VY2115", OutboundFareBasis: "DOWVYCLB", ReturnFlight: "VY5200", ReturnFareBasis: "ABC123", METToken: "testtoken"})
	if err != nil {
		t.Fatal(err)
	}
	p, _ := url.Parse(u)
	for k, want := range map[string]string{"o": "BCN", "d": "AGP", "dd": "2027-02-03", "rd": "2027-02-08", "adt": "2", "chd": "1", "inf": "1", "dt": "2", "c": "es-ES", "cur": "EUR", "ofn": "VY2115", "ofb": "DOWVYCLB", "rfn": "VY5200", "rfb": "ABC123", "MET_Token": "testtoken"} {
		if got := p.Query().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

func TestFlightURLRejectsInvalidExtendedParameters(t *testing.T) {
	base := LinkOptions{Origin: "BCN", Destination: "AGP", Date: time.Date(2027, 2, 3, 0, 0, 0, 0, time.UTC), Adults: 1}
	for name, edit := range map[string]func(*LinkOptions){
		"flight number":              func(o *LinkOptions) { o.OutboundFlight = "VY-12" },
		"fare without flight":        func(o *LinkOptions) { o.OutboundFareBasis = "DOWVYCLB" },
		"return flight without date": func(o *LinkOptions) { o.ReturnFlight = "VY5200" },
		"return fare without flight": func(o *LinkOptions) { o.ReturnFareBasis = "DOWVYCLB" },
		"invalid currency":           func(o *LinkOptions) { o.Currency = "CAD" },
		"invalid token":              func(o *LinkOptions) { o.METToken = "token with spaces" },
	} {
		t.Run(name, func(t *testing.T) {
			o := base
			edit(&o)
			if _, err := FlightURL(o); err == nil {
				t.Fatal("invalid extended link options accepted")
			}
		})
	}
}
