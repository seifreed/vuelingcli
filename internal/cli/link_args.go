package cli

import (
	"fmt"
	"time"

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
			requested, err := setOutputFormat(format, a)
			if err != nil {
				return flightLinkOptions{}, err
			}
			format = requested
			continue
		}
		if len(a) > 2 && a[:2] == "--" {
			key, value, next, err := parseLongOption(args, i)
			if err != nil {
				return flightLinkOptions{}, err
			}
			i = next
			switch key {
			case "date", "return", "adults", "children", "infants", "discount", "culture", "currency", "flight", "fare-basis", "return-flight", "return-fare-basis", "token":
				values[key] = value
			default:
				return flightLinkOptions{}, fmt.Errorf("unknown option --%s", key)
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
	now := time.Now()
	date, err := parseOptionalDate(values["date"], now)
	if err != nil {
		return flightLinkOptions{}, err
	}
	returnDate, err := parseOptionalDate(values["return"], now)
	if err != nil {
		return flightLinkOptions{}, err
	}
	adults, err := parseIntOption(values["adults"], 1)
	if err != nil {
		return flightLinkOptions{}, fmt.Errorf("invalid --adults: %w", err)
	}
	children, err := parseIntOption(values["children"], 0)
	if err != nil {
		return flightLinkOptions{}, fmt.Errorf("invalid --children: %w", err)
	}
	infants, err := parseIntOption(values["infants"], 0)
	if err != nil {
		return flightLinkOptions{}, fmt.Errorf("invalid --infants: %w", err)
	}
	discount, err := parseIntOption(values["discount"], 0)
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
