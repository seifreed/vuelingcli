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
	read, err := cli.statusSource(options)
	if err != nil {
		return err
	}
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

// statusSource reads from the API unless --browser was requested or no API client is configured.
func (cli *CLI) statusSource(options flightStatusOptions) (func(context.Context) ([]vueling.FlightStatus, error), error) {
	if options.browser || cli.api == nil {
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		switch {
		case options.date.Before(today):
			return nil, errors.New("invalid --date: browser status lookups need today or later")
		case !browserStatusYearMatches(options.date, today):
			return nil, errors.New("browser status lookups cannot distinguish the requested year for that month and day")
		}
	}
	if options.browser || cli.api == nil {
		reader, err := cli.pageReader(options.endpoint)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context) ([]vueling.FlightStatus, error) {
			source, err := cli.webPageSource()
			if err != nil {
				return nil, err
			}
			return source.FlightStatusFromPage(ctx, reader, options.flight, options.origin, options.destination, options.date)
		}, nil
	}
	return func(ctx context.Context) ([]vueling.FlightStatus, error) {
		return cli.api.FlightStatus(ctx, options.flight, options.origin, options.destination, options.date)
	}, nil
}

func browserStatusYearMatches(date, today time.Time) bool {
	for year := today.Year(); ; year++ {
		candidate := time.Date(year, date.Month(), date.Day(), 0, 0, 0, 0, today.Location())
		if candidate.Month() != date.Month() || candidate.Day() != date.Day() || candidate.Before(today) {
			continue
		}
		return candidate.Year() == date.Year()
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
	err := watchPoll(ctx, every, read,
		func(statuses []vueling.FlightStatus, _ bool) (bool, error) {
			for _, s := range vueling.ChangedStatuses(previous, statuses) {
				if asJSON {
					if err := encoder.Encode(s); err != nil {
						return false, err
					}
				} else {
					if _, err := fmt.Fprintf(cli.stdout, "%s %s\n", time.Now().Format("15:04"), statusLine(s)); err != nil {
						return false, err
					}
				}
			}
			if vueling.AllFinal(statuses) {
				return true, nil
			}
			previous = statuses
			return false, nil
		},
		func(err error) error { return cli.writeWatchWarning("status", every, err) },
		sleepOrDone,
	)
	if ctx.Err() != nil {
		cli.logWatchCancellation("status")
	}
	return err
}
