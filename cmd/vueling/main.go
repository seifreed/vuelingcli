package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/airports"
	"github.com/seifreed/vuelingcli/internal/search"
	"github.com/seifreed/vuelingcli/internal/vueling"
)

const version = "0.1.0"

func main() {
	if len(os.Args) == 1 || os.Args[1] == "help" || os.Args[1] == "--help" || os.Args[1] == "-h" {
		fmt.Print("vueling — unofficial read-only Vueling CLI\n\nUsage:\n  vueling <command> [options]\n\nCommands:\n  search      Search flights (in progress)\n  calendar    Build a Vueling price-calendar link\n  link        Build a flight-selection link\n  airports    Find airport codes (in progress)\n  help        Show this help\n  version     Show version\n\nRun 'vueling <command> --help' for command options.\n")
		return
	}
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Printf("vueling %s\n", version)
	case "search":
		if len(os.Args) > 2 && (os.Args[2] == "--help" || os.Args[2] == "-h") {
			fmt.Print("Search Vueling flights\n\nUsage:\n  vueling search <origin> <destination> --date YYYY-MM-DD [options]\n\nOptions:\n  --return YYYY-MM-DD  Return date\n  --adults N           Adults (default 1)\n  --children N         Children (default 0)\n  --infants N          Infants (default 0)\n  --after HH:MM        Earliest departure time\n  --before HH:MM       Latest departure time\n  --direct             Direct flights only\n  --cheapest           Sort by price\n  --available          Hide unavailable offers\n  --limit N            Maximum results\n  --json               Machine-readable JSON output\n  --toon               TOON output for agents\n")
			return
		}
		fmt.Fprintln(os.Stderr, "search: live flight search is not implemented yet")
		os.Exit(1)
	case "airports":
		if len(os.Args) > 2 && (os.Args[2] == "--help" || os.Args[2] == "-h") {
			fmt.Print("Find Vueling airport codes\n\nUsage:\n  vueling airports <city, airport name, or IATA code> [--json]\n")
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
	jsonOutput := false
	var query []string
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
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
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(results)
	}
	for _, airport := range results {
		fmt.Printf("%-4s %s\n", airport.Code, airport.City)
	}
	return nil
}

func flightLink(args []string) error {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print("Build a Vueling flight-selection link\n\nUsage:\n  vueling link <origin> <destination> --date <date> [--return <date>] [--adults N] [--children N] [--infants N] [--discount 0-5] [--culture code] [--currency code] [--json|--toon]\n")
		return nil
	}
	values := map[string]string{}
	var positional []string
	format := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
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
			case "date", "return", "adults", "children", "infants", "discount", "culture", "currency":
				values[k] = v
			default:
				return fmt.Errorf("unknown option --%s", k)
			}
			continue
		}
		positional = append(positional, a)
	}
	if len(positional) != 2 || values["date"] == "" {
		return fmt.Errorf("usage: vueling link <origin> <destination> --date <date> [options]")
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
	link, err := vueling.FlightURL(vueling.LinkOptions{Origin: positional[0], Destination: positional[1], Date: date, ReturnDate: returnDate, Adults: adults, Children: children, Infants: infants, Discount: discount, Culture: values["culture"], Currency: values["currency"]})
	if err != nil {
		return err
	}
	if format == "json" {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{"url": link})
	}
	if format == "toon" {
		fmt.Printf("url: %s\n", link)
		return nil
	}
	fmt.Println(link)
	return nil
}

func calendar(args []string) error {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print("Build a Vueling price-calendar link\n\nUsage:\n  vueling calendar <origin IATA> <destination IATA> --date <date> [--json|--toon]\n")
		return nil
	}
	var positional []string
	dateValue, format := "", ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
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
		return fmt.Errorf("usage: vueling calendar <origin IATA> <destination IATA> --date <date> [--json|--toon]")
	}
	date, err := search.ParseDate(dateValue, time.Now())
	if err != nil {
		return err
	}
	link, err := vueling.CalendarURL(positional[0], positional[1], date)
	if err != nil {
		return err
	}
	if format == "json" {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{"url": link})
	}
	if format == "toon" {
		fmt.Printf("url: %s\n", link)
		return nil
	}
	fmt.Println(link)
	return nil
}
