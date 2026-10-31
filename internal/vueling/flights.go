// Package vueling builds Vueling links and reads rendered fares, flights, and flight status from Chrome.
package vueling

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// Flight is one rendered Vueling offer.
type Flight struct {
	Direction string `json:"direction"`
	Date      string `json:"date"`
	Flight    string `json:"flight"`
	Departure string `json:"departure"`
	Arrival   string `json:"arrival"`
	Fare      string `json:"fare"`
	Currency  string `json:"currency"`
	Itinerary string `json:"itinerary"`
	// Stops counts connections; Flights lists every VY flight number on the card
	// in order, and Via the connection airports it names ("Conexión en BCN").
	Stops   int      `json:"stops"`
	Flights []string `json:"flights,omitempty"`
	Via     []string `json:"via,omitempty"`
}

type renderedFlight struct {
	Direction string `json:"direction"`
	Label     string `json:"label"`
	Details   string `json:"details"`
}

var (
	flightNumberPattern = regexp.MustCompile(`\bVY\d{3,4}\b`)
	timePattern         = regexp.MustCompile(`\b(?:[01]?\d|2[0-3]):[0-5]\d\b`)
	farePattern         = regexp.MustCompile(`(?i)([0-9]+(?:[.,][0-9]{1,2})?)\s*(EUR|USD|SEK|NOK|DKK|CHF|GBP|€|£|\$)`)
)

const renderedFlightsScript = `(() => {
  const headings = [...document.querySelectorAll('h1,h2,h3,h4,[role="heading"]')]
    .filter(e => /^(IDA|VUELTA)\s*:/i.test((e.innerText || '').trim()));
  const buttons = [...document.querySelectorAll('button')];
  const offers = buttons.filter(b => /seleccionar vuelo por/i.test([b.innerText, b.getAttribute('aria-label'), b.textContent].join(' ')));
  if (offers.length === 0) {
    const search = buttons.find(b => (b.innerText || '').trim().toUpperCase() === 'BUSCAR');
    if (search && search.dataset.vuelingcliSubmitted !== '1') {
      search.dataset.vuelingcliSubmitted = '1';
      search.click();
    }
  }
  return offers
    .map(b => {
      const heading = headings.filter(h => h.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING).pop();
      const label = [b.getAttribute('aria-label'), b.innerText, b.textContent].filter(Boolean).join(' ').trim();
      let card = b;
      for (let i = 0; i < 8 && card.parentElement; i++) {
        card = card.parentElement;
        const text = card.innerText || '';
        const offers = [...card.querySelectorAll('button')].filter(e => /seleccionar vuelo por/i.test([e.innerText, e.getAttribute('aria-label'), e.textContent].join(' ')));
        if (offers.length === 1 && /VY\d{3,4}/.test(text) && /Vueling/i.test(text) && text.split('\n').length >= 4) break;
      }
      return {
        direction: heading ? (/^VUELTA/i.test(heading.innerText.trim()) ? 'return' : 'outbound') : '',
        label,
        details: (card.innerText || '').trim()
      };
    });
})()`

const renderedPriceScript = `(() => {
  const params = new URL(location.href).searchParams;
  const day = params.get('dateComplete')?.slice(0, 2);
  const tables = [...document.querySelectorAll('table')];
  const findFare = (table, value) => [...(table?.querySelectorAll('button') || [])].find(b => new RegExp('^' + Number(value) + '\\s').test((b.innerText || '').trim()) && /(?:€|EUR|£|USD|\\$)/.test(b.innerText || ''))?.innerText.match(/[0-9]+(?:[.,][0-9]{1,2})?\s*(?:EUR|USD|SEK|NOK|DKK|CHF|GBP|€|£|\\$)/i)?.[0]?.trim();
  const fare = findFare(tables[0], day);
  const month = params.get('vuelingcliMonth') === '1';
  const dayFares = table => month ? [...(table?.querySelectorAll('button') || [])].map(b => (b.innerText || '').trim()).filter(t => /(?:€|EUR|£|USD|\$)/.test(t)).map(t => ({day: t.match(/^(\d{1,2})\s/)?.[1], fare: t.match(/[0-9]+(?:[.,][0-9]{1,2})?\s*(?:EUR|USD|SEK|NOK|DKK|CHF|GBP|€|£|\$)/i)?.[0]?.trim()})).filter(d => d.day && d.fare) : [];
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
  const returnRoute = document.body.innerText.split('\n').map(t => t.trim()).find(t => /^VUELTA:/i.test(t));
  if (month) return days.length && route && (!returnDate || returnDays.length) ? {route, date: params.get('dateComplete'), fare, returnRoute, returnDate, returnFare, days, returnDays} : [];
  const priced = table => [...(table?.querySelectorAll('button') || [])].some(b => /(?:€|EUR|£|USD|\$)/.test(b.innerText || ''));
  if (route && priced(tables[0]) && !fare) return {error: 'shows no fare for the requested date'};
  if (route && returnDate && priced(tables[1]) && !returnFare) return {error: 'shows no fare for the requested return date'};
  return fare && route && (!returnDate || returnFare) ? {route, date: params.get('dateComplete'), fare, returnRoute, returnDate, returnFare} : [];
})()`

// CalendarPricesFromBrowser reads the requested outbound fare from the public rendered calendar.
func CalendarPricesFromBrowser(ctx context.Context, endpoint, calendarURL, returnDate string) (map[string]string, error) {
	calendar, err := readCalendar(ctx, endpoint, calendarURL, returnDate, false)
	return calendar.Price, err
}

// CalendarMonthFromBrowser also reads every visible day fare of the outbound and return months.
func CalendarMonthFromBrowser(ctx context.Context, endpoint, calendarURL, returnDate string) (CalendarMonth, error) {
	return readCalendar(ctx, endpoint, calendarURL, returnDate, true)
}

func readCalendar(ctx context.Context, endpoint, calendarURL, returnDate string, month bool) (CalendarMonth, error) {
	if _, err := url.ParseRequestURI(calendarURL); err != nil {
		return CalendarMonth{}, fmt.Errorf("invalid Vueling calendar URL: %w", err)
	}
	if returnDate != "" || month {
		u, _ := url.Parse(calendarURL)
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
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	if endpoint == "" && runtime.GOOS == "darwin" {
		output, err := readPageWithAppleEvents(ctx, calendarURL, renderedPriceScript)
		if err != nil {
			return CalendarMonth{}, err
		}
		return decodeCalendar(output)
	}
	if endpoint == "" {
		endpoint = "http://127.0.0.1:9222"
	}
	if err := validateDevToolsEndpoint(endpoint); err != nil {
		return CalendarMonth{}, err
	}
	if err := checkDevToolsEndpoint(endpoint); err != nil {
		return CalendarMonth{}, err
	}
	allocator, stopAllocator := chromedp.NewRemoteAllocator(ctx, endpoint)
	defer stopAllocator()
	tab, stopTab := chromedp.NewContext(allocator)
	defer stopTab()
	var output []byte
	if err := chromedp.Run(tab, chromedp.Navigate(calendarURL), chromedp.Sleep(5*time.Second), chromedp.Evaluate(renderedPriceScript, &output)); err != nil {
		return CalendarMonth{}, fmt.Errorf("read Vueling calendar in Chrome: %w", err)
	}
	return decodeCalendar(output)
}

func decodeCalendar(output []byte) (CalendarMonth, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(output, &fields); err != nil || len(fields) == 0 {
		return CalendarMonth{}, fmt.Errorf("Vueling did not render the requested calendar price")
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
			return CalendarMonth{}, fmt.Errorf("Vueling did not render the requested calendar price")
		}
	}
	if message := price["error"]; message != "" {
		return CalendarMonth{}, fmt.Errorf("Vueling %s", message)
	}
	return CalendarMonth{Price: price, Days: monthDays(days, price["date"]), ReturnDays: monthDays(returnDays, price["returnDate"])}, nil
}

// FlightsFromBrowser reads only visible result-card text from Chrome. It never
// reads cookies, storage, request headers, or tokens.
func FlightsFromBrowser(ctx context.Context, endpoint, searchURL, outboundDate, returnDate string) ([]Flight, error) {
	if _, err := url.ParseRequestURI(searchURL); err != nil {
		return nil, fmt.Errorf("invalid Vueling search URL: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	var cards []renderedFlight
	if endpoint == "" && runtime.GOOS == "darwin" {
		output, err := readPageWithAppleEvents(ctx, searchURL, renderedFlightsScript)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(output, &cards); err != nil {
			return nil, fmt.Errorf("could not decode rendered Vueling offers")
		}
	} else {
		if endpoint == "" {
			endpoint = "http://127.0.0.1:9222"
		}
		if err := validateDevToolsEndpoint(endpoint); err != nil {
			return nil, err
		}
		if err := checkDevToolsEndpoint(endpoint); err != nil {
			return nil, err
		}
		allocator, stopAllocator := chromedp.NewRemoteAllocator(ctx, endpoint)
		defer stopAllocator()
		tab, stopTab := chromedp.NewContext(allocator)
		defer stopTab()
		if err := chromedp.Run(tab, chromedp.Navigate(searchURL)); err != nil {
			return nil, fmt.Errorf("open Vueling in Chrome: %w", err)
		}
		deadline := time.Now().Add(45 * time.Second)
		for time.Now().Before(deadline) {
			if err := chromedp.Run(tab, chromedp.Evaluate(renderedFlightsScript, &cards)); err != nil {
				select {
				case <-tab.Done():
					return nil, fmt.Errorf("Chrome closed before Vueling results loaded")
				case <-time.After(500 * time.Millisecond):
					continue
				}
			}
			if len(cards) > 0 {
				break
			}
			select {
			case <-tab.Done():
				return nil, fmt.Errorf("Chrome closed before Vueling results loaded")
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
	if len(cards) == 0 {
		return nil, fmt.Errorf("Vueling did not render flight offers; the session may need attention or Vueling may be temporarily unavailable")
	}

	flights := make([]Flight, 0, len(cards))
	for _, card := range cards {
		flight, err := parseRenderedFlight(card, outboundDate, returnDate)
		if err != nil {
			return nil, err
		}
		flights = append(flights, flight)
	}
	return flights, nil
}

func readPageWithAppleEvents(ctx context.Context, searchURL, pageScript string) ([]byte, error) {
	const script = `on run argv
	set searchURL to item 1 of argv
	set pageScript to item 2 of argv
	tell application "Google Chrome"
		if (count of windows) is 0 then error "VUELING_NO_CHROME_WINDOW"
		set resultTab to make new tab at end of tabs of front window with properties {URL:searchURL}
		repeat 90 times
			try
				set resultJSON to execute resultTab javascript ("JSON.stringify(" & pageScript & ")")
				if resultJSON is not missing value and resultJSON is not "[]" then
					close resultTab
					return resultJSON
				end if
			on error errorText
				if errorText contains "Executing JavaScript through AppleScript is turned off" then error "VUELING_JS_DISABLED"
			end try
			delay 0.5
		end repeat
		error "VUELING_NO_OFFERS"
	end tell
end run`
	cmd := exec.CommandContext(ctx, "osascript", "-e", script, searchURL, pageScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.ToLower(string(output))
		if strings.Contains(message, "vueling_js_disabled") {
			return nil, fmt.Errorf("Chrome blocks JavaScript from Apple Events; enable View > Developer > Allow JavaScript from Apple Events, then retry")
		}
		if strings.Contains(message, "vueling_no_offers") {
			return nil, fmt.Errorf("Vueling did not render results; the tab was left open in Chrome so you can check it, and a manual search there shows whether Vueling is unavailable or the session needs attention. Retry later rather than in a loop")
		}
		return nil, fmt.Errorf("could not read the Vueling tab through Chrome Apple Events: %w", err)
	}
	return []byte(strings.TrimSpace(string(output))), nil
}

func checkDevToolsEndpoint(endpoint string) error {
	u, _ := url.Parse(endpoint)
	u.Path, u.RawQuery = "/json/version", ""
	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(u.String())
	if err != nil {
		return fmt.Errorf("Chrome DevTools is unavailable at %s; enable local remote debugging for the Chrome profile holding Vueling, then retry (%w)", endpoint, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Chrome DevTools returned HTTP %d at %s", response.StatusCode, endpoint)
	}
	return nil
}

func validateDevToolsEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "http" || u.Hostname() == "" || u.User != nil {
		return fmt.Errorf("Chrome DevTools endpoint must be a local http URL, such as http://127.0.0.1:9222")
	}
	if u.Hostname() != "localhost" && !net.ParseIP(u.Hostname()).IsLoopback() {
		return fmt.Errorf("Chrome DevTools endpoint must use localhost or a loopback IP")
	}
	return nil
}

func parseRenderedFlight(card renderedFlight, outboundDate, returnDate string) (Flight, error) {
	flightNumber := flightNumberPattern.FindString(card.Label + "\n" + card.Details)
	times := timePattern.FindAllString(card.Label+"\n"+card.Details, -1)
	fare := farePattern.FindStringSubmatch(card.Label)
	if flightNumber == "" || len(times) < 2 || len(fare) != 3 {
		return Flight{}, fmt.Errorf("could not parse a rendered Vueling flight card")
	}
	direction, date := card.Direction, outboundDate
	switch direction {
	case "return":
		date = returnDate
	case "":
		return Flight{}, fmt.Errorf("could not determine whether a rendered flight is outbound or return")
	}
	currency := strings.ToUpper(fare[2])
	switch currency {
	case "€":
		currency = "EUR"
	case "£":
		currency = "GBP"
	case "$":
		currency = "USD"
	}
	itinerary := strings.Join(strings.Fields(card.Details), " ")
	legs := readLegs(flightNumber, itinerary)
	return Flight{
		Direction: direction,
		Date:      date,
		Flight:    flightNumber,
		Departure: times[0],
		Arrival:   times[1],
		Fare:      strings.ReplaceAll(fare[1], ",", "."),
		Currency:  currency,
		Itinerary: itinerary,
		Stops:     legs.stops,
		Flights:   legs.flights,
		Via:       legs.via,
	}, nil
}
