package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/seifreed/vuelingcli/internal/usecase"
)

const selectUsage = "usage: vueling select <origin city or IATA> <destination city or IATA> --date <date> (--flight VY1234 | --cheapest) [--return <date> (--return-flight VY1234 | --return-cheapest)] [options]"

// parseSelectOptions reads how each direction's flight is picked and hands the passenger, date and
// currency options to the flights parser.
func parseSelectOptions(args []string) (flightOptions, usecase.FlightPick, usecase.FlightPick, error) {
	var rest []string
	var outbound, inbound usecase.FlightPick
	cheapest, returnCheapest, direct, positional := false, false, false, 0
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--cheapest" || a == "--return-cheapest" || a == "--direct":
			cheapest, returnCheapest, direct = cheapest || a == "--cheapest", returnCheapest || a == "--return-cheapest", direct || a == "--direct"
			continue
		case !strings.HasPrefix(a, "--") || a == "--json" || a == "--toon":
			if !strings.HasPrefix(a, "-") {
				positional++
			}
			rest = append(rest, a)
			continue
		}
		key, value, next, err := parseLongOption(args, i)
		if err != nil {
			return flightOptions{}, usecase.FlightPick{}, usecase.FlightPick{}, err
		}
		i = next
		switch key {
		case "flight":
			outbound = parseFlightPick(value)
		case "return-flight":
			inbound = parseFlightPick(value)
		case "date", "return", "adults", "children", "infants", "discount", "currency":
			rest = append(rest, "--"+key, value)
		default:
			return flightOptions{}, usecase.FlightPick{}, usecase.FlightPick{}, fmt.Errorf("unknown option --%s", key)
		}
	}
	if positional != 2 || (outbound.Flight != "") == cheapest {
		return flightOptions{}, usecase.FlightPick{}, usecase.FlightPick{}, errors.New(selectUsage)
	}
	if direct && !cheapest && !returnCheapest {
		return flightOptions{}, usecase.FlightPick{}, usecase.FlightPick{}, errors.New("--direct needs --cheapest or --return-cheapest")
	}
	options, err := parseFlightOptions(rest)
	if err != nil {
		return flightOptions{}, usecase.FlightPick{}, usecase.FlightPick{}, selectUsageError(err)
	}
	if err := checkReturnPick(options, inbound.Flight != "", returnCheapest); err != nil {
		return flightOptions{}, usecase.FlightPick{}, usecase.FlightPick{}, err
	}
	outbound.Direct, inbound.Direct = direct, direct
	return options, outbound, inbound, nil
}

// selectUsageError replaces the flights parser's usage line, which names the flights command, with select's.
func selectUsageError(err error) error {
	if errors.Is(err, errFlightUsage) {
		return errors.New(selectUsage)
	}
	return err
}

// checkReturnPick requires exactly one way of picking the return flight when, and only when, there is a return date.
func checkReturnPick(options flightOptions, returnFlight, returnCheapest bool) error {
	switch {
	case options.returnDate.IsZero() && (returnFlight || returnCheapest):
		return errors.New("--return-flight and --return-cheapest need --return")
	case options.returnDate.IsZero():
		return nil
	case returnFlight == returnCheapest:
		return errors.New(selectUsage)
	case options.returnDate.Before(options.date):
		return errors.New("--return must not be before --date")
	}
	return nil
}
