package cli

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/airports"
	"github.com/seifreed/vuelingcli/internal/vueling"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

const calendarCurrency = "EUR"

// calendarRequest names the fares to read; returnDate is DD/MM/YYYY or empty for one-way.
type calendarRequest struct {
	origin, destination string
	date                time.Time
	returnDate          string
	month               bool
}

// readCalendar reads calendar fares from Vueling's JSON API, or from the rendered page in Chrome
// when browser is set or no API client is configured.
func (cli *CLI) readCalendar(ctx context.Context, browser bool, reader web.PageReader, link string, request calendarRequest) (vueling.CalendarMonth, error) {
	if !browser && cli.api != nil {
		return cli.apiCalendar(ctx, request)
	}
	if request.month {
		return web.CalendarMonthFromPage(ctx, reader, link, request.returnDate)
	}
	price, err := web.CalendarPricesFromPage(ctx, reader, link, request.returnDate)
	return vueling.CalendarMonth{Price: price}, err
}

func (cli *CLI) apiCalendar(ctx context.Context, request calendarRequest) (vueling.CalendarMonth, error) {
	date := request.date.Format("02/01/2006")
	outbound, err := cli.api.Calendar(ctx, request.origin, request.destination, calendarCurrency)
	if err != nil {
		return vueling.CalendarMonth{}, err
	}
	fare, err := requestedFare(outbound, date, request.month, "Vueling shows no fare for the requested date")
	if err != nil {
		return vueling.CalendarMonth{}, err
	}
	price := map[string]string{"route": routeLabel("IDA", request.origin), "date": date, "fare": fare, "returnRoute": "", "returnDate": request.returnDate, "returnFare": ""}
	calendar := vueling.CalendarMonth{Price: price}
	if request.month {
		calendar.Days = vueling.MonthOf(outbound, date)
	}
	if request.returnDate == "" {
		return calendar, nil
	}
	inbound, err := cli.api.Calendar(ctx, request.destination, request.origin, calendarCurrency)
	if err != nil {
		return vueling.CalendarMonth{}, err
	}
	if price["returnFare"], err = requestedFare(inbound, request.returnDate, request.month, "Vueling shows no fare for the requested return date"); err != nil {
		return vueling.CalendarMonth{}, err
	}
	price["returnRoute"] = routeLabel("VUELTA", request.destination)
	if request.month {
		calendar.ReturnDays = vueling.MonthOf(inbound, request.returnDate)
	}
	return calendar, nil
}

// requestedFare returns the fare of date; a missing fare is an error unless the whole month was requested.
func requestedFare(days []vueling.CalendarDay, date string, month bool, missing string) (string, error) {
	if day, ok := vueling.FareOn(days, date); ok {
		return day.Fare, nil
	}
	if month {
		return "", nil
	}
	return "", errors.New(missing)
}

// routeLabel mirrors the calendar page's leg heading, such as "IDA: BARCELONA".
func routeLabel(leg, code string) string {
	city, ok := airports.City(code)
	if !ok {
		city = code
	}
	return leg + ": " + strings.ToUpper(city)
}
