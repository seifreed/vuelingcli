package search

import (
	"fmt"
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
	if strings.HasPrefix(value, "+") {
		var days int
		if _, err := fmt.Sscanf(value, "+%d", &days); err != nil || days < 0 {
			return time.Time{}, fmt.Errorf("invalid relative date %q (use +N)", value)
		}
		return today.AddDate(0, 0, days), nil
	}
	for _, layout := range []string{time.DateOnly, "02/01/2006"} {
		if parsed, err := time.ParseInLocation(layout, value, now.Location()); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date %q (use YYYY-MM-DD, DD/MM/YYYY, today, tomorrow, or +N)", value)
}
