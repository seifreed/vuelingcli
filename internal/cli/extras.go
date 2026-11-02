package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/seifreed/vuelingcli/internal/usecase"
	"github.com/seifreed/vuelingcli/internal/vueling"
)

const extrasUsage = "usage: vueling extras <origin city or IATA> <destination city or IATA> --date <date> (--flight VY1234 | --cheapest) [options]"

const extrasNotice = "Prices are for this flight only. Nothing was booked: the flight was put in a temporary anonymous cart, with no passengers and no payment, and that session was ended."

func (cli *CLI) extras(args []string) error {
	if isHelp(args) {
		_, err := fmt.Fprint(cli.stdout, "Read the exact seat, bag and extra-service prices Vueling offers for one flight\n\nUsage:\n  vueling extras <origin city or IATA> <destination city or IATA> --date <date> (--flight VY1234 | --cheapest) [--bundle CODE] [--adults N] [--children N] [--infants N] [--currency code] [--json|--toon]\n\nShows each seat type's price range and how many of its seats are free, checked and cabin bag prices by weight, and the prices of sports equipment, pets, the Flex Pack and other extras. Prices are per passenger unless marked as sold for all passengers. Seats are priced for the first passenger; lap infants take no seat.\n\nVueling only prices extras for a flight in a cart, so this command opens its own anonymous session, puts the flight's cheapest fare in that session's cart, reads the prices and ends the session, even on error. It never adds passengers, contact details, seats or services, never pays and never books. --bundle picks the fare bundle the prices assume (default VYBA, Fly Light; VCB2 Fly, VCB1 Fly 25 kg, VPRO Fly Pro, VCB3 Fly Grande), since bundles include some extras.\n")
		return err
	}
	options, pick, bundle, err := parseExtrasOptions(args)
	if err != nil {
		return err
	}
	if err := cli.requireAPI(); err != nil {
		return err
	}
	extras, err := cli.api.Extras(context.Background(), options.flightQuery(), pick, bundle)
	if err != nil {
		return err
	}
	if options.format != "" {
		return cli.writeStructured(map[string]any{"extras": extras, "booked": false, "sessionEnded": true, "note": extrasNotice}, options.format)
	}
	return cli.printExtras(extras)
}

func (cli *CLI) printExtras(extras vueling.Extras) error {
	bundle, ok := vueling.BundleName(extras.Bundle)
	if !ok {
		bundle = extras.Bundle
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s %s → %s, with %s (%s), prices in %s\n", extras.Flight, extras.Date, extras.Origin, extras.Destination, bundle, extras.Bundle, extras.Currency)
	writeExtraSection(&b, "Seats", extras.Seats)
	writeExtraSection(&b, "Bags", extras.Bags)
	writeExtraSection(&b, "Other extras", extras.Services)
	b.WriteString(extrasNotice + "\n")
	_, err := fmt.Fprint(cli.stdout, b.String())
	return err
}

func writeExtraSection(b *strings.Builder, title string, prices []vueling.ExtraPrice) {
	if len(prices) == 0 {
		fmt.Fprintf(b, "%s: none offered\n", title)
		return
	}
	fmt.Fprintf(b, "%s:\n", title)
	for _, price := range prices {
		fmt.Fprintf(b, "  %s\n", extraLine(price))
	}
}

// extraLine prints one extra, such as "Front 9.00–14.00, 36 free [front]" or "Flex Pack 14.00, sold for all passengers [FXPK]".
func extraLine(price vueling.ExtraPrice) string {
	line := price.Name
	if price.Segment != "" {
		line = price.Segment + " " + line
	}
	line += " " + vueling.FormatAmount(price.Price)
	if price.MaxPrice > price.Price {
		line += "–" + vueling.FormatAmount(price.MaxPrice)
	}
	if !price.PerPassenger {
		line += ", sold for all passengers"
	}
	if price.Available != nil {
		line += fmt.Sprintf(", %d free", *price.Available)
	}
	return line + " [" + price.Code + "]"
}

// parseExtrasOptions reads how the flight and bundle are picked and hands the date, passenger and currency
// options to the flights parser.
func parseExtrasOptions(args []string) (flightOptions, usecase.FlightPick, string, error) {
	var rest []string
	var pick usecase.FlightPick
	bundle, cheapest, positional := vueling.DefaultExtrasBundle, false, 0
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--cheapest":
			cheapest = true
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
			return flightOptions{}, usecase.FlightPick{}, "", err
		}
		i = next
		switch key {
		case "flight":
			pick = parseFlightPick(value)
		case "bundle":
			bundle = strings.ToUpper(value)
		case "date", "adults", "children", "infants", "currency":
			rest = append(rest, "--"+key, value)
		default:
			return flightOptions{}, usecase.FlightPick{}, "", fmt.Errorf("unknown option --%s", key)
		}
	}
	if positional != 2 || (pick.Flight != "") == cheapest {
		return flightOptions{}, usecase.FlightPick{}, "", errors.New(extrasUsage)
	}
	if _, ok := vueling.BundleName(bundle); !ok {
		return flightOptions{}, usecase.FlightPick{}, "", fmt.Errorf("unknown --bundle %s: use VYBA, VCB2, VCB1, VPRO or VCB3", bundle)
	}
	options, err := parseFlightOptions(rest)
	if err != nil {
		if errors.Is(err, errFlightUsage) {
			return flightOptions{}, usecase.FlightPick{}, "", errors.New(extrasUsage)
		}
		return flightOptions{}, usecase.FlightPick{}, "", err
	}
	return options, pick, bundle, nil
}
