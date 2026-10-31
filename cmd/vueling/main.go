// Command vueling reads Vueling fares, flights, and flight status from the user's Chrome session.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/seifreed/vuelingcli/internal/airports"
	"github.com/seifreed/vuelingcli/internal/browser"
	"github.com/seifreed/vuelingcli/internal/search"
	"github.com/seifreed/vuelingcli/internal/vueling"
	toon "github.com/toon-format/toon-go"
)

var version = "0.1.0"

func main() {
	if len(os.Args) == 1 || os.Args[1] == "help" || os.Args[1] == "--help" || os.Args[1] == "-h" {
		fmt.Print("vueling — unofficial read-only Vueling CLI\n\nUsage:\n  vueling <command> [options]\n\nCommands:\n  flights     Read live flight offers from Chrome\n  status      Read a flight's live status from Chrome\n  search      Build an official flight-search link\n  calendar    Build a Vueling price-calendar link\n  link        Build a flight-selection link\n  airports    Search Vueling airport codes\n  help        Show this help\n  version     Show version\n\nRun 'vueling <command> --help' for command options.\n")
		return
	}
	switch os.Args[1] {
	case "version", "--version", "-v":
		if err := showVersion(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "version: %v\n", err)
			os.Exit(2)
		}
	case "search":
		if len(os.Args) > 2 && (os.Args[2] == "--help" || os.Args[2] == "-h") {
			fmt.Print("Build an official Vueling flight-search link. Live prices are shown on Vueling's website.\n\nUsage:\n  vueling search <origin city or IATA> <destination city or IATA> --date <date> [--return <date>] [--adults N] [--children N] [--infants N] [--discount 0-5] [--culture code] [--currency code] [--flight VY1234] [--fare-basis CODE] [--return-flight VY5678] [--return-fare-basis CODE] [--token VALUE] [--open] [--json|--toon]\n")
			return
		}
		if err := flightLink(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "search: %v\n", err)
			os.Exit(2)
		}
	case "airports":
		if len(os.Args) > 2 && (os.Args[2] == "--help" || os.Args[2] == "-h") {
			fmt.Print("Find Vueling airport codes\n\nUsage:\n  vueling airports <city, airport name, or IATA code> [--json|--toon]\n")
			return
		}
		if err := airportSearch(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "airports: %v\n", err)
			os.Exit(2)
		}
	case "calendar":
		if err := calendar(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "calendar: %v\n", err)
			os.Exit(2)
		}
	case "flights":
		if err := flights(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "flights: %v\n", err)
			os.Exit(2)
		}
	case "status":
		if err := flightStatus(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "status: %v\n", err)
			os.Exit(2)
		}
	case "link":
		if err := flightLink(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "link: %v\n", err)
			os.Exit(2)
		}
	default:
		fmt.Fprintf(os.Stderr, "vueling: unknown command %q (run 'vueling help')\n", os.Args[1])
		os.Exit(2)
	}
}

func flights(args []string) error {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print("Read live flight offers from your Chrome session\n\nUsage:\n  vueling flights <origin city or IATA> <destination city or IATA> --date <date> [--return <date>] [--adults N] [--children N] [--infants N] [--currency code] [--cdp-url http://127.0.0.1:9222] [--cheapest] [--direct] [--after HH:MM] [--before HH:MM] [--return-after HH:MM] [--return-before HH:MM] [--limit N] [--nearby] [--json|--toon]\n\nFilters, sorting, and limits apply to outbound and return offers independently. --after and --before are inclusive departure times; --return-after and --return-before replace them for return offers. --nearby also reads the price calendar and reports the cheapest day of the outbound (and return) month.\n\nOn macOS, enable View > Developer > Allow JavaScript from Apple Events in Chrome. Other systems need a local Chrome DevTools endpoint. This command reads rendered offers only; it never selects a fare.\n")
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
				i++
				if i == len(args) {
					return fmt.Errorf("--%s requires a value", k)
				}
				v = args[i]
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
	reader := browser.NewChromeReader(values["cdp-url"])
	flights, err := vueling.FlightsFromPage(context.Background(), reader, searchURL, date.Format(time.DateOnly), returnDate.Format(time.DateOnly))
	if err != nil {
		return err
	}
	flights = vueling.FilterFlights(flights, filter)
	result := map[string]any{"url": searchURL, "flights": flights}
	var outboundNearby, returnNearby *vueling.CalendarDay
	if nearby {
		outboundNearby, returnNearby = nearbyFares(reader, origin, destination, date, returnDate)
		if outboundNearby != nil {
			result["nearby"] = outboundNearby
		}
		if returnNearby != nil {
			result["returnNearby"] = returnNearby
		}
	}
	if format == "json" {
		return json.NewEncoder(os.Stdout).Encode(result)
	}
	if format == "toon" {
		return writeTOON(result)
	}
	fmt.Printf("%s\n", searchURL)
	for _, flight := range flights {
		fmt.Printf("%-8s %s %s %s–%s %s %s | %s%s\n", strings.ToUpper(flight.Direction), flight.Date, flight.Flight, flight.Departure, flight.Arrival, flight.Fare, flight.Currency, connectionSummary(flight), flight.Itinerary)
	}
	if outboundNearby != nil {
		fmt.Printf("cheapest nearby: %s from %s\n", outboundNearby.Date, outboundNearby.Fare)
	}
	if returnNearby != nil {
		fmt.Printf("cheapest nearby return: %s from %s\n", returnNearby.Date, returnNearby.Fare)
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
func nearbyFares(reader vueling.PageReader, origin, destination string, date, returnDate time.Time) (*vueling.CalendarDay, *vueling.CalendarDay) {
	link, err := vueling.CalendarURL(origin, destination, date)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flights: warning: nearby fares unavailable: %v\n", err)
		return nil, nil
	}
	returnValue := ""
	if !returnDate.IsZero() {
		returnValue = returnDate.Format("02/01/2006")
	}
	calendar, err := vueling.CalendarMonthFromPage(context.Background(), reader, link, returnValue)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flights: warning: nearby fares unavailable: %v\n", err)
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
		fmt.Fprintln(os.Stderr, "flights: warning: nearby fares unavailable: Vueling did not render readable calendar fares")
	}
	if returnValue == "" {
		return outbound, nil
	}
	inbound := cheapest(calendar.ReturnDays)
	if inbound == nil {
		fmt.Fprintln(os.Stderr, "flights: warning: nearby return fares unavailable: Vueling did not render readable return calendar fares")
	}
	return outbound, inbound
}

func flightStatus(args []string) error {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print("Read a Vueling flight's live status from your Chrome session\n\nUsage:\n  vueling status <VY flight number> [--date <date>] [--watch <minutes>] [--cdp-url http://127.0.0.1:9222] [--json|--toon]\n  vueling status <origin city or IATA> <destination city or IATA> [--date <date>] [--watch <minutes>] [--cdp-url http://127.0.0.1:9222] [--json|--toon]\n\nThe date defaults to today. This command fills Vueling's public flight-status form and reads the rendered cards.\n--watch re-reads every N minutes (at least 5), prints only cards whose status or times changed, and stops once every flight has landed or been cancelled, or on Ctrl-C. It cannot be combined with --toon.\n")
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
			if i+1 == len(args) {
				return fmt.Errorf("--%s requires a value", k)
			}
			i++
			v = args[i]
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
	reader := browser.NewChromeReader(endpoint)
	if watch > 0 {
		return watchFlightStatus(time.Duration(watch)*time.Minute, format == "json", func(ctx context.Context) ([]vueling.FlightStatus, error) {
			return vueling.FlightStatusFromPage(ctx, reader, flight, origin, destination, date)
		})
	}
	statuses, err := vueling.FlightStatusFromPage(context.Background(), reader, flight, origin, destination, date)
	if err != nil {
		return err
	}
	switch format {
	case "json":
		return json.NewEncoder(os.Stdout).Encode(statuses)
	case "toon":
		return writeTOON(statuses)
	}
	for _, s := range statuses {
		fmt.Println(statusLine(s))
	}
	return nil
}

func statusLine(s vueling.FlightStatus) string {
	return fmt.Sprintf("%s %s %s %s–%s %s→%s", s.Flight, s.Date, s.Status, s.Departure, s.Arrival, s.Origin, s.Destination)
}

func watchFlightStatus(every time.Duration, asJSON bool, read func(context.Context) ([]vueling.FlightStatus, error)) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	encoder := json.NewEncoder(os.Stdout)
	var previous []vueling.FlightStatus
	for first := true; ; first = false {
		statuses, err := read(ctx)
		switch {
		case ctx.Err() != nil:
			return nil
		case err != nil && first:
			return err
		case err != nil:
			fmt.Fprintf(os.Stderr, "status: %v (retrying in %s)\n", err, every)
		default:
			for _, s := range vueling.ChangedStatuses(previous, statuses) {
				if asJSON {
					if err := encoder.Encode(s); err != nil {
						return err
					}
				} else {
					fmt.Printf("%s %s\n", time.Now().Format("15:04"), statusLine(s))
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

func showVersion(args []string) error {
	format := ""
	for _, arg := range args {
		switch arg {
		case "--help", "-h":
			fmt.Print("Show the CLI version\n\nUsage:\n  vueling version [--json|--toon]\n")
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
	value := map[string]string{"version": version}
	switch format {
	case "json":
		return json.NewEncoder(os.Stdout).Encode(value)
	case "toon":
		return writeTOON(value)
	default:
		fmt.Printf("vueling %s\n", version)
		return nil
	}
}

func airportSearch(args []string) error {
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
		return json.NewEncoder(os.Stdout).Encode(results)
	}
	if format == "toon" {
		return writeTOON(results)
	}
	for _, airport := range results {
		fmt.Printf("%-4s %s\n", airport.Code, airport.City)
	}
	return nil
}

func flightLink(args []string) error {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print("Build a Vueling flight-selection link\n\nUsage:\n  vueling link <origin city or IATA> <destination city or IATA> --date <date> [--return <date>] [--adults N] [--children N] [--infants N] [--discount 0-5] [--culture code] [--currency code] [--flight VY1234] [--fare-basis CODE] [--return-flight VY5678] [--return-fare-basis CODE] [--token VALUE] [--open] [--json|--toon]\n")
		return nil
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
				i++
				if i == len(args) {
					return fmt.Errorf("--%s requires a value", k)
				}
				v = args[i]
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
		return json.NewEncoder(os.Stdout).Encode(map[string]string{"url": link})
	}
	if format == "toon" {
		return writeTOON(map[string]string{"url": link})
	}
	fmt.Println(link)
	return nil
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

func calendar(args []string) error {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print("Read Vueling calendar prices or build its link\n\nUsage:\n  vueling calendar <origin city or IATA> <destination city or IATA> --date <date> [--return <date>] [--prices [--month]] [--open] [--json|--toon]\n")
		return nil
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
			i++
			if i == len(args) {
				return fmt.Errorf("--date requires a value")
			}
			dateValue = args[i]
		case a == "--return":
			i++
			if i == len(args) {
				return fmt.Errorf("--return requires a value")
			}
			returnValue = args[i]
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
	reader := browser.NewChromeReader("")
	if month {
		return calendarMonth(reader, link, returnDate, format)
	}
	if prices {
		price, err := vueling.CalendarPricesFromPage(context.Background(), reader, link, returnDate)
		if err != nil {
			return err
		}
		price["url"] = link
		if format == "json" {
			return json.NewEncoder(os.Stdout).Encode(price)
		}
		if format == "toon" {
			return writeTOON(price)
		}
		fmt.Printf("%s %s: %s\n", price["route"], date.Format("02/01/2006"), price["fare"])
		return nil
	}
	if open {
		if err := openURL(link); err != nil {
			return fmt.Errorf("open calendar link: %w", err)
		}
	}
	if format == "json" {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{"url": link})
	}
	if format == "toon" {
		return writeTOON(map[string]string{"url": link})
	}
	fmt.Println(link)
	return nil
}

func calendarMonth(reader vueling.PageReader, link, returnDate, format string) error {
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
		return json.NewEncoder(os.Stdout).Encode(result)
	}
	if format == "toon" {
		return writeTOON(result)
	}
	printCalendarDays(calendar.Price["route"], calendar.Days, cheapest)
	if returnDate != "" {
		printCalendarDays(calendar.Price["returnRoute"], calendar.ReturnDays, returnCheapest)
	}
	return nil
}

func printCalendarDays(route string, days []vueling.CalendarDay, cheapest vueling.CalendarDay) {
	fmt.Println(route)
	for _, day := range days {
		mark := ""
		if day == cheapest {
			mark = " [cheapest]"
		}
		fmt.Printf("%s: %s%s\n", day.Date, day.Fare, mark)
	}
}

func writeTOON(value any) error {
	encoded, err := encodeTOON(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, encoded)
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
