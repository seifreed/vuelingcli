package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

const maxScheduleMonths = 12

var spanishMonths = [12]string{"ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}

type scheduleOptions struct {
	origin, destination string
	from                time.Time
	months              int
	currency, format    string
	days                bool
}

func (cli *CLI) schedule(args []string) error {
	if isHelp(args) {
		_, err := fmt.Fprint(cli.stdout, "Show on which days Vueling flies a route\n\nUsage:\n  vueling schedule <origin city or IATA> <destination city or IATA> [--from <date>] [--months 1-12] [--days] [--currency code] [--json|--toon]\n\nLists, month by month from --from (default today) for --months (default 3), the days Vueling sells a one-way flight and their weekdays.\n--days also prints each day's cheapest flight, its departure time and its fare.\nReads the availability behind the website's date picker, which covers about eleven months ahead; later days show no flights yet.\n")
		return err
	}
	options, err := parseScheduleOptions(args, time.Now())
	if err != nil {
		return err
	}
	if err := cli.requireAPI(); err != nil {
		return err
	}
	days, err := cli.api.Schedule(context.Background(), options.origin, options.destination, options.currency, options.from, options.months)
	if err != nil {
		return err
	}
	schedule := vueling.NewSchedule(options.origin, options.destination, days, options.from, options.months)
	if options.format != "" {
		return cli.writeStructured(schedule, options.format)
	}
	return cli.printSchedule(schedule, options.days)
}

func parseScheduleOptions(args []string, now time.Time) (scheduleOptions, error) {
	var positional []string
	options := scheduleOptions{months: 3}
	fromValue, monthsValue, currencyValue := "", "", ""
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--days":
			options.days = true
		case a == "--json" || a == "--toon":
			requested, err := setOutputFormat(options.format, a)
			if err != nil {
				return scheduleOptions{}, err
			}
			options.format = requested
		case strings.HasPrefix(a, "--"):
			key, value, next, err := parseLongOption(args, i)
			if err != nil {
				return scheduleOptions{}, err
			}
			switch key {
			case "from":
				fromValue = value
			case "months":
				monthsValue = value
			case "currency":
				currencyValue = value
			default:
				return scheduleOptions{}, fmt.Errorf("unknown option --%s", key)
			}
			i = next
		case strings.HasPrefix(a, "-"):
			return scheduleOptions{}, fmt.Errorf("unknown option %s", a)
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) != 2 {
		return scheduleOptions{}, errors.New("usage: vueling schedule <origin> <destination> [--from <date>] [--months 1-12] [--days] [--currency code] [--json|--toon]")
	}
	var err error
	if options.origin, err = resolveAirport(positional[0]); err != nil {
		return scheduleOptions{}, fmt.Errorf("origin: %w", err)
	}
	if options.destination, err = resolveAirport(positional[1]); err != nil {
		return scheduleOptions{}, fmt.Errorf("destination: %w", err)
	}
	if options.origin == options.destination {
		return scheduleOptions{}, errors.New("origin and destination must differ")
	}
	if options.months, err = parseIntOption(monthsValue, options.months); err != nil || options.months < 1 || options.months > maxScheduleMonths {
		return scheduleOptions{}, fmt.Errorf("--months must be a number from 1 to %d", maxScheduleMonths)
	}
	if options.currency, err = vueling.NormalizeCurrency(currencyValue); err != nil {
		return scheduleOptions{}, err
	}
	if options.from, err = parseOptionalDate(fromValue, now); err != nil {
		return scheduleOptions{}, err
	}
	if fromValue == "" {
		options.from = now
	}
	return options, nil
}

func (cli *CLI) printSchedule(schedule vueling.Schedule, perDay bool) error {
	if _, err := fmt.Fprintf(cli.stdout, "%s → %s · %s a %s\n", schedule.Origin, schedule.Destination, schedule.From, schedule.To); err != nil {
		return err
	}
	summary := "sin vuelos a la venta en estas fechas"
	if len(schedule.Dates) > 0 {
		summary = fmt.Sprintf("%d días · %s · primer vuelo %s · último %s", len(schedule.Dates), strings.Join(schedule.Weekdays, " "), schedule.First, schedule.Last)
	}
	if _, err := fmt.Fprintln(cli.stdout, summary); err != nil {
		return err
	}
	for _, month := range schedule.Months {
		if _, err := fmt.Fprintln(cli.stdout, monthLine(month)); err != nil {
			return err
		}
	}
	if !perDay {
		return nil
	}
	for _, day := range schedule.Dates {
		if _, err := fmt.Fprintln(cli.stdout, dayLine(day)); err != nil {
			return err
		}
	}
	return nil
}

func monthLine(month vueling.ScheduleMonth) string {
	label := month.Month
	if start, err := time.Parse("2006-01", month.Month); err == nil {
		label = fmt.Sprintf("%s %d", spanishMonths[start.Month()-1], start.Year())
	}
	if len(month.Days) == 0 {
		return fmt.Sprintf("%-8s  sin vuelos", label)
	}
	days := make([]string, len(month.Days))
	for i, day := range month.Days {
		days[i] = strconv.Itoa(day)
	}
	return fmt.Sprintf("%-8s  %2d días  %-27s  %s", label, len(month.Days), strings.Join(month.Weekdays, " "), strings.Join(days, " "))
}

func dayLine(day vueling.ScheduledDay) string {
	line := fmt.Sprintf("%s %s  %-7s %s", day.Date, day.Weekday, day.Flight, day.Departure)
	if day.Fare != "" {
		line += "  desde " + day.Fare
	}
	if day.Connection {
		line += "  conexión"
	}
	return line
}
