package cli

import (
	"fmt"
	"strings"
	"time"

	dateparse "github.com/seifreed/vuelingcli/internal/date"
)

const maxWatchMinutes = int64((1<<63 - 1) / int64(time.Minute))

type flightStatusOptions struct {
	flight, origin, destination string
	date                        time.Time
	endpoint, format            string
	watch                       int
	browser                     bool
}

func parseFlightStatusOptions(args []string) (flightStatusOptions, error) {
	var positional []string
	dateValue, endpoint, format, watch, browser := "today", "", "", 0, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--browser" {
			browser = true
			continue
		}
		if a == "--json" || a == "--toon" {
			requested, err := setOutputFormat(format, a)
			if err != nil {
				return flightStatusOptions{}, err
			}
			format = requested
			continue
		}
		if !strings.HasPrefix(a, "--") {
			positional = append(positional, a)
			continue
		}
		key, value, next, err := parseLongOption(args, i)
		if err != nil {
			return flightStatusOptions{}, err
		}
		i = next
		switch key {
		case "date":
			dateValue = value
		case "cdp-url":
			endpoint, browser = value, true
		case "watch":
			n, err := parseIntOption(value, 0)
			if err != nil || n < 5 {
				return flightStatusOptions{}, fmt.Errorf("--watch must be a whole number of minutes, at least 5")
			}
			if int64(n) > maxWatchMinutes {
				return flightStatusOptions{}, fmt.Errorf("--watch interval is too long")
			}
			watch = n
		default:
			return flightStatusOptions{}, fmt.Errorf("unknown option --%s", key)
		}
	}
	if watch > 0 && format == "toon" {
		return flightStatusOptions{}, fmt.Errorf("--watch cannot be used with --toon")
	}
	flight, origin, destination := "", "", ""
	switch len(positional) {
	case 1:
		flight = strings.ToUpper(positional[0])
	case 2:
		var err error
		if origin, err = resolveAirport(positional[0]); err != nil {
			return flightStatusOptions{}, err
		}
		if destination, err = resolveAirport(positional[1]); err != nil {
			return flightStatusOptions{}, err
		}
	default:
		return flightStatusOptions{}, fmt.Errorf("usage: vueling status <VY flight number> | <origin> <destination> [--date <date>]")
	}
	parseDate := dateparse.ParseDateAllowingPast
	if browser {
		parseDate = dateparse.ParseDate
	}
	date, err := parseDate(dateValue, time.Now())
	if err != nil {
		return flightStatusOptions{}, fmt.Errorf("invalid --date: %w", err)
	}
	return flightStatusOptions{
		flight: flight, origin: origin, destination: destination, date: date,
		endpoint: endpoint, format: format, watch: watch, browser: browser,
	}, nil
}
