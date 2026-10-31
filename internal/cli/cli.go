// Package cli runs commands for reading public Vueling fares, flights, and flight status.
package cli

import (
	"fmt"
	"io"

	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

// PageReaderFactory creates a reader for the requested browser endpoint.
type PageReaderFactory func(endpoint string) web.PageReader

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
		if _, err := fmt.Fprintf(cli.stderr, "vueling: unknown command %q (run 'vueling help')\n", command); err != nil {
			return 1
		}
		return 2
	}
	if err != nil {
		if _, writeErr := fmt.Fprintf(cli.stderr, "%s: %v\n", label, err); writeErr != nil {
			return 1
		}
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
