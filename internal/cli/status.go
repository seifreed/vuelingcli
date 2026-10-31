package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/seifreed/vuelingcli/internal/search"
	"github.com/seifreed/vuelingcli/internal/vueling"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

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
			return web.FlightStatusFromPage(ctx, reader, flight, origin, destination, date)
		})
	}
	statuses, err := web.FlightStatusFromPage(context.Background(), reader, flight, origin, destination, date)
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
