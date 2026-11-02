package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/airports"
	"github.com/seifreed/vuelingcli/internal/usecase"
	"github.com/seifreed/vuelingcli/internal/vueling"
)

type dealsOptions struct {
	origin       string
	destinations []string
	from         time.Time
	months       int
	maxPrice     float64
	limit        int
	direct       bool
	currency     string
	format       string
}

func (cli *CLI) deals(args []string) error {
	if isHelp(args) {
		_, err := fmt.Fprint(cli.stdout, "Find the cheapest Vueling fare to every destination from an origin\n\nUsage:\n  vueling deals <origin city or IATA> [--to city-or-IATA[,…]] [--from <date>] [--months 1-12] [--max price] [--direct] [--limit N] [--currency code] [--json|--toon]\n\nReads the best-price API behind Vueling's \"Ofertas desde\" section: one cheapest one-way fare per destination\nwithin --months (default 4) from --from (default today), cheapest first.\n")
		return err
	}
	options, err := parseDealsOptions(args, time.Now())
	if err != nil {
		return err
	}
	if err := cli.requireAPI(); err != nil {
		return err
	}
	deals, err := usecase.FindDeals(context.Background(), cli.api, usecase.DealSearch{
		Origin: options.origin, Destinations: options.destinations, From: options.from, Months: options.months,
		MaxPrice: options.maxPrice, Limit: options.limit, Direct: options.direct, Currency: options.currency,
	})
	if err != nil {
		return err
	}
	if len(deals) == 0 {
		return fmt.Errorf("Vueling shows no deals from %s matching those options", options.origin)
	}
	if options.format != "" {
		return cli.writeStructured(deals, options.format)
	}
	for _, deal := range deals {
		if _, err := fmt.Fprintf(cli.stdout, "%-4s %-26s %s %s\n", deal.Destination, deal.City, deal.Departure.Format("02/01/2006"), vueling.FormatCompactAmount(deal.Price)+" "+deal.Currency); err != nil {
			return err
		}
	}
	return nil
}

func parseDealsOptions(args []string, now time.Time) (dealsOptions, error) {
	options := dealsOptions{months: 4}
	var positional []string
	values := map[string]string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--direct":
			options.direct = true
		case a == "--json" || a == "--toon":
			format, err := setOutputFormat(options.format, a)
			if err != nil {
				return dealsOptions{}, err
			}
			options.format = format
		case strings.HasPrefix(a, "--"):
			key, value, next, err := parseLongOption(args, i)
			if err != nil {
				return dealsOptions{}, err
			}
			switch key {
			case "to", "from", "months", "max", "limit", "currency":
				values[key], i = value, next
			default:
				return dealsOptions{}, fmt.Errorf("unknown option --%s", key)
			}
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) != 1 {
		return dealsOptions{}, errors.New("usage: vueling deals <origin> [--to destinations] [--from date] [--months 1-12] [--max price] [--direct] [--limit N]")
	}
	var err error
	if options.origin, err = resolveAirport(positional[0]); err != nil {
		return dealsOptions{}, err
	}
	if options.destinations, err = dealDestinations(values["to"], options.origin); err != nil {
		return dealsOptions{}, err
	}
	options.from = now
	if values["from"] != "" {
		if options.from, err = parseOptionalDate(values["from"], now); err != nil {
			return dealsOptions{}, fmt.Errorf("invalid --from: %w", err)
		}
	}
	if options.months, err = parseIntOption(values["months"], 4); err != nil || options.months < 1 || options.months > 12 {
		return dealsOptions{}, errors.New("--months must be a whole number from 1 to 12")
	}
	if options.limit, err = parseNonNegativeLimit(values["limit"], 0); err != nil {
		return dealsOptions{}, err
	}
	if values["max"] != "" {
		if options.maxPrice, err = parsePositivePrice(values["max"], "--max"); err != nil {
			return dealsOptions{}, err
		}
	}
	if options.currency, err = vueling.NormalizeCurrency(values["currency"]); err != nil {
		return dealsOptions{}, err
	}
	return options, nil
}

// dealDestinations defaults to every catalogued station; Vueling drops the ones it does not fly to from origin.
func dealDestinations(value, origin string) ([]string, error) {
	var codes []string
	if value == "" {
		for _, airport := range airports.Search("") {
			if airport.Code != origin {
				codes = append(codes, airport.Code)
			}
		}
		return codes, nil
	}
	for _, name := range strings.Split(value, ",") {
		code, err := resolveAirport(name)
		if err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, nil
}
