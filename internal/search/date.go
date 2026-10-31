// Package search parses the travel dates the CLI accepts.
package search

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseDate accepts ISO dates and the common relative forms used by the CLI.
func ParseDate(value string, now time.Time) (time.Time, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch value {
	case "today":
		return today, nil
	case "tomorrow":
		return today.AddDate(0, 0, 1), nil
	}
	weekdays := map[string]time.Weekday{
		"sunday": time.Sunday, "sun": time.Sunday, "domingo": time.Sunday,
		"monday": time.Monday, "mon": time.Monday, "lunes": time.Monday,
		"tuesday": time.Tuesday, "tue": time.Tuesday, "martes": time.Tuesday,
		"wednesday": time.Wednesday, "wed": time.Wednesday, "miércoles": time.Wednesday, "miercoles": time.Wednesday,
		"thursday": time.Thursday, "thu": time.Thursday, "jueves": time.Thursday,
		"friday": time.Friday, "fri": time.Friday, "viernes": time.Friday,
		"saturday": time.Saturday, "sat": time.Saturday, "sábado": time.Saturday, "sabado": time.Saturday,
	}
	if weekday, ok := weekdays[value]; ok {
		days := (int(weekday) - int(today.Weekday()) + 7) % 7
		if days == 0 {
			days = 7
		}
		return today.AddDate(0, 0, days), nil
	}
	if strings.HasPrefix(value, "+") {
		days, err := strconv.Atoi(value[1:])
		if err != nil || days < 0 {
			return time.Time{}, fmt.Errorf("invalid relative date %q (use +N)", value)
		}
		return today.AddDate(0, 0, days), nil
	}
	for _, layout := range []string{time.DateOnly, "02/01/2006"} {
		if parsed, err := time.ParseInLocation(layout, value, now.Location()); err == nil {
			if parsed.Before(today) {
				return time.Time{}, fmt.Errorf("date must be today or later")
			}
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date %q (use YYYY-MM-DD, DD/MM/YYYY, today, tomorrow, or +N)", value)
}
