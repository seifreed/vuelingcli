package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	dateparse "github.com/seifreed/vuelingcli/internal/date"
	"github.com/seifreed/vuelingcli/internal/vueling"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

func (cli *CLI) calendar(args []string) error {
	if isHelp(args) {
		_, err := fmt.Fprint(cli.stdout, "Read Vueling calendar prices or build its link\n\nUsage:\n  vueling calendar <origin city or IATA> <destination city or IATA> --date <date> [--return <date>] [--prices [--month]] [--open] [--json|--toon]\n")
		return err
	}
	var positional []string
	dateValue, returnValue, format := "", "", ""
	open, prices, month := false, false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--open":
			open = true
		case a == "--prices":
			prices = true
		case a == "--month":
			month = true
		case a == "--date":
			remaining := args[i:]
			if len(remaining) < 2 {
				return fmt.Errorf("--date requires a value")
			}
			dateValue = remaining[1]
			i++
		case a == "--return":
			remaining := args[i:]
			if len(remaining) < 2 {
				return fmt.Errorf("--return requires a value")
			}
			returnValue = remaining[1]
			i++
		case len(a) > 9 && a[:9] == "--return=":
			returnValue = a[9:]
		case len(a) > 7 && a[:7] == "--date=":
			dateValue = a[7:]
		case a == "--json" || a == "--toon":
			if format != "" {
				return fmt.Errorf("--json and --toon cannot be used together")
			}
			format = a[2:]
		case len(a) > 0 && a[0] == '-':
			return fmt.Errorf("unknown option %s", a)
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) != 2 || dateValue == "" {
		return fmt.Errorf("usage: vueling calendar <origin city or IATA> <destination city or IATA> --date <date> [--return <date>] [--prices [--month]] [--open] [--json|--toon]")
	}
	if month && !prices {
		return fmt.Errorf("--month requires --prices")
	}
	origin, err := resolveAirport(positional[0])
	if err != nil {
		return fmt.Errorf("origin: %w", err)
	}
	destination, err := resolveAirport(positional[1])
	if err != nil {
		return fmt.Errorf("destination: %w", err)
	}
	date, err := dateparse.ParseDate(dateValue, time.Now())
	if err != nil {
		return err
	}
	var returnDate string
	if returnValue != "" {
		parsed, err := dateparse.ParseDate(returnValue, time.Now())
		if err != nil {
			return err
		}
		if parsed.Before(date) {
			return fmt.Errorf("return date must be on or after departure date")
		}
		if parsed.Sub(date) > 330*24*time.Hour {
			return fmt.Errorf("return date must be within 330 days of departure for calendar price lookup")
		}
		returnDate = parsed.Format("02/01/2006")
	}
	link, err := vueling.CalendarURL(origin, destination, date)
	if err != nil {
		return err
	}
	reader := cli.newReader("")
	if month {
		return cli.calendarMonth(reader, link, returnDate, format)
	}
	if prices {
		price, err := web.CalendarPricesFromPage(context.Background(), reader, link, returnDate)
		if err != nil {
			return err
		}
		price["url"] = link
		if format == "json" {
			return json.NewEncoder(cli.stdout).Encode(price)
		}
		if format == "toon" {
			return cli.writeTOON(price)
		}
		_, err = fmt.Fprintf(cli.stdout, "%s %s: %s\n", price["route"], date.Format("02/01/2006"), price["fare"])
		return err
	}
	if open {
		if err := openURL(link); err != nil {
			return fmt.Errorf("open calendar link: %w", err)
		}
	}
	if format == "json" {
		return json.NewEncoder(cli.stdout).Encode(map[string]string{"url": link})
	}
	if format == "toon" {
		return cli.writeTOON(map[string]string{"url": link})
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
	if format == "json" {
		return json.NewEncoder(cli.stdout).Encode(result)
	}
	if format == "toon" {
		return cli.writeTOON(result)
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
