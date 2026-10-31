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

func TestParseFare(t *testing.T) {
	for _, tc := range []struct {
		text string
		want float64
		ok   bool
	}{
		{"49,99 €", 49.99, true},
		{"49.99 EUR", 49.99, true},
		{"35 €", 35, true},
		{"9,9 €", 9.9, true},
		{"1.234,50 €", 1234.5, true},
		{"1,234.50 USD", 1234.5, true},
		{"1.234 €", 1234, true},
		{"€", 0, false},
		{"", 0, false},
	} {
		got, ok := ParseFare(tc.text)
		if got != tc.want || ok != tc.ok {
			t.Errorf("ParseFare(%q) = %v, %v; want %v, %v", tc.text, got, ok, tc.want, tc.ok)
		}
	}
}

func TestMonthDaysAndCheapestDay(t *testing.T) {
	days := monthDays([]renderedDay{{"14", "59,99 €"}, {"3", "49,99 €"}, {"9", "120 €"}, {"3", "10 €"}, {"31", "5 €"}, {"20", "49,99 €"}}, "14/11/2026")
	want := []CalendarDay{{"03/11/2026", "49,99 €"}, {"09/11/2026", "120 €"}, {"14/11/2026", "59,99 €"}, {"20/11/2026", "49,99 €"}}
	if len(days) != len(want) {
		t.Fatalf("monthDays = %v, want %v", days, want)
	}
	for i := range want {
		if days[i] != want[i] {
			t.Fatalf("monthDays = %v, want %v", days, want)
		}
	}
	if cheapest, ok := CheapestDay(days); !ok || cheapest != want[0] {
		t.Fatalf("CheapestDay = %v, %v; want %v", cheapest, ok, want[0])
	}
	if _, ok := CheapestDay(nil); ok {
		t.Fatal("CheapestDay found a fare in no days")
	}
	if got := monthDays([]renderedDay{{"1", "9 €"}}, ""); got != nil {
		t.Fatalf("monthDays without reference = %v", got)
	}
}
