package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/airports"
	"github.com/seifreed/vuelingcli/internal/search"
	"github.com/seifreed/vuelingcli/internal/vueling"
	toon "github.com/toon-format/toon-go"
)

const version = "0.1.0"

func main() {
	if len(os.Args) == 1 || os.Args[1] == "help" || os.Args[1] == "--help" || os.Args[1] == "-h" {
		fmt.Print("vueling — unofficial read-only Vueling CLI\n\nUsage:\n  vueling <command> [options]\n\nCommands:\n  search      Build an official flight-search link\n  calendar    Build a Vueling price-calendar link\n  link        Build a flight-selection link\n  airports    Find common airport codes\n  help        Show this help\n  version     Show version\n\nRun 'vueling <command> --help' for command options.\n")
		return
	}
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Printf("vueling %s\n", version)
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

func airportSearch(args []string) error {
	format := ""
	var query []string
	for _, arg := range args {
		if arg == "--json" || arg == "--toon" {
			requested := arg[2:]
			if format != "" && format != requested {
				return fmt.Errorf("--json and --toon cannot be used together")
			}
			format = requested
		} else if strings.HasPrefix(arg, "-") {
			return fmt.Errorf("unknown option %s", arg)
		} else {
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
		fmt.Print("Build a Vueling price-calendar link\n\nUsage:\n  vueling calendar <origin city or IATA> <destination city or IATA> --date <date> [--open] [--json|--toon]\n")
		return nil
	}
	var positional []string
	dateValue, format := "", ""
	open := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--open":
			open = true
		case a == "--date":
			i++
			if i == len(args) {
				return fmt.Errorf("--date requires a value")
			}
			dateValue = args[i]
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
		return fmt.Errorf("usage: vueling calendar <origin city or IATA> <destination city or IATA> --date <date> [--open] [--json|--toon]")
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
	link, err := vueling.CalendarURL(origin, destination, date)
	if err != nil {
		return err
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
