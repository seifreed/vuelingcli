package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

const (
	tripsMaxNights     = 30
	tripsDefaultWindow = 30
	tripsHorizonMonths = 11
)

type tripsOptions struct {
	origin, destination  string
	from, to             time.Time
	minNights, maxNights int
	maxPrice             float64
	limit                int
	currency             string
	format               string
}

func parseTripsOptions(args []string, now time.Time) (tripsOptions, error) {
	options := tripsOptions{}
	var positional []string
	values := map[string]string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--json" || a == "--toon":
			format, err := setOutputFormat(options.format, a)
			if err != nil {
				return tripsOptions{}, err
			}
			options.format = format
		case strings.HasPrefix(a, "--"):
			key, value, next, err := parseLongOption(args, i)
			if err != nil {
				return tripsOptions{}, err
			}
			switch key {
			case "from", "to", "nights", "max", "limit", "currency":
				values[key], i = value, next
			default:
				return tripsOptions{}, fmt.Errorf("unknown option --%s", key)
			}
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) != 2 || values["nights"] == "" {
		return tripsOptions{}, errors.New("usage: vueling trips <origin> <destination> --nights N[-M] [--from date] [--to date] [--max price] [--limit N]")
	}
	var err error
	if options.origin, err = resolveAirport(positional[0]); err != nil {
		return tripsOptions{}, fmt.Errorf("origin: %w", err)
	}
	if options.destination, err = resolveAirport(positional[1]); err != nil {
		return tripsOptions{}, fmt.Errorf("destination: %w", err)
	}
	if options.origin == options.destination {
		return tripsOptions{}, errors.New("origin and destination must be different airports")
	}
	if options.minNights, options.maxNights, err = parseNights(values["nights"]); err != nil {
		return tripsOptions{}, err
	}
	if options.from, options.to, err = parseTripWindow(values["from"], values["to"], options.maxNights, now); err != nil {
		return tripsOptions{}, err
	}
	if options.limit, err = parseNonNegativeLimit(values["limit"], 10); err != nil {
		return tripsOptions{}, err
	}
	if values["max"] != "" {
		if options.maxPrice, err = parsePositivePrice(values["max"], "--max"); err != nil {
			return tripsOptions{}, err
		}
	}
	if options.currency, err = vueling.NormalizeCurrency(values["currency"]); err != nil {
		return tripsOptions{}, err
	}
	return options, nil
}

// parseNights reads a stay such as "4" or "3-5".
func parseNights(value string) (int, int, error) {
	low, high, isRange := strings.Cut(value, "-")
	if !isRange {
		high = low
	}
	minNights, lowErr := strconv.Atoi(strings.TrimSpace(low))
	maxNights, highErr := strconv.Atoi(strings.TrimSpace(high))
	if lowErr != nil || highErr != nil || minNights < 1 || maxNights > tripsMaxNights || minNights > maxNights {
		return 0, 0, fmt.Errorf("--nights must be N or N-M with 1 ≤ N ≤ M ≤ %d", tripsMaxNights)
	}
	return minNights, maxNights, nil
}

// parseTripWindow reads the departure window; the latest return must fall within Vueling's fare calendar.
func parseTripWindow(fromValue, toValue string, maxNights int, now time.Time) (time.Time, time.Time, error) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	from := today
	var err error
	if fromValue != "" {
		if from, err = parseOptionalDate(fromValue, now); err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --from: %w", err)
		}
	}
	to := from.AddDate(0, 0, tripsDefaultWindow)
	if toValue != "" {
		if to, err = parseOptionalDate(toValue, now); err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --to: %w", err)
		}
		if to.Before(from) {
			return time.Time{}, time.Time{}, errors.New("--to must be on or after --from")
		}
	}
	horizon := today.AddDate(0, tripsHorizonMonths, 0)
	if from.AddDate(0, 0, maxNights).After(horizon) {
		return time.Time{}, time.Time{}, fmt.Errorf("the return must be by %s, the end of Vueling's fare calendar", horizon.Format("02/01/2006"))
	}
	if latest := horizon.AddDate(0, 0, -maxNights); to.After(latest) {
		if toValue != "" {
			return time.Time{}, time.Time{}, fmt.Errorf("--to must be by %s so the return falls within Vueling's fare calendar", latest.Format("02/01/2006"))
		}
		to = latest
	}
	return from, to, nil
}
