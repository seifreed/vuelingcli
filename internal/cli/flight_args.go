package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	dateparse "github.com/seifreed/vuelingcli/internal/date"
	"github.com/seifreed/vuelingcli/internal/vueling"
)

type flightOptions struct {
	origin, destination        string
	date, returnDate           time.Time
	adults, children, infants  int
	currency, endpoint, format string
	filter                     vueling.FlightFilter
	nearby                     bool
}

func parseFlightOptions(args []string) (flightOptions, error) {
	values := map[string]string{}
	var positional []string
	format := ""
	var filter vueling.FlightFilter
	nearby := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--json" || a == "--toon" {
			if format != "" {
				return flightOptions{}, fmt.Errorf("--json and --toon cannot be used together")
			}
			format = a[2:]
			continue
		}
		if a == "--nearby" {
			nearby = true
			continue
		}
		if a == "--cheapest" || a == "--direct" {
			filter.Cheapest = filter.Cheapest || a == "--cheapest"
			filter.Direct = filter.Direct || a == "--direct"
			continue
		}
		if strings.HasPrefix(a, "--") {
			k, v, hasValue := strings.Cut(a[2:], "=")
			if !hasValue {
				remaining := args[i:]
				if len(remaining) < 2 {
					return flightOptions{}, fmt.Errorf("--%s requires a value", k)
				}
				v = remaining[1]
				i++
			}
			switch k {
			case "date", "return", "adults", "children", "infants", "currency", "cdp-url", "after", "before", "return-after", "return-before", "limit":
				values[k] = v
			default:
				return flightOptions{}, fmt.Errorf("unknown option --%s", k)
			}
			continue
		}
		if strings.HasPrefix(a, "-") {
			return flightOptions{}, fmt.Errorf("unknown option %s", a)
		}
		positional = append(positional, a)
	}
	if len(positional) != 2 || values["date"] == "" {
		return flightOptions{}, fmt.Errorf("usage: vueling flights <origin city or IATA> <destination city or IATA> --date <date> [options]")
	}
	origin, err := resolveAirport(positional[0])
	if err != nil {
		return flightOptions{}, fmt.Errorf("origin: %w", err)
	}
	destination, err := resolveAirport(positional[1])
	if err != nil {
		return flightOptions{}, fmt.Errorf("destination: %w", err)
	}
	parseDate := func(key string) (time.Time, error) {
		if values[key] == "" {
			return time.Time{}, nil
		}
		return dateparse.ParseDate(values[key], time.Now())
	}
	date, err := parseDate("date")
	if err != nil {
		return flightOptions{}, err
	}
	returnDate, err := parseDate("return")
	if err != nil {
		return flightOptions{}, err
	}
	integer := func(key string, fallback int) (int, error) {
		if values[key] == "" {
			return fallback, nil
		}
		return strconv.Atoi(values[key])
	}
	adults, err := integer("adults", 1)
	if err != nil {
		return flightOptions{}, fmt.Errorf("invalid --adults: %w", err)
	}
	children, err := integer("children", 0)
	if err != nil {
		return flightOptions{}, fmt.Errorf("invalid --children: %w", err)
	}
	infants, err := integer("infants", 0)
	if err != nil {
		return flightOptions{}, fmt.Errorf("invalid --infants: %w", err)
	}
	if filter.Limit, err = integer("limit", 0); err != nil || (values["limit"] != "" && filter.Limit < 1) {
		return flightOptions{}, fmt.Errorf("invalid --limit: must be a whole number of at least 1")
	}
	clock := func(key string, fallback int) (int, error) {
		if values[key] == "" {
			return fallback, nil
		}
		minutes, err := vueling.ParseClock(values[key])
		if err != nil {
			return 0, fmt.Errorf("invalid --%s: %w", key, err)
		}
		return minutes, nil
	}
	afterMinutes, err := clock("after", 0)
	if err != nil {
		return flightOptions{}, err
	}
	beforeMinutes, err := clock("before", 24*60)
	if err != nil {
		return flightOptions{}, err
	}
	if afterMinutes > beforeMinutes {
		return flightOptions{}, fmt.Errorf("--after must not be later than --before")
	}
	returnAfterMinutes, err := clock("return-after", 0)
	if err != nil {
		return flightOptions{}, err
	}
	returnBeforeMinutes, err := clock("return-before", 24*60)
	if err != nil {
		return flightOptions{}, err
	}
	if returnAfterMinutes > returnBeforeMinutes {
		return flightOptions{}, fmt.Errorf("--return-after must not be later than --return-before")
	}
	if (values["return-after"] != "" || values["return-before"] != "") && values["return"] == "" {
		return flightOptions{}, fmt.Errorf("--return-after and --return-before need --return")
	}
	filter.After, filter.Before = values["after"], values["before"]
	filter.ReturnAfter, filter.ReturnBefore = values["return-after"], values["return-before"]
	currency := strings.ToUpper(values["currency"])
	if currency == "" {
		currency = "EUR"
	}
	return flightOptions{
		origin: origin, destination: destination, date: date, returnDate: returnDate,
		adults: adults, children: children, infants: infants, currency: currency,
		endpoint: values["cdp-url"], filter: filter, nearby: nearby, format: format,
	}, nil
}
