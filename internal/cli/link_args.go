package cli

import (
	"fmt"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

type flightLinkOptions struct {
	link   vueling.LinkOptions
	format string
	open   bool
}

func parseFlightLinkOptions(command string, args []string) (flightLinkOptions, error) {
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
	input, err := parseFlightInput(values, positional, fmt.Errorf("usage: vueling %s <origin city or IATA> <destination city or IATA> --date <date> [options]", command))
	if err != nil {
		return flightLinkOptions{}, err
	}
	discount, err := parseIntOption(values["discount"], 0)
	if err != nil {
		return flightLinkOptions{}, fmt.Errorf("invalid --discount: %w", err)
	}
	return flightLinkOptions{
		link: vueling.LinkOptions{
			Origin: input.origin, Destination: input.destination, Date: input.date, ReturnDate: input.returnDate,
			Adults: input.adults, Children: input.children, Infants: input.infants, Discount: discount,
			Culture: values["culture"], Currency: values["currency"],
			OutboundFlight: values["flight"], OutboundFareBasis: values["fare-basis"],
			ReturnFlight: values["return-flight"], ReturnFareBasis: values["return-fare-basis"],
			METToken: values["token"],
		},
		format: format, open: open,
	}, nil
}
