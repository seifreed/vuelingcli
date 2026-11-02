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
	if _, err := CalendarURL("BCN", "AGP", time.Time{}); err == nil {
		t.Fatal("zero date accepted")
	}
}

func TestCheapestDay(t *testing.T) {
	days := []CalendarDay{{"03/11/2026", "49,99 €"}, {"09/11/2026", "120 €"}, {"14/11/2026", "59,99 €"}, {"20/11/2026", "49,99 €"}}
	if cheapest, ok := CheapestDay(days); !ok || cheapest != days[0] {
		t.Fatalf("CheapestDay = %v, %v; want %v", cheapest, ok, days[0])
	}
	if _, ok := CheapestDay(nil); ok {
		t.Fatal("CheapestDay found a fare in no days")
	}
}

func TestFareOnAndMonthOf(t *testing.T) {
	days := []CalendarDay{{"30/11/2026", "40 €"}, {"03/12/2026", "29 €"}, {"20/12/2026", "35 €"}, {"03/01/2027", "50 €"}}
	if day, ok := FareOn(days, "03/12/2026"); !ok || day != days[1] {
		t.Fatalf("FareOn = %v, %v; want %v", day, ok, days[1])
	}
	if _, ok := FareOn(days, "04/12/2026"); ok {
		t.Fatal("FareOn found a fare on a day without one")
	}
	if month := MonthOf(days, "15/12/2026"); len(month) != 2 || month[0] != days[1] || month[1] != days[2] {
		t.Fatalf("MonthOf = %v, want December's two days", month)
	}
	if month := MonthOf(days, ""); len(month) != 0 {
		t.Fatalf("MonthOf without a date = %v, want none", month)
	}
}
