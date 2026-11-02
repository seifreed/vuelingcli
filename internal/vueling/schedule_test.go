package vueling

import (
	"reflect"
	"testing"
	"time"
)

func TestNewScheduleGroupsTheWindowByMonth(t *testing.T) {
	days := []ScheduledDay{
		{Date: "2027-02-27", Flight: "VY1"},
		{Date: "2027-03-03", Flight: "VY8502"},
		{Date: "2027-03-06", Flight: "VY8502"},
		{Date: "2027-03-10", Flight: "VY8502"},
		{Date: "2027-05-02", Flight: "VY8502"},
		{Date: "2027-06-01", Flight: "VY9"},
	}
	schedule := NewSchedule("BCN", "ESU", days, time.Date(2027, 3, 1, 15, 0, 0, 0, time.Local), 3)
	want := Schedule{
		Origin: "BCN", Destination: "ESU", From: "2027-03-01", To: "2027-05-31",
		First: "2027-03-03", Last: "2027-05-02",
		Weekdays: []string{"mié", "sáb", "dom"},
		Months: []ScheduleMonth{
			{Month: "2027-03", Days: []int{3, 6, 10}, Weekdays: []string{"mié", "sáb"}},
			{Month: "2027-04", Days: []int{}, Weekdays: []string{}},
			{Month: "2027-05", Days: []int{2}, Weekdays: []string{"dom"}},
		},
		Dates: []ScheduledDay{
			{Date: "2027-03-03", Weekday: "mié", Flight: "VY8502"},
			{Date: "2027-03-06", Weekday: "sáb", Flight: "VY8502"},
			{Date: "2027-03-10", Weekday: "mié", Flight: "VY8502"},
			{Date: "2027-05-02", Weekday: "dom", Flight: "VY8502"},
		},
	}
	if !reflect.DeepEqual(schedule, want) {
		t.Fatalf("schedule = %+v\nwant %+v", schedule, want)
	}
}

func TestNewScheduleWithoutFlights(t *testing.T) {
	schedule := NewSchedule("BCN", "FEZ", []ScheduledDay{{Date: "bad"}}, time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC), 2)
	if schedule.First != "" || len(schedule.Dates) != 0 || len(schedule.Months) != 2 || schedule.Months[1].Month != "2027-01" || schedule.To != "2027-01-31" {
		t.Fatalf("schedule = %+v", schedule)
	}
}
