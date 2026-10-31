package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

func (cli *CLI) flightStatus(args []string) error {
	if isHelp(args) {
		if _, err := fmt.Fprint(cli.stdout, "Read a Vueling flight's live status\n\nUsage:\n  vueling status <VY flight number> [--date <date>] [--watch <minutes>] [--browser [--cdp-url http://127.0.0.1:9222]] [--json|--toon]\n  vueling status <origin city or IATA> <destination city or IATA> [--date <date>] [--watch <minutes>] [--browser [--cdp-url http://127.0.0.1:9222]] [--json|--toon]\n\nThe date defaults to today; past dates are accepted. This command reads Vueling's public flight-status API directly.\nTimes are local airport times as Vueling currently shows them; details include scheduled times, terminals, gates, and baggage belts.\n--browser (implied by --cdp-url) instead fills Vueling's flight-status form in your Chrome session and reads the rendered cards; it only accepts today or later.\n--watch re-reads every N minutes (at least 5), prints only cards whose status or times changed, and stops once every flight has landed or been cancelled, or on Ctrl-C. It cannot be combined with --toon.\n"); err != nil {
			return err
		}
		return nil
	}
	options, err := parseFlightStatusOptions(args)
	if err != nil {
		return err
	}
	read := cli.statusSource(options)
	if options.watch > 0 {
		return cli.watchFlightStatus(time.Duration(options.watch)*time.Minute, options.format == "json", read)
	}
	statuses, err := read(context.Background())
	if err != nil {
		return err
	}
	if options.format != "" {
		return cli.writeStructured(statuses, options.format)
	}
	for _, s := range statuses {
		if _, err := fmt.Fprintln(cli.stdout, statusLine(s)); err != nil {
			return err
		}
	}
	return nil
}

// statusSource reads Vueling's flight-status API unless --browser asked for Chrome or no API client is configured.
func (cli *CLI) statusSource(options flightStatusOptions) func(context.Context) ([]vueling.FlightStatus, error) {
	if options.browser || cli.api == nil {
		reader := cli.newReader(options.endpoint)
		return func(ctx context.Context) ([]vueling.FlightStatus, error) {
			return web.FlightStatusFromPage(ctx, reader, options.flight, options.origin, options.destination, options.date)
		}
	}
	return func(ctx context.Context) ([]vueling.FlightStatus, error) {
		return cli.api.FlightStatus(ctx, options.flight, options.origin, options.destination, options.date)
	}
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
			if _, writeErr := fmt.Fprintf(cli.stderr, "status: %v (retrying in %s)\n", err, every); writeErr != nil {
				return errors.Join(err, fmt.Errorf("write status warning: %w", writeErr))
			}
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
