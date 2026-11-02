package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/seifreed/vuelingcli/internal/observability"
)

// Run executes one command and returns its process exit code.
func (cli *CLI) Run(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		if _, err := fmt.Fprint(cli.stdout, "vueling — unofficial read-only Vueling CLI\n\nUsage:\n  vueling <command> [options]\n\nCommands:\n  flights     Read a day's flight offers from Vueling\n  status      Read a flight's live status\n  alerts      Read Vueling's current disruption notice\n  search      Build an official flight-search link\n  calendar    Read Vueling calendar prices or build its link\n  deals       Find the cheapest fare to every destination from an origin\n  trips       Find the cheapest departure and return days for an N-night trip\n  select      Price a flight and its fare bundles without booking\n  extras      Read a flight's exact seat, bag and extra prices without booking\n  link        Build a flight-selection link\n  fees        Read Vueling's prices for bags, seats, changes and extras\n  routes      List the destinations Vueling flies to from an origin\n  schedule    Show on which days Vueling flies a route\n  airports    Search Vueling airport codes\n  session     Show, extend, or end the reused anonymous session\n  help        Show this help\n  version     Show version\n\nRun 'vueling <command> --help' for command options.\n"); err != nil {
			return 2
		}
		return 0
	}
	command, args := args[0], args[1:]
	label := command
	var err error
	started := time.Now()
	if cli.logger != nil {
		cli.logger.LogAttrs(context.Background(), observability.TraceLevel, "Command started",
			slog.String("event_name", "command.started"), slog.String("operation", label))
	}
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
		err = cli.flightLink(command, args)
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
	case "deals":
		err = cli.deals(args)
	case "trips":
		err = cli.trips(args)
	case "routes":
		err = cli.routes(args)
	case "schedule":
		err = cli.schedule(args)
	case "alerts":
		err = cli.alerts(args)
	case "fees":
		err = cli.fees(args)
	case "session":
		err = cli.session(args)
	case "flights":
		err = cli.flights(args)
	case "status":
		err = cli.flightStatus(args)
	case "extras":
		err = cli.extras(args)
	case "select":
		err = cli.selectFlight(args)
	case "link":
		err = cli.flightLink(command, args)
	default:
		if cli.logger != nil {
			cli.logger.Warn("Command was not recognized",
				"event_name", "command.rejected", "operation", "unknown_command", "outcome", "error", "error_code", "unknown_command", "exit_code", 2)
		}
		if _, err := fmt.Fprintf(cli.stderr, "vueling: unknown command %q (run 'vueling help')\n", command); err != nil {
			return 1
		}
		return 2
	}
	if err != nil {
		if cli.logger != nil {
			attrs := []slog.Attr{
				slog.String("event_name", "command.finished"), slog.String("operation", label),
				slog.String("outcome", "error"), slog.Int("exit_code", 2), slog.String("error_type", errorType(err)),
			}
			if code := httpStatusCode(err); code != 0 {
				attrs = append(attrs, slog.Int("status_code", code))
			}
			cli.logger.LogAttrs(context.Background(), slog.LevelError, "Command failed",
				append(attrs, slog.Int64("duration_ms", time.Since(started).Milliseconds()))...)
		}
		if _, writeErr := fmt.Fprintf(cli.stderr, "%s: %s\n", label, safeErrorMessage(err.Error())); writeErr != nil {
			return 1
		}
		return 2
	}
	if cli.logger != nil {
		cli.logger.LogAttrs(context.Background(), slog.LevelInfo, "Command completed",
			slog.String("event_name", "command.finished"), slog.String("operation", label),
			slog.Int64("duration_ms", time.Since(started).Milliseconds()), slog.String("outcome", "success"), slog.Int("exit_code", 0))
	}
	return 0
}

func errorType(err error) string {
	var status interface{ HTTPStatusCode() int }
	if errors.As(err, &status) {
		return "http_status"
	}
	for {
		cause := errors.Unwrap(err)
		if cause == nil {
			return reflect.TypeOf(err).String()
		}
		err = cause
	}
}

func httpStatusCode(err error) int {
	var status interface{ HTTPStatusCode() int }
	if errors.As(err, &status) {
		return status.HTTPStatusCode()
	}
	return 0
}

func safeErrorMessage(message string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, message)
}
