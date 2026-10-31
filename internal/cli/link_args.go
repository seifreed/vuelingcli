package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	dateparse "github.com/seifreed/vuelingcli/internal/date"
	"github.com/seifreed/vuelingcli/internal/vueling"
)

type flightLinkOptions struct {
	link   vueling.LinkOptions
	format string
	open   bool
}

func parseFlightLinkOptions(args []string) (flightLinkOptions, error) {
	values := map[string]string{}
	var positional []string
	format := ""
	open := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--open" {
			open = true
			continue
		}
		if a == "--json" || a == "--toon" {
			if format != "" {
				return flightLinkOptions{}, fmt.Errorf("--json and --toon cannot be used together")
			}
			format = a[2:]
			continue
		}
		if len(a) > 2 && a[:2] == "--" {
			k, v, hasValue := strings.Cut(a[2:], "=")
			if !hasValue {
				remaining := args[i:]
				if len(remaining) < 2 {
					return flightLinkOptions{}, fmt.Errorf("--%s requires a value", k)
				}
				v = remaining[1]
				i++
			}
			switch k {
			case "date", "return", "adults", "children", "infants", "discount", "culture", "currency", "flight", "fare-basis", "return-flight", "return-fare-basis", "token":
				values[k] = v
			default:
				return flightLinkOptions{}, fmt.Errorf("unknown option --%s", k)
			}
			continue
		}
		positional = append(positional, a)
	}
	if len(positional) != 2 || values["date"] == "" {
		return flightLinkOptions{}, fmt.Errorf("usage: vueling link <origin city or IATA> <destination city or IATA> --date <date> [options]")
	}
	origin, err := resolveAirport(positional[0])
	if err != nil {
		return flightLinkOptions{}, fmt.Errorf("origin: %w", err)
	}
	destination, err := resolveAirport(positional[1])
	if err != nil {
		return flightLinkOptions{}, fmt.Errorf("destination: %w", err)
	}
	parseDate := func(key string) (time.Time, error) {
		if values[key] == "" {
			return time.Time{}, nil
		}
		return dateparse.ParseDate(values[key], time.Now())
	}
	date, err := parseDate("date")
	if err != nil {
		return flightLinkOptions{}, err
	}
	returnDate, err := parseDate("return")
	if err != nil {
		return flightLinkOptions{}, err
	}
	integer := func(key string, fallback int) (int, error) {
		if values[key] == "" {
			return fallback, nil
		}
		return strconv.Atoi(values[key])
	}
	adults, err := integer("adults", 1)
	if err != nil {
		return flightLinkOptions{}, fmt.Errorf("invalid --adults: %w", err)
	}
	children, err := integer("children", 0)
	if err != nil {
		return flightLinkOptions{}, fmt.Errorf("invalid --children: %w", err)
	}
	infants, err := integer("infants", 0)
	if err != nil {
		return flightLinkOptions{}, fmt.Errorf("invalid --infants: %w", err)
	}
	discount, err := integer("discount", 0)
	if err != nil {
		return flightLinkOptions{}, fmt.Errorf("invalid --discount: %w", err)
	}
	return flightLinkOptions{
		link: vueling.LinkOptions{
			Origin: origin, Destination: destination, Date: date, ReturnDate: returnDate,
			Adults: adults, Children: children, Infants: infants, Discount: discount,
			Culture: values["culture"], Currency: values["currency"],
			OutboundFlight: values["flight"], OutboundFareBasis: values["fare-basis"],
			ReturnFlight: values["return-flight"], ReturnFareBasis: values["return-fare-basis"],
			METToken: values["token"],
		},
		format: format, open: open,
	}, nil
}
