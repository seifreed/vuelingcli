// Package date parses travel dates accepted by the CLI.
package date

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseDate accepts ISO dates and the common relative forms used by the CLI, rejecting calendar dates before today.
func ParseDate(value string, now time.Time) (time.Time, error) {
	return parseDate(value, now, false)
}

// ParseDateAllowingPast is ParseDate for lookups such as flight status, where past calendar dates are valid.
func ParseDateAllowingPast(value string, now time.Time) (time.Time, error) {
	return parseDate(value, now, true)
}

func parseDate(value string, now time.Time, allowPast bool) (time.Time, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch value {
	case "today", "hoy":
		return validateSupportedDate(value, today)
	case "tomorrow", "mañana", "manana":
		return validateSupportedDate(value, today.AddDate(0, 0, 1))
	case "yesterday", "ayer":
		if allowPast {
			return validateSupportedDate(value, today.AddDate(0, 0, -1))
		}
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
		return validateSupportedDate(value, today.AddDate(0, 0, days))
	}
	if strings.HasPrefix(value, "+") {
		days, err := strconv.Atoi(value[1:])
		if err != nil || days < 0 {
			return time.Time{}, fmt.Errorf("invalid relative date %q (use +N)", value)
		}
		if today.Year() > 9999 || int64(days) > (int64(9999)-int64(today.Year())+1)*366 {
			return time.Time{}, fmt.Errorf("relative date %q is outside the supported date range", value)
		}
		date := today.AddDate(0, 0, days)
		if date.Year() > 9999 {
			return time.Time{}, fmt.Errorf("relative date %q is outside the supported date range", value)
		}
		return date, nil
	}
	for _, layout := range []string{time.DateOnly, "02/01/2006"} {
		if parsed, err := time.ParseInLocation(layout, value, now.Location()); err == nil {
			if !allowPast && parsed.Before(today) {
				return time.Time{}, fmt.Errorf("date must be today or later")
			}
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date %q (use YYYY-MM-DD, DD/MM/YYYY, today, tomorrow, +N, or a weekday)", value)
}

func validateSupportedDate(value string, date time.Time) (time.Time, error) {
	if date.Year() > 9999 {
		return time.Time{}, fmt.Errorf("date %q is outside the supported date range", value)
	}
	return date, nil
}
