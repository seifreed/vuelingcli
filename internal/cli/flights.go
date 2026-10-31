package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/search"
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
	values := map[string]string{}
	var positional []string
	format := ""
	var filter vueling.FlightFilter
	nearby := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--json" || a == "--toon" {
			if format != "" {
				return fmt.Errorf("--json and --toon cannot be used together")
			}
			format = a[2:]
			continue
		}
		if a == "--nearby" {
			nearby = true
			continue
		}
		if a == "--cheapest" || a == "--direct" {
			filter.Cheapest = filter.Cheapest || a == "--cheapest"
			filter.Direct = filter.Direct || a == "--direct"
			continue
		}
		if strings.HasPrefix(a, "--") {
			k, v, hasValue := strings.Cut(a[2:], "=")
			if !hasValue {
				remaining := args[i:]
				if len(remaining) < 2 {
					return fmt.Errorf("--%s requires a value", k)
				}
				v = remaining[1]
				i++
			}
			switch k {
			case "date", "return", "adults", "children", "infants", "currency", "cdp-url", "after", "before", "return-after", "return-before", "limit":
				values[k] = v
			default:
				return fmt.Errorf("unknown option --%s", k)
			}
			continue
		}
		if strings.HasPrefix(a, "-") {
			return fmt.Errorf("unknown option %s", a)
		}
		positional = append(positional, a)
	}
	if len(positional) != 2 || values["date"] == "" {
		return fmt.Errorf("usage: vueling flights <origin city or IATA> <destination city or IATA> --date <date> [options]")
	}
	origin, err := resolveAirport(positional[0])
	if err != nil {
		return fmt.Errorf("origin: %w", err)
	}
	destination, err := resolveAirport(positional[1])
	if err != nil {
		return fmt.Errorf("destination: %w", err)
	}
	parseDate := func(key string) (time.Time, error) {
		if values[key] == "" {
			return time.Time{}, nil
		}
		return search.ParseDate(values[key], time.Now())
	}
	date, err := parseDate("date")
	if err != nil {
		return err
	}
	returnDate, err := parseDate("return")
	if err != nil {
		return err
	}
	integer := func(key string, fallback int) (int, error) {
		if values[key] == "" {
			return fallback, nil
		}
		return strconv.Atoi(values[key])
	}
	adults, err := integer("adults", 1)
	if err != nil {
		return fmt.Errorf("invalid --adults: %w", err)
	}
	children, err := integer("children", 0)
	if err != nil {
		return fmt.Errorf("invalid --children: %w", err)
	}
	infants, err := integer("infants", 0)
	if err != nil {
		return fmt.Errorf("invalid --infants: %w", err)
	}
	if filter.Limit, err = integer("limit", 0); err != nil || (values["limit"] != "" && filter.Limit < 1) {
		return fmt.Errorf("invalid --limit: must be a whole number of at least 1")
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
		return err
	}
	beforeMinutes, err := clock("before", 24*60)
	if err != nil {
		return err
	}
	if afterMinutes > beforeMinutes {
		return fmt.Errorf("--after must not be later than --before")
	}
	returnAfterMinutes, err := clock("return-after", 0)
	if err != nil {
		return err
	}
	returnBeforeMinutes, err := clock("return-before", 24*60)
	if err != nil {
		return err
	}
	if returnAfterMinutes > returnBeforeMinutes {
		return fmt.Errorf("--return-after must not be later than --return-before")
	}
	if (values["return-after"] != "" || values["return-before"] != "") && values["return"] == "" {
		return fmt.Errorf("--return-after and --return-before need --return")
	}
	filter.After, filter.Before = values["after"], values["before"]
	filter.ReturnAfter, filter.ReturnBefore = values["return-after"], values["return-before"]
	currency := strings.ToUpper(values["currency"])
	if currency == "" {
		currency = "EUR"
	}
	searchURL, err := vueling.FlightSearchURL(vueling.LinkOptions{Origin: origin, Destination: destination, Date: date, ReturnDate: returnDate, Adults: adults, Children: children, Infants: infants, Currency: currency})
	if err != nil {
		return err
	}
	reader := cli.newReader(values["cdp-url"])
	flights, err := web.FlightsFromPage(context.Background(), reader, searchURL, date.Format(time.DateOnly), returnDate.Format(time.DateOnly))
	if err != nil {
		return err
	}
	flights = vueling.FilterFlights(flights, filter)
	result := map[string]any{"url": searchURL, "flights": flights}
	var outboundNearby, returnNearby *vueling.CalendarDay
	if nearby {
		outboundNearby, returnNearby = cli.nearbyFares(reader, origin, destination, date, returnDate)
		if outboundNearby != nil {
			result["nearby"] = outboundNearby
		}
		if returnNearby != nil {
			result["returnNearby"] = returnNearby
		}
	}
	if format == "json" {
		return json.NewEncoder(cli.stdout).Encode(result)
	}
	if format == "toon" {
		return cli.writeTOON(result)
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

func (cli *CLI) nearbyFares(reader vueling.PageReader, origin, destination string, date, returnDate time.Time) (*vueling.CalendarDay, *vueling.CalendarDay) {
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
