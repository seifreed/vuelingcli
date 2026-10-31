// Package cli runs commands for reading public Vueling fares, flights, and flight status.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/seifreed/vuelingcli/internal/airports"
	"github.com/seifreed/vuelingcli/internal/search"
	"github.com/seifreed/vuelingcli/internal/vueling"
	toon "github.com/toon-format/toon-go"
)

// PageReaderFactory creates a reader for the requested browser endpoint.
type PageReaderFactory func(endpoint string) vueling.PageReader

// CLI runs commands with caller-provided output and browser dependencies.
type CLI struct {
	version   string
	stdout    io.Writer
	stderr    io.Writer
	newReader PageReaderFactory
}

// New creates a CLI configured with its version, streams, and page-reader factory.
func New(version string, stdout, stderr io.Writer, newReader PageReaderFactory) *CLI {
	return &CLI{version: version, stdout: stdout, stderr: stderr, newReader: newReader}
}

// Run executes one command and returns its process exit code.
func (cli *CLI) Run(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		if _, err := fmt.Fprint(cli.stdout, "vueling — unofficial read-only Vueling CLI\n\nUsage:\n  vueling <command> [options]\n\nCommands:\n  flights     Read live flight offers from Chrome\n  status      Read a flight's live status from Chrome\n  search      Build an official flight-search link\n  calendar    Build a Vueling price-calendar link\n  link        Build a flight-selection link\n  airports    Search Vueling airport codes\n  help        Show this help\n  version     Show version\n\nRun 'vueling <command> --help' for command options.\n"); err != nil {
			return 2
		}
		return 0
	}
	command, args := args[0], args[1:]
	label := command
	var err error
	switch command {
	case "version", "--version", "-v":
		label = "version"
		err = cli.showVersion(args)
	case "search":
		if isHelp(args) {
			if _, err := fmt.Fprint(cli.stdout, "Build an official Vueling flight-search link. Live prices are shown on Vueling's website.\n\nUsage:\n  vueling search <origin city or IATA> <destination city or IATA> --date <date> [--return <date>] [--adults N] [--children N] [--infants N] [--discount 0-5] [--culture code] [--currency code] [--flight VY1234] [--fare-basis CODE] [--return-flight VY5678] [--return-fare-basis CODE] [--token VALUE] [--open] [--json|--toon]\n"); err != nil {
				return 2
			}
			return 0
		}
		err = cli.flightLink(args)
	case "airports":
		if isHelp(args) {
			if _, err := fmt.Fprint(cli.stdout, "Find Vueling airport codes\n\nUsage:\n  vueling airports <city, airport name, or IATA code> [--json|--toon]\n"); err != nil {
				return 2
			}
			return 0
		}
		err = cli.airportSearch(args)
	case "calendar":
		err = cli.calendar(args)
	case "flights":
		err = cli.flights(args)
	case "status":
		err = cli.flightStatus(args)
	case "link":
		err = cli.flightLink(args)
	default:
		_, _ = fmt.Fprintf(cli.stderr, "vueling: unknown command %q (run 'vueling help')\n", command)
		return 2
	}
	if err != nil {
		_, _ = fmt.Fprintf(cli.stderr, "%s: %v\n", label, err)
		return 2
	}
	return 0
}

func isHelp(args []string) bool {
	if len(args) != 1 {
		return false
	}
	for _, arg := range args {
		return arg == "--help" || arg == "-h"
	}
	return false
}

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
	flights, err := vueling.FlightsFromPage(context.Background(), reader, searchURL, date.Format(time.DateOnly), returnDate.Format(time.DateOnly))
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
	calendar, err := vueling.CalendarMonthFromPage(context.Background(), reader, link, returnValue)
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

func (cli *CLI) flightStatus(args []string) error {
	if isHelp(args) {
		if _, err := fmt.Fprint(cli.stdout, "Read a Vueling flight's live status from your Chrome session\n\nUsage:\n  vueling status <VY flight number> [--date <date>] [--watch <minutes>] [--cdp-url http://127.0.0.1:9222] [--json|--toon]\n  vueling status <origin city or IATA> <destination city or IATA> [--date <date>] [--watch <minutes>] [--cdp-url http://127.0.0.1:9222] [--json|--toon]\n\nThe date defaults to today. This command fills Vueling's public flight-status form and reads the rendered cards.\n--watch re-reads every N minutes (at least 5), prints only cards whose status or times changed, and stops once every flight has landed or been cancelled, or on Ctrl-C. It cannot be combined with --toon.\n"); err != nil {
			return err
		}
		return nil
	}
	var positional []string
	dateValue, endpoint, format, watch := "today", "", "", 0
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--json" || a == "--toon" {
			if format != "" {
				return fmt.Errorf("--json and --toon cannot be used together")
			}
			format = a[2:]
			continue
		}
		if !strings.HasPrefix(a, "--") {
			positional = append(positional, a)
			continue
		}
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
		case "date":
			dateValue = v
		case "cdp-url":
			endpoint = v
		case "watch":
			n, err := strconv.Atoi(v)
			if err != nil || n < 5 {
				return fmt.Errorf("--watch must be a whole number of minutes, at least 5")
			}
			watch = n
		default:
			return fmt.Errorf("unknown option --%s", k)
		}
	}
	if watch > 0 && format == "toon" {
		return fmt.Errorf("--watch cannot be used with --toon")
	}
	flight, origin, destination := "", "", ""
	switch len(positional) {
	case 1:
		flight = strings.ToUpper(positional[0])
	case 2:
		var err error
		if origin, err = resolveAirport(positional[0]); err != nil {
			return err
		}
		if destination, err = resolveAirport(positional[1]); err != nil {
			return err
		}
	default:
		return fmt.Errorf("usage: vueling status <VY flight number> | <origin> <destination> [--date <date>]")
	}
	date, err := search.ParseDate(dateValue, time.Now())
	if err != nil {
		return fmt.Errorf("invalid --date: %w", err)
	}
	reader := cli.newReader(endpoint)
	if watch > 0 {
		return cli.watchFlightStatus(time.Duration(watch)*time.Minute, format == "json", func(ctx context.Context) ([]vueling.FlightStatus, error) {
			return vueling.FlightStatusFromPage(ctx, reader, flight, origin, destination, date)
		})
	}
	statuses, err := vueling.FlightStatusFromPage(context.Background(), reader, flight, origin, destination, date)
	if err != nil {
		return err
	}
	switch format {
	case "json":
		return json.NewEncoder(cli.stdout).Encode(statuses)
	case "toon":
		return cli.writeTOON(statuses)
	}
	for _, s := range statuses {
		if _, err := fmt.Fprintln(cli.stdout, statusLine(s)); err != nil {
			return err
		}
	}
	return nil
}

func statusLine(s vueling.FlightStatus) string {
	return fmt.Sprintf("%s %s %s %s–%s %s→%s", s.Flight, s.Date, s.Status, s.Departure, s.Arrival, s.Origin, s.Destination)
}

func (cli *CLI) watchFlightStatus(every time.Duration, asJSON bool, read func(context.Context) ([]vueling.FlightStatus, error)) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	encoder := json.NewEncoder(cli.stdout)
	var previous []vueling.FlightStatus
	for first := true; ; first = false {
		statuses, err := read(ctx)
		switch {
		case ctx.Err() != nil:
			return nil
		case err != nil && first:
			return err
		case err != nil:
			_, _ = fmt.Fprintf(cli.stderr, "status: %v (retrying in %s)\n", err, every)
		default:
			for _, s := range vueling.ChangedStatuses(previous, statuses) {
				if asJSON {
					if err := encoder.Encode(s); err != nil {
						return err
					}
				} else {
					if _, err := fmt.Fprintf(cli.stdout, "%s %s\n", time.Now().Format("15:04"), statusLine(s)); err != nil {
						return err
					}
				}
			}
			if vueling.AllFinal(statuses) {
				return nil
			}
			previous = statuses
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(every):
		}
	}
}

func (cli *CLI) showVersion(args []string) error {
	format := ""
	for _, arg := range args {
		switch arg {
		case "--help", "-h":
			if _, err := fmt.Fprint(cli.stdout, "Show the CLI version\n\nUsage:\n  vueling version [--json|--toon]\n"); err != nil {
				return err
			}
			return nil
		case "--json", "--toon":
			if format != "" && format != arg[2:] {
				return fmt.Errorf("--json and --toon cannot be used together")
			}
			format = arg[2:]
		default:
			return fmt.Errorf("unknown option %s", arg)
		}
	}
	value := map[string]string{"version": cli.version}
	switch format {
	case "json":
		return json.NewEncoder(cli.stdout).Encode(value)
	case "toon":
		return cli.writeTOON(value)
	default:
		_, err := fmt.Fprintf(cli.stdout, "vueling %s\n", cli.version)
		return err
	}
}

func (cli *CLI) airportSearch(args []string) error {
	format := ""
	var query []string
	for _, arg := range args {
		switch {
		case arg == "--json" || arg == "--toon":
			requested := arg[2:]
			if format != "" && format != requested {
				return fmt.Errorf("--json and --toon cannot be used together")
			}
			format = requested
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown option %s", arg)
		default:
			query = append(query, arg)
		}
	}
	if len(query) > 1 {
		return fmt.Errorf("use one city, airport, or IATA code")
	}
	term := ""
	if len(query) == 1 {
		term = query[0]
	}
	results := airports.Search(term)
	if format == "json" {
		return json.NewEncoder(cli.stdout).Encode(results)
	}
	if format == "toon" {
		return cli.writeTOON(results)
	}
	for _, airport := range results {
		if _, err := fmt.Fprintf(cli.stdout, "%-4s %s\n", airport.Code, airport.City); err != nil {
			return err
		}
	}
	return nil
}

func (cli *CLI) flightLink(args []string) error {
	if isHelp(args) {
		_, err := fmt.Fprint(cli.stdout, "Build a Vueling flight-selection link\n\nUsage:\n  vueling link <origin city or IATA> <destination city or IATA> --date <date> [--return <date>] [--adults N] [--children N] [--infants N] [--discount 0-5] [--culture code] [--currency code] [--flight VY1234] [--fare-basis CODE] [--return-flight VY5678] [--return-fare-basis CODE] [--token VALUE] [--open] [--json|--toon]\n")
		return err
	}
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
				return fmt.Errorf("--json and --toon cannot be used together")
			}
			format = a[2:]
			continue
		}
		if len(a) > 2 && a[:2] == "--" {
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
			case "date", "return", "adults", "children", "infants", "discount", "culture", "currency", "flight", "fare-basis", "return-flight", "return-fare-basis", "token":
				values[k] = v
			default:
				return fmt.Errorf("unknown option --%s", k)
			}
			continue
		}
		positional = append(positional, a)
	}
	if len(positional) != 2 || values["date"] == "" {
		return fmt.Errorf("usage: vueling link <origin city or IATA> <destination city or IATA> --date <date> [options]")
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
	discount, err := integer("discount", 0)
	if err != nil {
		return fmt.Errorf("invalid --discount: %w", err)
	}
	link, err := vueling.FlightURL(vueling.LinkOptions{Origin: origin, Destination: destination, Date: date, ReturnDate: returnDate, Adults: adults, Children: children, Infants: infants, Discount: discount, Culture: values["culture"], Currency: values["currency"], OutboundFlight: values["flight"], OutboundFareBasis: values["fare-basis"], ReturnFlight: values["return-flight"], ReturnFareBasis: values["return-fare-basis"], METToken: values["token"]})
	if err != nil {
		return err
	}
	if open {
		if err := openURL(link); err != nil {
			return fmt.Errorf("open flight link: %w", err)
		}
	}
	if format == "json" {
		return json.NewEncoder(cli.stdout).Encode(map[string]string{"url": link})
	}
	if format == "toon" {
		return cli.writeTOON(map[string]string{"url": link})
	}
	_, err = fmt.Fprintln(cli.stdout, link)
	return err
}

func resolveAirport(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) == 3 {
		return strings.ToUpper(value), nil
	}
	matches := airports.Search(value)
	switch len(matches) {
	case 1:
		return matches[0].Code, nil
	case 0:
		return "", fmt.Errorf("%q is not in the airport catalogue; use an IATA code or 'vueling airports'", value)
	default:
		codes := make([]string, len(matches))
		for i, match := range matches {
			codes[i] = match.Code
		}
		return "", fmt.Errorf("%q matches multiple airports (%s); use an IATA code", value, strings.Join(codes, ", "))
	}
}

func openURL(link string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", link)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", link)
	case "linux":
		cmd = exec.Command("xdg-open", link)
	default:
		return fmt.Errorf("opening links is not supported on %s", runtime.GOOS)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func (cli *CLI) calendar(args []string) error {
	if isHelp(args) {
		_, err := fmt.Fprint(cli.stdout, "Read Vueling calendar prices or build its link\n\nUsage:\n  vueling calendar <origin city or IATA> <destination city or IATA> --date <date> [--return <date>] [--prices [--month]] [--open] [--json|--toon]\n")
		return err
	}
	var positional []string
	dateValue, returnValue, format := "", "", ""
	open, prices, month := false, false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--open":
			open = true
		case a == "--prices":
			prices = true
		case a == "--month":
			month = true
		case a == "--date":
			remaining := args[i:]
			if len(remaining) < 2 {
				return fmt.Errorf("--date requires a value")
			}
			dateValue = remaining[1]
			i++
		case a == "--return":
			remaining := args[i:]
			if len(remaining) < 2 {
				return fmt.Errorf("--return requires a value")
			}
			returnValue = remaining[1]
			i++
		case len(a) > 9 && a[:9] == "--return=":
			returnValue = a[9:]
		case len(a) > 7 && a[:7] == "--date=":
			dateValue = a[7:]
		case a == "--json" || a == "--toon":
			if format != "" {
				return fmt.Errorf("--json and --toon cannot be used together")
			}
			format = a[2:]
		case len(a) > 0 && a[0] == '-':
			return fmt.Errorf("unknown option %s", a)
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) != 2 || dateValue == "" {
		return fmt.Errorf("usage: vueling calendar <origin city or IATA> <destination city or IATA> --date <date> [--return <date>] [--prices [--month]] [--open] [--json|--toon]")
	}
	if month && !prices {
		return fmt.Errorf("--month requires --prices")
	}
	origin, err := resolveAirport(positional[0])
	if err != nil {
		return fmt.Errorf("origin: %w", err)
	}
	destination, err := resolveAirport(positional[1])
	if err != nil {
		return fmt.Errorf("destination: %w", err)
	}
	date, err := search.ParseDate(dateValue, time.Now())
	if err != nil {
		return err
	}
	var returnDate string
	if returnValue != "" {
		parsed, err := search.ParseDate(returnValue, time.Now())
		if err != nil {
			return err
		}
		if parsed.Before(date) {
			return fmt.Errorf("return date must be on or after departure date")
		}
		if parsed.Sub(date) > 330*24*time.Hour {
			return fmt.Errorf("return date must be within 330 days of departure for calendar price lookup")
		}
		returnDate = parsed.Format("02/01/2006")
	}
	link, err := vueling.CalendarURL(origin, destination, date)
	if err != nil {
		return err
	}
	reader := cli.newReader("")
	if month {
		return cli.calendarMonth(reader, link, returnDate, format)
	}
	if prices {
		price, err := vueling.CalendarPricesFromPage(context.Background(), reader, link, returnDate)
		if err != nil {
			return err
		}
		price["url"] = link
		if format == "json" {
			return json.NewEncoder(cli.stdout).Encode(price)
		}
		if format == "toon" {
			return cli.writeTOON(price)
		}
		_, err = fmt.Fprintf(cli.stdout, "%s %s: %s\n", price["route"], date.Format("02/01/2006"), price["fare"])
		return err
	}
	if open {
		if err := openURL(link); err != nil {
			return fmt.Errorf("open calendar link: %w", err)
		}
	}
	if format == "json" {
		return json.NewEncoder(cli.stdout).Encode(map[string]string{"url": link})
	}
	if format == "toon" {
		return cli.writeTOON(map[string]string{"url": link})
	}
	_, err = fmt.Fprintln(cli.stdout, link)
	return err
}

func (cli *CLI) calendarMonth(reader vueling.PageReader, link, returnDate, format string) error {
	calendar, err := vueling.CalendarMonthFromPage(context.Background(), reader, link, returnDate)
	if err != nil {
		return err
	}
	cheapest, ok := vueling.CheapestDay(calendar.Days)
	if !ok {
		return fmt.Errorf("Vueling did not render readable calendar fares")
	}
	result := map[string]any{"url": link, "days": calendar.Days, "cheapest": cheapest}
	for key, value := range calendar.Price {
		result[key] = value
	}
	var returnCheapest vueling.CalendarDay
	if returnDate != "" {
		if returnCheapest, ok = vueling.CheapestDay(calendar.ReturnDays); !ok {
			return fmt.Errorf("Vueling did not render readable return calendar fares")
		}
		result["returnDays"], result["returnCheapest"] = calendar.ReturnDays, returnCheapest
	}
	if format == "json" {
		return json.NewEncoder(cli.stdout).Encode(result)
	}
	if format == "toon" {
		return cli.writeTOON(result)
	}
	if err := cli.printCalendarDays(calendar.Price["route"], calendar.Days, cheapest); err != nil {
		return err
	}
	if returnDate != "" {
		if err := cli.printCalendarDays(calendar.Price["returnRoute"], calendar.ReturnDays, returnCheapest); err != nil {
			return err
		}
	}
	return nil
}

func (cli *CLI) printCalendarDays(route string, days []vueling.CalendarDay, cheapest vueling.CalendarDay) error {
	if _, err := fmt.Fprintln(cli.stdout, route); err != nil {
		return err
	}
	for _, day := range days {
		mark := ""
		if day == cheapest {
			mark = " [cheapest]"
		}
		if _, err := fmt.Fprintf(cli.stdout, "%s: %s%s\n", day.Date, day.Fare, mark); err != nil {
			return err
		}
	}
	return nil
}

func (cli *CLI) writeTOON(value any) error {
	encoded, err := encodeTOON(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cli.stdout, encoded)
	return err
}

func encodeTOON(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return "", err
	}
	return toon.MarshalString(generic)
}
