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

func TestFlightSearchURL(t *testing.T) {
	u, err := FlightSearchURL(LinkOptions{Origin: "bcn", Destination: "lgw", Date: time.Date(2027, 2, 3, 0, 0, 0, 0, time.UTC), Adults: 1, Currency: "gbp"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	if p.Host != "tickets.vueling.com" || p.Path != "/booking/flightSearch" {
		t.Fatalf("unexpected flight-search location %q", u)
	}
	for k, want := range map[string]string{"o": "BCN", "d": "LGW", "dd": "2027-02-03", "adt": "1", "chd": "0", "inf": "0", "dt": "0", "b": "1", "cur": "GBP"} {
		if got := p.Query().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if _, err := FlightSearchURL(LinkOptions{Origin: "BCN", Destination: "BCN", Date: time.Date(2027, 2, 3, 0, 0, 0, 0, time.UTC), Adults: 1}); err == nil {
		t.Fatal("same-airport flight search accepted")
	}
}

func TestFlightURLRejectsInvalidCoreParameters(t *testing.T) {
	base := LinkOptions{Origin: "BCN", Destination: "AGP", Date: time.Date(2027, 2, 3, 0, 0, 0, 0, time.UTC), Adults: 1}
	for name, edit := range map[string]func(*LinkOptions){
		"short origin":             func(o *LinkOptions) { o.Origin = "BC" },
		"numeric destination":      func(o *LinkOptions) { o.Destination = "A1P" },
		"missing date":             func(o *LinkOptions) { o.Date = time.Time{} },
		"no adults":                func(o *LinkOptions) { o.Adults = 0 },
		"more infants than adults": func(o *LinkOptions) { o.Infants = 2 },
		"ten passengers":           func(o *LinkOptions) { o.Adults, o.Children = 5, 5 },
		"passenger count overflow": func(o *LinkOptions) { o.Children = int(^uint(0) >> 1) },
		"return before departure":  func(o *LinkOptions) { o.ReturnDate = o.Date.AddDate(0, 0, -1) },
		"discount above five":      func(o *LinkOptions) { o.Discount = 6 },
		"flight number too long":   func(o *LinkOptions) { o.OutboundFlight = "VY12345" },
		"flight number letters":    func(o *LinkOptions) { o.OutboundFlight = "VY12A" },
		"non-alphanumeric fare":    func(o *LinkOptions) { o.OutboundFlight, o.OutboundFareBasis = "VY1234", "DOW-VY" },
	} {
		t.Run(name, func(t *testing.T) {
			o := base
			edit(&o)
			if _, err := FlightURL(o); err == nil {
				t.Fatal("invalid link options accepted")
			}
		})
	}
}

func TestIsAlphaNumeric(t *testing.T) {
	for value, want := range map[string]bool{"abc123": true, "": false, "a-b": false} {
		if got := isAlphaNumeric(value); got != want {
			t.Errorf("isAlphaNumeric(%q) = %v, want %v", value, got, want)
		}
	}
}
