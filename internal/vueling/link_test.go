package vueling

import (
	"net/url"
	"testing"
	"time"
)

func TestFlightURL(t *testing.T) {
	u, err := FlightURL(LinkOptions{Origin: "bcn", Destination: "AGP", Date: time.Date(2027, 2, 3, 0, 0, 0, 0, time.UTC), ReturnDate: time.Date(2027, 2, 8, 0, 0, 0, 0, time.UTC), Adults: 2, Children: 1, Infants: 1, Discount: 2})
	if err != nil {
		t.Fatal(err)
	}
	p, _ := url.Parse(u)
	for k, want := range map[string]string{"o": "BCN", "d": "AGP", "dd": "2027-02-03", "rd": "2027-02-08", "adt": "2", "chd": "1", "inf": "1", "dt": "2", "c": "es-ES", "cur": "EUR"} {
		if got := p.Query().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}
