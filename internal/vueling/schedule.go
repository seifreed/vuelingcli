package vueling

import "time"

// ScheduledDay is a day Vueling sells a flight on a route, with that day's cheapest flight.
type ScheduledDay struct {
	Date       string `json:"date"`
	Weekday    string `json:"weekday"`
	Flight     string `json:"flight"`
	Departure  string `json:"departure"`
	Fare       string `json:"fare"`
	Connection bool   `json:"connection,omitempty"`
}

// ScheduleMonth lists the days of one calendar month on which a route operates.
type ScheduleMonth struct {
	Month    string   `json:"month"`
	Days     []int    `json:"days"`
	Weekdays []string `json:"weekdays"`
}

// Schedule is when a route operates within a window of whole or partial months.
type Schedule struct {
	Origin      string          `json:"origin"`
	Destination string          `json:"destination"`
	From        string          `json:"from"`
	To          string          `json:"to"`
	First       string          `json:"firstDate,omitempty"`
	Last        string          `json:"lastDate,omitempty"`
	Weekdays    []string        `json:"weekdays"`
	Months      []ScheduleMonth `json:"months"`
	Dates       []ScheduledDay  `json:"dates"`
}

// SpanishWeekdays abbreviates weekdays the way Vueling's Spanish website does, indexed by time.Weekday.
var SpanishWeekdays = [7]string{"dom", "lun", "mar", "mié", "jue", "vie", "sáb"}

// NewSchedule keeps the days, dated YYYY-MM-DD and sorted, that fall between from and the end of
// the window of months, and groups them by month.
func NewSchedule(origin, destination string, days []ScheduledDay, from time.Time, months int) Schedule {
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	end := time.Date(from.Year(), from.Month()+time.Month(months), 1, 0, 0, 0, 0, time.UTC)
	schedule := Schedule{
		Origin: origin, Destination: destination,
		From: from.Format(time.DateOnly), To: end.AddDate(0, 0, -1).Format(time.DateOnly),
		Weekdays: []string{}, Dates: []ScheduledDay{},
	}
	for month := 0; month < months; month++ {
		start := time.Date(from.Year(), from.Month()+time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		schedule.Months = append(schedule.Months, ScheduleMonth{Month: start.Format("2006-01"), Days: []int{}, Weekdays: []string{}})
	}
	var operating [7]bool
	monthly := make([][7]bool, months)
	for _, day := range days {
		date, err := time.Parse(time.DateOnly, day.Date)
		if err != nil || date.Before(from) || !date.Before(end) {
			continue
		}
		index := (date.Year()-from.Year())*12 + int(date.Month()-from.Month())
		schedule.Months[index].Days = append(schedule.Months[index].Days, date.Day())
		monthly[index][date.Weekday()] = true
		operating[date.Weekday()] = true
		day.Weekday = SpanishWeekdays[date.Weekday()]
		schedule.Dates = append(schedule.Dates, day)
	}
	for i := range schedule.Months {
		schedule.Months[i].Weekdays = weekdayNames(monthly[i])
	}
	schedule.Weekdays = weekdayNames(operating)
	if len(schedule.Dates) > 0 {
		schedule.First, schedule.Last = schedule.Dates[0].Date, schedule.Dates[len(schedule.Dates)-1].Date
	}
	return schedule
}

// weekdayNames lists the marked weekdays from Monday to Sunday.
func weekdayNames(marked [7]bool) []string {
	names := []string{}
	for i := 1; i <= 7; i++ {
		if weekday := time.Weekday(i % 7); marked[weekday] {
			names = append(names, SpanishWeekdays[weekday])
		}
	}
	return names
}
