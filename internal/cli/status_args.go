package cli

import (
	"fmt"
	"strconv"
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
}

func parseFlightStatusOptions(args []string) (flightStatusOptions, error) {
	var positional []string
	dateValue, endpoint, format, watch := "today", "", "", 0
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--json" || a == "--toon" {
			if format != "" {
				return flightStatusOptions{}, fmt.Errorf("--json and --toon cannot be used together")
			}
			format = a[2:]
			continue
		}
		if !strings.HasPrefix(a, "--") {
			positional = append(positional, a)
			continue
		}
		k, v, hasValue := strings.Cut(a[2:], "=")
		if !hasValue {
			remaining := args[i:]
			if len(remaining) < 2 {
				return flightStatusOptions{}, fmt.Errorf("--%s requires a value", k)
			}
			v = remaining[1]
			i++
		}
		switch k {
		case "date":
			dateValue = v
		case "cdp-url":
			endpoint = v
		case "watch":
			n, err := strconv.Atoi(v)
			if err != nil || int64(n) < 5 || int64(n) > maxWatchMinutes {
				return flightStatusOptions{}, fmt.Errorf("--watch must be a whole number of minutes between 5 and %d", maxWatchMinutes)
			}
			watch = n
		default:
			return flightStatusOptions{}, fmt.Errorf("unknown option --%s", k)
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
	date, err := dateparse.ParseDate(dateValue, time.Now())
	if err != nil {
		return flightStatusOptions{}, fmt.Errorf("invalid --date: %w", err)
	}
	return flightStatusOptions{
		flight: flight, origin: origin, destination: destination, date: date,
		endpoint: endpoint, format: format, watch: watch,
	}, nil
}
