package vueling

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CalendarDay is one rendered day fare of a price-calendar month.
type CalendarDay struct {
	Date string `json:"date"`
	Fare string `json:"fare"`
}

// CalendarMonth holds the requested fares and the visible day fares of each month.
type CalendarMonth struct {
	Price      map[string]string
	Days       []CalendarDay
	ReturnDays []CalendarDay
}

type renderedDay struct {
	Day  string `json:"day"`
	Fare string `json:"fare"`
}

var fareAmountPattern = regexp.MustCompile(`[0-9]+(?:[.,][0-9]+)*`)

// CalendarURL builds Vueling's public price-calendar URL from airport codes.
func CalendarURL(origin, destination string, date time.Time) (string, error) {
	origin, destination = strings.ToUpper(strings.TrimSpace(origin)), strings.ToUpper(strings.TrimSpace(destination))
	if !validIATACode(origin) || !validIATACode(destination) || origin == destination {
		return "", fmt.Errorf("origin and destination must be different three-letter IATA codes")
	}
	if date.IsZero() {
		return "", fmt.Errorf("date is required")
	}
	query := url.Values{
		"originCalendar":      {origin},
		"destinationCalendar": {destination},
		"dateComplete":        {date.Format("02/01/2006")},
	}
	return "https://www.vueling.com/es/calendario-de-precios?" + query.Encode(), nil
}

// monthDays dates rendered day fares within the month of reference (DD/MM/YYYY), sorted by day.
func monthDays(rendered []renderedDay, reference string) []CalendarDay {
	if len(reference) != len("02/01/2006") {
		return nil
	}
	seen := map[string]bool{}
	var days []CalendarDay
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
		days = append(days, CalendarDay{Date: date, Fare: day.Fare})
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Date < days[j].Date })
	return days
}

// ParseFare reads the amount of a displayed fare such as "49,99 €" or "1.234,50 EUR".
func ParseFare(text string) (float64, bool) {
	amount := fareAmountPattern.FindString(text)
	if amount == "" {
		return 0, false
	}
	digits := strings.NewReplacer(".", "", ",", "")
	if i := strings.LastIndexAny(amount, ".,"); i >= 0 && len(amount)-i-1 <= 2 {
		amount = digits.Replace(amount[:i]) + "." + amount[i+1:]
	} else {
		amount = digits.Replace(amount)
	}
	value, err := strconv.ParseFloat(amount, 64)
	return value, err == nil
}

// CheapestDay returns the earliest day with the lowest parseable fare.
func CheapestDay(days []CalendarDay) (CalendarDay, bool) {
	var cheapest CalendarDay
	best, found := 0.0, false
	for _, day := range days {
		if value, ok := ParseFare(day.Fare); ok && (!found || value < best) {
			cheapest, best, found = day, value, true
		}
	}
	return cheapest, found
}
