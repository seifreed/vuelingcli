package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

func (cli *CLI) calendar(args []string) error {
	if isHelp(args) {
		_, err := fmt.Fprint(cli.stdout, "Read Vueling calendar prices or build its link\n\nUsage:\n  vueling calendar <origin city or IATA> <destination city or IATA> --date <date> [--return <date>] [--prices [--month]] [--open] [--json|--toon]\n\n--open also opens the calendar URL in the default browser when reading prices.\n")
		return err
	}
	options, err := parseCalendarOptions(args, time.Now())
	if err != nil {
		return err
	}
	link, err := vueling.CalendarURL(options.origin, options.destination, options.date)
	if err != nil {
		return err
	}
	if options.open {
		if err := openURL(link); err != nil {
			return fmt.Errorf("open calendar link: %w", err)
		}
	}
	if options.month {
		return cli.calendarMonth(cli.newReader(""), link, options.returnDate, options.format)
	}
	if options.prices {
		price, err := web.CalendarPricesFromPage(context.Background(), cli.newReader(""), link, options.returnDate)
		if err != nil {
			return err
		}
		price["url"] = link
		if options.format != "" {
			return cli.writeStructured(price, options.format)
		}
		_, err = fmt.Fprintf(cli.stdout, "%s %s: %s\n", price["route"], options.date.Format("02/01/2006"), price["fare"])
		return err
	}
	if options.format != "" {
		return cli.writeStructured(map[string]string{"url": link}, options.format)
	}
	_, err = fmt.Fprintln(cli.stdout, link)
	return err
}

func (cli *CLI) calendarMonth(reader web.PageReader, link, returnDate, format string) error {
	calendar, err := web.CalendarMonthFromPage(context.Background(), reader, link, returnDate)
	if err != nil {
		return err
	}
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
