package vueling

import (
	"net/url"
	"testing"
	"time"
)

func TestCalendarURL(t *testing.T) {
	got, err := CalendarURL("bcn", "mla", time.Date(2027, 1, 14, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(got)
	if err != nil || u.Host != "www.vueling.com" || u.Path != "/es/calendario-de-precios" || u.Query().Get("originCalendar") != "BCN" || u.Query().Get("destinationCalendar") != "MLA" || u.Query().Get("dateComplete") != "14/01/2027" {
		t.Fatalf("unexpected calendar URL %q (%v)", got, err)
	}
	if _, err := CalendarURL("BCN", "BCN", time.Now()); err == nil {
		t.Fatal("same-airport route accepted")
	}
}
