package web

import (
	"reflect"
	"testing"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

func TestMonthDays(t *testing.T) {
	days := monthDays([]renderedDay{{"14", "59,99 €"}, {"3", "49,99 €"}, {"9", "120 €"}, {"3", "10 €"}, {"31", "5 €"}, {"20", "49,99 €"}}, "14/11/2026")
	want := []vueling.CalendarDay{
		{Date: "03/11/2026", Fare: "49,99 €"},
		{Date: "09/11/2026", Fare: "120 €"},
		{Date: "14/11/2026", Fare: "59,99 €"},
		{Date: "20/11/2026", Fare: "49,99 €"},
	}
	if !reflect.DeepEqual(days, want) {
		t.Fatalf("monthDays = %v, want %v", days, want)
	}
	if got := monthDays([]renderedDay{{"1", "9 €"}}, ""); got != nil {
		t.Fatalf("monthDays without reference = %v", got)
	}
}
