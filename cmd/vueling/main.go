package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	if len(os.Args) == 1 || os.Args[1] == "help" || os.Args[1] == "--help" || os.Args[1] == "-h" {
		fmt.Print("vueling — unofficial read-only Vueling CLI\n\nUsage:\n  vueling <command> [options]\n\nCommands:\n  search      Search flights\n  airports    Find airport codes\n  help        Show this help\n  version     Show version\n\nRun 'vueling <command> --help' for command options.\n")
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
		fmt.Fprintln(os.Stderr, "airports: lookup is not implemented yet")
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "vueling: unknown command %q (run 'vueling help')\n", os.Args[1])
		os.Exit(2)
	}
}
