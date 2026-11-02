package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/seifreed/vuelingcli/internal/usecase"
	"github.com/seifreed/vuelingcli/internal/vueling"
)

func (cli *CLI) trips(args []string) error {
	if isHelp(args) {
		_, err := fmt.Fprint(cli.stdout, "Find the cheapest departure and return days for a trip of a few nights\n\nUsage:\n  vueling trips <origin city or IATA> <destination city or IATA> --nights N[-M] [--from <date>] [--to <date>] [--max price] [--limit N] [--currency code] [--json|--toon]\n\nPairs each departure day from --from (default today) to --to (default --from + 30 days) with the return\nday N to M nights later, using the cheapest one-way calendar fare of each day per adult, cheapest total first.\n--limit defaults to 10 (0 lists every combination); --max keeps totals up to that price.\n")
		return err
	}
	options, err := parseTripsOptions(args, time.Now())
	if err != nil {
		return err
	}
	if err := cli.requireAPI(); err != nil {
		return err
	}
	trips, err := usecase.FindTrips(context.Background(), cli.api, vueling.TripSearch{
		Origin: options.origin, Destination: options.destination,
		From: options.from, To: options.to,
		MinNights: options.minNights, MaxNights: options.maxNights,
		MaxPrice: options.maxPrice, Limit: options.limit, Currency: options.currency,
	})
	if err := tripResultError(err, trips, options.origin, options.destination); err != nil {
		return err
	}
	if options.format != "" {
		return cli.writeStructured(trips, options.format)
	}
	for _, trip := range trips {
		if _, err := fmt.Fprintf(cli.stdout, "%s → %s  %d %s  out %s + ret %s = %s %s\n", trip.Departure, trip.Return, trip.Nights, nightsLabel(trip.Nights), vueling.FormatCompactAmount(trip.Outbound), vueling.FormatCompactAmount(trip.ReturnFare), vueling.FormatCompactAmount(trip.Total), trip.Currency); err != nil {
			return err
		}
	}
	return nil
}

func tripResultError(err error, trips []vueling.Trip, origin, destination string) error {
	if err != nil {
		return err
	}
	if len(trips) == 0 {
		return fmt.Errorf("Vueling shows no %s–%s trips matching those dates and nights", origin, destination)
	}
	return nil
}

func nightsLabel(nights int) string {
	if nights == 1 {
		return "noche"
	}
	return "noches"
}
