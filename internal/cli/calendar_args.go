package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

type calendarOptions struct {
	origin, destination string
	date                time.Time
	returnDate, format  string
	currency            string
	currencySet         bool
	open, prices, month bool
	browser             bool
	watch               int
	below               float64
}

func parseCalendarOptions(args []string, now time.Time) (calendarOptions, error) {
	var positional []string
	dateValue, returnValue, format, currencyValue, watchValue, belowValue := "", "", "", "", "", ""
	currencySet := false
	open, prices, month, browser := false, false, false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--open":
			open = true
		case a == "--prices":
			prices = true
		case a == "--month":
			month = true
		case a == "--browser":
			browser = true
		case isCalendarValueOption(a):
			key, value, next, err := parseLongOption(args, i)
			if err != nil {
				return calendarOptions{}, err
			}
			i = next
			switch key {
			case "date":
				dateValue = value
			case "return":
				returnValue = value
			case "watch":
				watchValue = value
			case "below":
				belowValue = value
			case "currency":
				currencyValue = value
				currencySet = true
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
		return calendarOptions{}, fmt.Errorf("usage: vueling calendar <origin city or IATA> <destination city or IATA> --date <date> [--return <date>] [--prices [--month] [--currency code | --browser] [--watch <minutes> [--below <price>]]] [--open] [--json|--toon]")
	}
	if month && !prices {
		return calendarOptions{}, fmt.Errorf("--month requires --prices")
	}
	if browser && !prices {
		return calendarOptions{}, fmt.Errorf("--browser requires --prices")
	}
	if currencySet && (!prices || browser) {
		return calendarOptions{}, fmt.Errorf("--currency requires --prices and cannot be used with --browser")
	}
	watch, below, err := parseCalendarWatch(watchValue, belowValue, format, prices, month, browser)
	if err != nil {
		return calendarOptions{}, err
	}
	currency, err := vueling.NormalizeCurrency(currencyValue)
	if err != nil {
		return calendarOptions{}, err
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
		format: format, open: open, prices: prices, month: month, browser: browser, currency: currency, currencySet: currencySet,
		watch: watch, below: below,
	}, nil
}

func isCalendarValueOption(arg string) bool {
	for _, name := range []string{"--date", "--return", "--currency", "--watch", "--below"} {
		if arg == name || strings.HasPrefix(arg, name+"=") {
			return true
		}
	}
	return false
}

// parseCalendarWatch validates --watch and --below; fares change slowly, so the interval is at least 30 minutes.
func parseCalendarWatch(watchValue, belowValue, format string, prices, month, browser bool) (watch int, below float64, err error) {
	if belowValue != "" && watchValue == "" {
		return 0, 0, fmt.Errorf("--below requires --watch")
	}
	if watchValue == "" {
		return 0, 0, nil
	}
	switch {
	case !prices || browser:
		return 0, 0, fmt.Errorf("--watch requires --prices and cannot be used with --browser")
	case month:
		return 0, 0, fmt.Errorf("--watch cannot be used with --month")
	case format == "toon":
		return 0, 0, fmt.Errorf("--watch cannot be used with --toon")
	}
	if watch, err = parseIntOption(watchValue, 0); err != nil || watch < minCalendarWatchMinutes {
		return 0, 0, fmt.Errorf("--watch must be a whole number of minutes, at least %d", minCalendarWatchMinutes)
	}
	if int64(watch) > maxWatchMinutes {
		return 0, 0, fmt.Errorf("--watch interval is too long")
	}
	if belowValue != "" {
		below, err = parsePositivePrice(belowValue, "--below")
		if err != nil {
			return 0, 0, err
		}
	}
	return watch, below, nil
}
