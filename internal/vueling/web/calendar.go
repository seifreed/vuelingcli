package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

type renderedDay struct {
	Day  string `json:"day"`
	Fare string `json:"fare"`
}

const renderedPriceScript = `(() => {
  const params = new URL(location.href).searchParams;
  const day = params.get('dateComplete')?.slice(0, 2);
  const tables = [...document.querySelectorAll('table')];
  const hasCurrency = text => /\b(?:EUR|USD|SEK|NOK|DKK|CHF|GBP)\b|[€£$]/i.test(text);
  const findFare = (table, value) => [...(table?.querySelectorAll('button') || [])].find(b => new RegExp('^' + Number(value) + '\\s').test((b.innerText || '').trim()) && hasCurrency(b.innerText || ''))?.innerText.match(/[-−]?[0-9]+(?:[.,][0-9]{3})*(?:[.,][0-9]{1,2})?\s*(?:EUR|USD|SEK|NOK|DKK|CHF|GBP|€|£|\$)/i)?.[0]?.trim();
  const fare = findFare(tables[0], day);
  const month = params.get('vuelingcliMonth') === '1';
  const dayFares = table => month ? [...(table?.querySelectorAll('button') || [])].map(b => (b.innerText || '').trim()).filter(hasCurrency).map(t => ({day: t.match(/^(\d{1,2})\s/)?.[1], fare: t.match(/[-−]?[0-9]+(?:[.,][0-9]{3})*(?:[.,][0-9]{1,2})?\s*(?:EUR|USD|SEK|NOK|DKK|CHF|GBP|€|£|\$)/i)?.[0]?.trim()})).filter(d => d.day && d.fare) : [];
  const days = dayFares(tables[0]);
  const returnDate = params.get('vuelingcliReturnDate') || '';
  const monthNames = ['enero','febrero','marzo','abril','mayo','junio','julio','agosto','septiembre','octubre','noviembre','diciembre'];
  const selectedMonths = [...document.querySelectorAll('button[aria-pressed="true"]')];
  const expectedMonth = returnDate ? monthNames[Number(returnDate.slice(3,5)) - 1] : '';
  const selectedReturnMonth = (selectedMonths[1]?.innerText || '').normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase();
  if (returnDate && selectedMonths.length > 1) {
    if (!selectedReturnMonth.startsWith(expectedMonth)) {
      const matches = [...document.querySelectorAll('button')].filter(b => (b.innerText || '').trim().normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase().startsWith(expectedMonth));
      if (matches.length > 1) matches[1].click();
      else return {error: 'does not show the requested return month in the visible month tabs; use calendar --open to inspect it'};
      return [];
    }
  }
  let returnFare = '';
  let returnDays = [];
  if (returnDate && selectedReturnMonth.startsWith(expectedMonth)) {
    returnFare = findFare(tables[1], returnDate.slice(0, 2));
    returnDays = dayFares(tables[1]);
  }
  if (returnDate && !selectedReturnMonth.startsWith(expectedMonth)) return [];
  const route = document.body.innerText.split('\n').map(t => t.trim()).find(t => /^IDA:/i.test(t));
  const returnRoute = returnDate ? document.body.innerText.split('\n').map(t => t.trim()).find(t => /^VUELTA:/i.test(t)) || '' : '';
  if (month) return days.length && route && (!returnDate || returnDays.length) ? {route, date: params.get('dateComplete'), fare, returnRoute, returnDate, returnFare, days, returnDays} : [];
  const priced = table => [...(table?.querySelectorAll('button') || [])].some(b => hasCurrency(b.innerText || ''));
  if (route && priced(tables[0]) && !fare) return {error: 'shows no fare for the requested date'};
  if (route && returnDate && priced(tables[1]) && !returnFare) return {error: 'shows no fare for the requested return date'};
  return fare && route && (!returnDate || returnFare) ? {route, date: params.get('dateComplete'), fare, returnRoute, returnDate, returnFare} : [];
})()`

// CalendarPricesFromPage reads the requested outbound fare from the public rendered calendar.
func (Source) CalendarPricesFromPage(ctx context.Context, reader PageReader, calendarURL, returnDate string) (map[string]string, error) {
	calendar, err := readCalendar(ctx, reader, calendarURL, returnDate, false)
	return calendar.Price, err
}

// CalendarMonthFromPage also reads every visible day fare of the outbound and return months.
func (Source) CalendarMonthFromPage(ctx context.Context, reader PageReader, calendarURL, returnDate string) (vueling.CalendarMonth, error) {
	return readCalendar(ctx, reader, calendarURL, returnDate, true)
}

func readCalendar(ctx context.Context, reader PageReader, calendarURL, returnDate string, month bool) (vueling.CalendarMonth, error) {
	u, err := url.ParseRequestURI(calendarURL)
	if err != nil {
		return vueling.CalendarMonth{}, fmt.Errorf("invalid Vueling calendar URL: %w", err)
	}
	if returnDate != "" || month {
		query := u.Query()
		if returnDate != "" {
			query.Set("vuelingcliReturnDate", returnDate)
		}
		if month {
			query.Set("vuelingcliMonth", "1")
		}
		u.RawQuery = query.Encode()
		calendarURL = u.String()
	}
	output, err := readPage(ctx, reader, calendarURL, renderedPriceScript, 90*time.Second, "Vueling did not render the requested calendar price")
	if err != nil {
		return vueling.CalendarMonth{}, err
	}
	return decodeCalendar(output)
}

func decodeCalendar(output []byte) (vueling.CalendarMonth, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(output, &fields); err != nil {
		return vueling.CalendarMonth{}, fmt.Errorf("could not decode rendered Vueling calendar: %w", err)
	}
	if len(fields) == 0 {
		return vueling.CalendarMonth{}, fmt.Errorf("Vueling did not render the requested calendar price")
	}
	price := map[string]string{}
	var days, returnDays []renderedDay
	for key, value := range fields {
		var err error
		switch key {
		case "days":
			err = json.Unmarshal(value, &days)
		case "returnDays":
			err = json.Unmarshal(value, &returnDays)
		default:
			var text string
			err = json.Unmarshal(value, &text)
			price[key] = text
		}
		if err != nil {
			return vueling.CalendarMonth{}, fmt.Errorf("could not decode rendered Vueling calendar field %q: %w", key, err)
		}
	}
	if message := price["error"]; message != "" {
		return vueling.CalendarMonth{}, fmt.Errorf("Vueling %s", message)
	}
	for _, fare := range []string{price["fare"], price["returnFare"]} {
		if fare != "" {
			if _, ok := vueling.ParseFare(fare); !ok {
				return vueling.CalendarMonth{}, fmt.Errorf("Vueling returned an unreadable calendar fare %q", fare)
			}
		}
	}
	for _, fareDays := range [][]renderedDay{days, returnDays} {
		for _, day := range fareDays {
			if _, ok := vueling.ParseFare(day.Fare); !ok {
				return vueling.CalendarMonth{}, fmt.Errorf("Vueling returned an unreadable calendar fare %q", day.Fare)
			}
		}
	}
	return vueling.CalendarMonth{Price: price, Days: monthDays(days, price["date"]), ReturnDays: monthDays(returnDays, price["returnDate"])}, nil
}

func monthDays(rendered []renderedDay, reference string) []vueling.CalendarDay {
	if len(reference) != len("02/01/2006") {
		return nil
	}
	seen := map[string]bool{}
	var days []vueling.CalendarDay
	for _, day := range rendered {
		number, err := strconv.Atoi(day.Day)
		if err != nil {
			continue
		}
		date := fmt.Sprintf("%02d%s", number, reference[2:])
		if _, err := time.Parse("02/01/2006", date); err != nil || seen[date] {
			continue
		}
		seen[date] = true
		days = append(days, vueling.CalendarDay{Date: date, Fare: day.Fare})
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Date < days[j].Date })
	return days
}
