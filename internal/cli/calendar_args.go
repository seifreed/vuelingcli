package cli

import (
	"fmt"
	"time"
)

type calendarOptions struct {
	origin, destination string
	date                time.Time
	returnDate, format  string
	open, prices, month bool
}

func parseCalendarOptions(args []string, now time.Time) (calendarOptions, error) {
	var positional []string
	dateValue, returnValue, format := "", "", ""
	open, prices, month := false, false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--open":
			open = true
		case a == "--prices":
			prices = true
		case a == "--month":
			month = true
		case a == "--date" || a == "--return" || (len(a) > 9 && a[:9] == "--return=") || (len(a) > 7 && a[:7] == "--date="):
			key, value, next, err := parseLongOption(args, i)
			if err != nil {
				return calendarOptions{}, err
			}
			i = next
			if key == "date" {
				dateValue = value
			} else {
				returnValue = value
			}
		case a == "--json" || a == "--toon":
			requested, err := setOutputFormat(format, a)
			if err != nil {
				return calendarOptions{}, err
			}
			format = requested
		case len(a) > 0 && a[0] == '-':
			return calendarOptions{}, fmt.Errorf("unknown option %s", a)
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) != 2 || dateValue == "" {
		return calendarOptions{}, fmt.Errorf("usage: vueling calendar <origin city or IATA> <destination city or IATA> --date <date> [--return <date>] [--prices [--month]] [--open] [--json|--toon]")
	}
	if month && !prices {
		return calendarOptions{}, fmt.Errorf("--month requires --prices")
	}
	origin, err := resolveAirport(positional[0])
	if err != nil {
		return calendarOptions{}, fmt.Errorf("origin: %w", err)
	}
	destination, err := resolveAirport(positional[1])
	if err != nil {
		return calendarOptions{}, fmt.Errorf("destination: %w", err)
	}
	date, err := parseOptionalDate(dateValue, now)
	if err != nil {
		return calendarOptions{}, err
	}
	var returnDate string
	parsed, err := parseOptionalDate(returnValue, now)
	if err != nil {
		return calendarOptions{}, err
	}
	if returnValue != "" {
		if parsed.Before(date) {
			return calendarOptions{}, fmt.Errorf("return date must be on or after departure date")
		}
		if parsed.After(date.AddDate(0, 0, 330)) {
			return calendarOptions{}, fmt.Errorf("return date must be within 330 days of departure for calendar price lookup")
		}
		returnDate = parsed.Format("02/01/2006")
	}
	return calendarOptions{
		origin: origin, destination: destination, date: date, returnDate: returnDate,
		format: format, open: open, prices: prices, month: month,
	}, nil
}
