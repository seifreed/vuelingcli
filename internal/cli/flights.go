package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

func (cli *CLI) flights(args []string) error {
	if isHelp(args) {
		if _, err := fmt.Fprint(cli.stdout, "Read live flight offers from your Chrome session\n\nUsage:\n  vueling flights <origin city or IATA> <destination city or IATA> --date <date> [--return <date>] [--adults N] [--children N] [--infants N] [--currency code] [--cdp-url http://127.0.0.1:9222] [--cheapest] [--direct] [--after HH:MM] [--before HH:MM] [--return-after HH:MM] [--return-before HH:MM] [--limit N] [--nearby] [--json|--toon]\n\nFilters, sorting, and limits apply to outbound and return offers independently. --after and --before are inclusive departure times; --return-after and --return-before replace them for return offers. --nearby also reads the price calendar and reports the cheapest day of the outbound (and return) month.\n\nOn macOS, enable View > Developer > Allow JavaScript from Apple Events in Chrome. Other systems need a local Chrome DevTools endpoint. This command reads rendered offers only; it never selects a fare.\n"); err != nil {
			return err
		}
		return nil
	}
	options, err := parseFlightOptions(args)
	if err != nil {
		return err
	}
	searchURL, err := vueling.FlightSearchURL(vueling.LinkOptions{Origin: options.origin, Destination: options.destination, Date: options.date, ReturnDate: options.returnDate, Adults: options.adults, Children: options.children, Infants: options.infants, Currency: options.currency})
	if err != nil {
		return err
	}
	reader := cli.newReader(options.endpoint)
	flights, err := web.FlightsFromPage(context.Background(), reader, searchURL, options.date.Format(time.DateOnly), options.returnDate.Format(time.DateOnly))
	if err != nil {
		return err
	}
	flights = vueling.FilterFlights(flights, options.filter)
	result := map[string]any{"url": searchURL, "flights": flights}
	var outboundNearby, returnNearby *vueling.CalendarDay
	if options.nearby {
		outboundNearby, returnNearby = cli.nearbyFares(reader, options.origin, options.destination, options.date, options.returnDate)
		if outboundNearby != nil {
			result["nearby"] = outboundNearby
		}
		if returnNearby != nil {
			result["returnNearby"] = returnNearby
		}
	}
	if options.format != "" {
		return cli.writeStructured(result, options.format)
	}
	if _, err := fmt.Fprintf(cli.stdout, "%s\n", searchURL); err != nil {
		return err
	}
	for _, flight := range flights {
		if _, err := fmt.Fprintf(cli.stdout, "%-8s %s %s %s–%s %s %s | %s%s\n", strings.ToUpper(flight.Direction), flight.Date, flight.Flight, flight.Departure, flight.Arrival, flight.Fare, flight.Currency, connectionSummary(flight), flight.Itinerary); err != nil {
			return err
		}
	}
	if outboundNearby != nil {
		if _, err := fmt.Fprintf(cli.stdout, "cheapest nearby: %s from %s\n", outboundNearby.Date, outboundNearby.Fare); err != nil {
			return err
		}
	}
	if returnNearby != nil {
		if _, err := fmt.Fprintf(cli.stdout, "cheapest nearby return: %s from %s\n", returnNearby.Date, returnNearby.Fare); err != nil {
			return err
		}
	}
	return nil
}

// connectionSummary renders a connecting flight's legs, such as
// "1 escala vía BCN · VY1383+VY1050 | ", and nothing for direct flights.

func connectionSummary(flight vueling.Flight) string {
	if flight.Stops == 0 {
		return ""
	}
	summary := fmt.Sprintf("%d escala", flight.Stops)
	if flight.Stops > 1 {
		summary += "s"
	}
	if len(flight.Via) > 0 {
		summary += " vía " + strings.Join(flight.Via, ", ")
	}
	if len(flight.Flights) > 1 {
		summary += " · " + strings.Join(flight.Flights, "+")
	}
	return summary + " | "
}

// nearbyFares reads the cheapest calendar day of the outbound and return months,
// warning on stderr instead of failing because the flight offers are the primary output.

func (cli *CLI) nearbyFares(reader web.PageReader, origin, destination string, date, returnDate time.Time) (*vueling.CalendarDay, *vueling.CalendarDay) {
	link, err := vueling.CalendarURL(origin, destination, date)
	if err != nil {
		_, _ = fmt.Fprintf(cli.stderr, "flights: warning: nearby fares unavailable: %v\n", err)
		return nil, nil
	}
	returnValue := ""
	if !returnDate.IsZero() {
		returnValue = returnDate.Format("02/01/2006")
	}
	calendar, err := web.CalendarMonthFromPage(context.Background(), reader, link, returnValue)
	if err != nil {
		_, _ = fmt.Fprintf(cli.stderr, "flights: warning: nearby fares unavailable: %v\n", err)
		return nil, nil
	}
	cheapest := func(days []vueling.CalendarDay) *vueling.CalendarDay {
		day, ok := vueling.CheapestDay(days)
		if !ok {
			return nil
		}
		return &day
	}
	outbound := cheapest(calendar.Days)
	if outbound == nil {
		_, _ = fmt.Fprintln(cli.stderr, "flights: warning: nearby fares unavailable: Vueling did not render readable calendar fares")
	}
	if returnValue == "" {
		return outbound, nil
	}
	inbound := cheapest(calendar.ReturnDays)
	if inbound == nil {
		_, _ = fmt.Fprintln(cli.stderr, "flights: warning: nearby return fares unavailable: Vueling did not render readable return calendar fares")
	}
	return outbound, inbound
}
