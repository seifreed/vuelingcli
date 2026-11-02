package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
	"github.com/seifreed/vuelingcli/internal/vueling/ports"
)

func (cli *CLI) flights(args []string) error {
	if isHelp(args) {
		if _, err := fmt.Fprint(cli.stdout, "Read every Vueling flight offer for a day\n\nUsage:\n  vueling flights <origin city or IATA> <destination city or IATA> --date <date> [--return <date>] [--adults N] [--children N] [--infants N] [--discount 0-5] [--currency code] [--browser] [--cdp-url http://127.0.0.1:9222] [--cheapest] [--direct] [--after HH:MM] [--before HH:MM] [--return-after HH:MM] [--return-before HH:MM] [--limit N] [--nearby] [--json|--toon]\n\nFilters, sorting, and limits apply to outbound and return offers independently. --after and --before are inclusive departure times; --return-after and --return-before replace them for return offers. --nearby also reads the price calendar and reports the cheapest day of the outbound (and return) month.\n\nOffers come from Vueling's availability API over HTTPS, using the anonymous session every visitor of tickets.vueling.com gets; no account or cookies are involved. Each fare is the cheapest price for one adult; infants do not change it. --discount applies a Spanish discount in EUR (1 resident, 2 large family, 3 special large family, 4 resident + large family, 5 resident + special large family); Vueling rejects it on routes without that discount.\n\n--browser reads the offers rendered in your Chrome session instead. On macOS, enable View > Developer > Allow JavaScript from Apple Events in Chrome. Other systems need a local Chrome DevTools endpoint (--cdp-url). This command never selects a fare.\n"); err != nil {
			return err
		}
		return nil
	}
	options, searchURL, err := parseFlightSearchOptions(args)
	if err != nil {
		return err
	}
	var reader ports.PageReader
	if options.browser || cli.api == nil {
		reader, err = cli.pageReader(options.endpoint)
		if err != nil {
			return err
		}
	}
	flights, err := cli.readFlights(reader, searchURL, options)
	if err != nil {
		return err
	}
	flights, err = vueling.FilterFlights(flights, options.filter)
	if err != nil {
		return err
	}
	result := map[string]any{"url": searchURL, "flights": flights}
	var outboundNearby, returnNearby *vueling.CalendarDay
	if options.nearby {
		outboundNearby, returnNearby = cli.nearbyFares(reader, options.browser, options.currency, options.origin, options.destination, options.date, options.returnDate)
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
		if _, err := fmt.Fprintf(cli.stdout, "%-8s %s %s %s–%s %s %s%s | %s%s\n", strings.ToUpper(flight.Direction), flight.Date, flight.Flight, flight.Departure, flight.ArrivalClock(), flight.Fare, flight.Currency, bundleSummary(flight), connectionSummary(flight), flight.Itinerary); err != nil {
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

func parseFlightSearchOptions(args []string) (flightOptions, string, error) {
	options, err := parseFlightOptions(args)
	if err != nil {
		return flightOptions{}, "", err
	}
	searchURL, err := vueling.FlightSearchURL(options.linkOptions())
	return options, searchURL, err
}

// connectionSummary renders a connecting flight's legs, such as
// "1 escala vía BCN · VY1383+VY1050 | ", and nothing for direct flights.

// bundleSummary lists the pricier fare bundles, such as Optima, next to the cheapest fare.
func bundleSummary(flight vueling.Flight) string {
	var bundles []string
	for bundle, fare := range flight.Fares {
		if fare != flight.Fare {
			bundles = append(bundles, bundle+" "+fare)
		}
	}
	if len(bundles) == 0 {
		return ""
	}
	sort.Strings(bundles)
	return " (" + strings.Join(bundles, ", ") + ")"
}

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

// nearbyFares reads the cheapest calendar day of the outbound and return months from Vueling's
// fare-calendar API, or from the rendered calendar page in Chrome when browser is set,
// structured warning instead of failing because the flight offers are the primary output.

func (cli *CLI) nearbyFares(reader ports.PageReader, browser bool, currency, origin, destination string, date, returnDate time.Time) (*vueling.CalendarDay, *vueling.CalendarDay) {
	returnValue := ""
	if !returnDate.IsZero() {
		returnValue = returnDate.Format("02/01/2006")
	}
	link, err := vueling.CalendarURL(origin, destination, date)
	var calendar vueling.CalendarMonth
	if err == nil {
		request := calendarRequest{origin: origin, destination: destination, date: date, returnDate: returnValue, currency: currency, month: true}
		calendar, err = cli.readCalendar(context.Background(), browser, reader, link, request)
	}
	if err != nil {
		direction := "outbound"
		if returnValue != "" {
			direction = "outbound_and_return"
		}
		cli.logNearbyFareWarning(direction, err)
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
		missingFareErr := errors.New("Vueling did not render readable calendar fares")
		cli.logNearbyFareWarning("outbound", missingFareErr)
	}
	if returnValue == "" {
		return outbound, nil
	}
	inbound := cheapest(calendar.ReturnDays)
	if inbound == nil {
		missingFareErr := errors.New("Vueling did not render readable return calendar fares")
		cli.logNearbyFareWarning("return", missingFareErr)
	}
	return outbound, inbound
}

func (cli *CLI) logNearbyFareWarning(direction string, cause error) {
	if cli.logger != nil {
		cli.logger.Warn("Nearby fare data unavailable", "event_name", "flights.nearby_fares.unavailable", "operation", "nearby_fares", "direction", direction, "outcome", "degraded", "error_type", errorType(cause))
	}
}
