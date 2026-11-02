package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/usecase"
	"github.com/seifreed/vuelingcli/internal/vueling"
)

var errFlightUsage = errors.New("usage: vueling flights <origin city or IATA> <destination city or IATA> --date <date> [options]")

type flightInput struct {
	origin, destination       string
	date, returnDate          time.Time
	adults, children, infants int
}

type flightOptions struct {
	flightInput
	discount                   int
	currency, endpoint, format string
	filter                     vueling.FlightFilter
	nearby, browser            bool
}

func parseFlightInput(values map[string]string, positional []string, usage error) (flightInput, error) {
	if len(positional) != 2 || values["date"] == "" {
		return flightInput{}, usage
	}
	origin, err := resolveAirport(positional[0])
	if err != nil {
		return flightInput{}, fmt.Errorf("origin: %w", err)
	}
	destination, err := resolveAirport(positional[1])
	if err != nil {
		return flightInput{}, fmt.Errorf("destination: %w", err)
	}
	now := time.Now()
	date, err := parseOptionalDate(values["date"], now)
	if err != nil {
		return flightInput{}, err
	}
	returnDate, err := parseOptionalDate(values["return"], now)
	if err != nil {
		return flightInput{}, err
	}
	adults, err := parseIntOption(values["adults"], 1)
	if err != nil {
		return flightInput{}, fmt.Errorf("invalid --adults: %w", err)
	}
	children, err := parseIntOption(values["children"], 0)
	if err != nil {
		return flightInput{}, fmt.Errorf("invalid --children: %w", err)
	}
	infants, err := parseIntOption(values["infants"], 0)
	if err != nil {
		return flightInput{}, fmt.Errorf("invalid --infants: %w", err)
	}
	if err := vueling.ValidatePassengerCounts(adults, children, infants); err != nil {
		return flightInput{}, err
	}
	if !returnDate.IsZero() && returnDate.Before(date) {
		return flightInput{}, errors.New("return date must be on or after departure date")
	}
	return flightInput{
		origin: origin, destination: destination, date: date, returnDate: returnDate,
		adults: adults, children: children, infants: infants,
	}, nil
}

func (options flightOptions) flightQuery() usecase.FlightQuery {
	return usecase.FlightQuery{
		Origin: options.origin, Destination: options.destination, Date: options.date, ReturnDate: options.returnDate,
		Adults: options.adults, Children: options.children, Infants: options.infants, Discount: options.discount, Currency: options.currency,
	}
}

func (options flightOptions) linkOptions() vueling.LinkOptions {
	return vueling.LinkOptions{
		Origin: options.origin, Destination: options.destination, Date: options.date, ReturnDate: options.returnDate,
		Adults: options.adults, Children: options.children, Infants: options.infants, Discount: options.discount, Currency: options.currency,
	}
}

func parseFlightPick(value string) usecase.FlightPick {
	return usecase.FlightPick{Flight: strings.ToUpper(strings.ReplaceAll(value, " ", ""))}
}

func parseFlightOptions(args []string) (flightOptions, error) {
	values := map[string]string{}
	var positional []string
	format := ""
	var filter vueling.FlightFilter
	nearby, browser := false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--json" || a == "--toon" {
			requested, err := setOutputFormat(format, a)
			if err != nil {
				return flightOptions{}, err
			}
			format = requested
			continue
		}
		if a == "--nearby" || a == "--browser" {
			nearby = nearby || a == "--nearby"
			browser = browser || a == "--browser"
			continue
		}
		if a == "--cheapest" || a == "--direct" {
			filter.Cheapest = filter.Cheapest || a == "--cheapest"
			filter.Direct = filter.Direct || a == "--direct"
			continue
		}
		if strings.HasPrefix(a, "--") {
			key, value, next, err := parseLongOption(args, i)
			if err != nil {
				return flightOptions{}, err
			}
			i = next
			switch key {
			case "date", "return", "adults", "children", "infants", "discount", "currency", "cdp-url", "after", "before", "return-after", "return-before", "limit":
				values[key] = value
			default:
				return flightOptions{}, fmt.Errorf("unknown option --%s", key)
			}
			continue
		}
		if strings.HasPrefix(a, "-") {
			return flightOptions{}, fmt.Errorf("unknown option %s", a)
		}
		positional = append(positional, a)
	}
	input, err := parseFlightInput(values, positional, errFlightUsage)
	if err != nil {
		return flightOptions{}, err
	}
	discount, err := parseIntOption(values["discount"], 0)
	if err != nil || discount < 0 || discount > 5 {
		return flightOptions{}, fmt.Errorf("invalid --discount: must be 0 (none) to 5")
	}
	if filter.Limit, err = parseIntOption(values["limit"], 0); err != nil || (values["limit"] != "" && filter.Limit < 1) {
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
	currency, err := vueling.NormalizeCurrency(values["currency"])
	if err != nil {
		return flightOptions{}, fmt.Errorf("invalid --currency: %w", err)
	}
	if discount > 0 && currency != "EUR" {
		return flightOptions{}, fmt.Errorf("--discount needs EUR, the only currency Vueling applies Spanish discounts in")
	}
	return flightOptions{
		flightInput: input,
		discount:    discount, currency: currency,
		endpoint: values["cdp-url"], filter: filter, nearby: nearby, browser: browser || values["cdp-url"] != "", format: format,
	}, nil
}
