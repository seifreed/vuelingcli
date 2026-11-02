package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
	"github.com/seifreed/vuelingcli/internal/vueling/ports"
)

func (cli *CLI) calendar(args []string) error {
	if isHelp(args) {
		_, err := fmt.Fprint(cli.stdout, "Read Vueling calendar prices or build its link\n\nUsage:\n  vueling calendar <origin city or IATA> <destination city or IATA> --date <date> [--return <date>] [--prices [--month] [--currency code | --browser] [--watch <minutes> [--below <price>]]] [--open] [--json|--toon]\n\n--prices reads the day fares from Vueling's public fare-calendar API over HTTPS; --month lists every day fare of the departure (and return) month.\n--watch re-reads the fare (outbound, or outbound plus return with --return) every N minutes (at least 30), prints the first reading and then only price changes, and stops on Ctrl-C. --below <price> stops with exit 0 once the fare is at or under the price, marking that line BELOW. --json prints one JSON object per line; --watch cannot be combined with --toon, --month or --browser.\n--browser reads the prices from the rendered calendar page in Chrome instead of the API.\n--open also opens the calendar URL in the default browser when reading prices.\n")
		return err
	}
	options, err := parseCalendarOptions(args, time.Now())
	if err != nil {
		return err
	}
	if options.currencySet && cli.api == nil {
		return fmt.Errorf("--currency requires the Vueling API client: %w", cli.requireAPI())
	}
	link, err := vueling.CalendarURL(options.origin, options.destination, options.date)
	if err != nil {
		return err
	}
	if options.open {
		if err := cli.openLink(link); err != nil {
			return fmt.Errorf("open calendar link: %w", err)
		}
	}
	if options.prices {
		request := calendarRequest{origin: options.origin, destination: options.destination, date: options.date, returnDate: options.returnDate, currency: options.currency, month: options.month}
		if options.watch > 0 {
			return cli.watchCalendarPrices(link, request, options)
		}
		var reader ports.PageReader
		if options.browser || cli.api == nil {
			reader, err = cli.pageReader("")
			if err != nil {
				return err
			}
		}
		calendar, err := cli.readCalendar(context.Background(), options.browser, reader, link, request)
		if err != nil {
			return err
		}
		if options.month {
			return cli.calendarMonth(link, calendar, options.returnDate, options.format)
		}
		return cli.calendarPrices(link, calendar.Price, options)
	}
	if options.format != "" {
		return cli.writeStructured(map[string]string{"url": link}, options.format)
	}
	_, err = fmt.Fprintln(cli.stdout, link)
	return err
}

func (cli *CLI) watchCalendarPrices(link string, request calendarRequest, options calendarOptions) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var reader ports.PageReader
	if cli.api == nil {
		var err error
		reader, err = cli.pageReader("")
		if err != nil {
			return err
		}
	}
	watch := calendarWatch{
		every: time.Duration(options.watch) * time.Minute, route: options.origin + "→" + options.destination, link: link,
		currency: options.currency, threshold: options.below, asJSON: options.format == "json", now: time.Now, wait: sleepOrDone,
	}
	return cli.watchCalendar(ctx, watch, func(ctx context.Context) (map[string]string, error) {
		calendar, err := cli.readCalendar(ctx, false, reader, link, request)
		return calendar.Price, err
	})
}

func (cli *CLI) calendarPrices(link string, price map[string]string, options calendarOptions) error {
	price["url"] = link
	if options.format != "" {
		return cli.writeStructured(price, options.format)
	}
	if _, err := fmt.Fprintf(cli.stdout, "%s %s: %s\n", price["route"], options.date.Format("02/01/2006"), price["fare"]); err != nil {
		return err
	}
	if options.returnDate == "" {
		return nil
	}
	_, err := fmt.Fprintf(cli.stdout, "%s %s: %s\n", price["returnRoute"], options.returnDate, price["returnFare"])
	return err
}

func (cli *CLI) calendarMonth(link string, calendar vueling.CalendarMonth, returnDate, format string) error {
	cheapest, ok := vueling.CheapestDay(calendar.Days)
	if !ok {
		return fmt.Errorf("Vueling did not render readable calendar fares")
	}
	result := map[string]any{"url": link, "days": calendar.Days, "cheapest": cheapest}
	for key, value := range calendar.Price {
		result[key] = value
	}
	var returnCheapest vueling.CalendarDay
	if returnDate != "" {
		if returnCheapest, ok = vueling.CheapestDay(calendar.ReturnDays); !ok {
			return fmt.Errorf("Vueling did not render readable return calendar fares")
		}
		result["returnDays"], result["returnCheapest"] = calendar.ReturnDays, returnCheapest
	}
	if format != "" {
		return cli.writeStructured(result, format)
	}
	if err := cli.printCalendarDays(calendar.Price["route"], calendar.Days, cheapest); err != nil {
		return err
	}
	if returnDate != "" {
		if err := cli.printCalendarDays(calendar.Price["returnRoute"], calendar.ReturnDays, returnCheapest); err != nil {
			return err
		}
	}
	return nil
}

func (cli *CLI) printCalendarDays(route string, days []vueling.CalendarDay, cheapest vueling.CalendarDay) error {
	if _, err := fmt.Fprintln(cli.stdout, route); err != nil {
		return err
	}
	for _, day := range days {
		mark := ""
		if day == cheapest {
			mark = " [cheapest]"
		}
		if _, err := fmt.Fprintf(cli.stdout, "%s: %s%s\n", day.Date, day.Fare, mark); err != nil {
			return err
		}
	}
	return nil
}
