package web

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

type renderedDay struct {
	Day  string `json:"day"`
	Fare string `json:"fare"`
}

func monthDays(rendered []renderedDay, reference string) []vueling.CalendarDay {
	if len(reference) != len("02/01/2006") {
		return nil
	}
	seen := map[string]bool{}
	var days []vueling.CalendarDay
	for _, day := range rendered {
		number, err := strconv.Atoi(day.Day)
		if err != nil {
			continue
		}
		date := fmt.Sprintf("%02d%s", number, reference[2:])
		if _, err := time.Parse("02/01/2006", date); err != nil || seen[date] {
			continue
		}
		seen[date] = true
		days = append(days, vueling.CalendarDay{Date: date, Fare: day.Fare})
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Date < days[j].Date })
	return days
}
