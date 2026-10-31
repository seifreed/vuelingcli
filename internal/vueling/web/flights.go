// Package web reads and parses Vueling's public rendered pages.
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

type renderedFlight struct {
	Direction string `json:"direction"`
	Label     string `json:"label"`
	Details   string `json:"details"`
}

var (
	flightNumberPattern = regexp.MustCompile(`\bVY\d{3,4}\b`)
	timePattern         = regexp.MustCompile(`\b(?:[01]?\d|2[0-3]):[0-5]\d\b`)
	farePattern         = regexp.MustCompile(`(?i)([0-9]+(?:[.,][0-9]{1,2})?)\s*(EUR|USD|SEK|NOK|DKK|CHF|GBP|€|£|\$)`)
	stopPattern         = regexp.MustCompile(`(?i)\b(escalas?|conexi[oó]n|stops?|connection)\b`)
	connectionPattern   = regexp.MustCompile(`(?i)\bconexi[oó]n en ([A-Z]{3})\b`)
)

const renderedFlightsScript = `(() => {
  const headings = [...document.querySelectorAll('h1,h2,h3,h4,[role="heading"]')]
    .filter(e => /^(IDA|VUELTA)\s*:/i.test((e.innerText || '').trim()));
  const buttons = [...document.querySelectorAll('button')];
  const offers = buttons.filter(b => /seleccionar vuelo por/i.test([b.innerText, b.getAttribute('aria-label'), b.textContent].join(' ')));
  if (offers.length === 0) {
    if (/ESTE D[IÍ]A NO HAY VUELOS/i.test(document.body.innerText || '')) return {error: 'shows no flights for that route on the requested date'};
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
  const hasCurrency = text => /\b(?:EUR|USD|SEK|NOK|DKK|CHF|GBP)\b|[€£$]/i.test(text);
  const findFare = (table, value) => [...(table?.querySelectorAll('button') || [])].find(b => new RegExp('^' + Number(value) + '\\s').test((b.innerText || '').trim()) && hasCurrency(b.innerText || ''))?.innerText.match(/[0-9]+(?:[.,][0-9]{1,2})?\s*(?:EUR|USD|SEK|NOK|DKK|CHF|GBP|€|£|\$)/i)?.[0]?.trim();
  const fare = findFare(tables[0], day);
  const month = params.get('vuelingcliMonth') === '1';
  const dayFares = table => month ? [...(table?.querySelectorAll('button') || [])].map(b => (b.innerText || '').trim()).filter(hasCurrency).map(t => ({day: t.match(/^(\d{1,2})\s/)?.[1], fare: t.match(/[0-9]+(?:[.,][0-9]{1,2})?\s*(?:EUR|USD|SEK|NOK|DKK|CHF|GBP|€|£|\$)/i)?.[0]?.trim()})).filter(d => d.day && d.fare) : [];
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
func CalendarPricesFromPage(ctx context.Context, reader PageReader, calendarURL, returnDate string) (map[string]string, error) {
	calendar, err := readCalendar(ctx, reader, calendarURL, returnDate, false)
	return calendar.Price, err
}

// CalendarMonthFromPage also reads every visible day fare of the outbound and return months.
func CalendarMonthFromPage(ctx context.Context, reader PageReader, calendarURL, returnDate string) (vueling.CalendarMonth, error) {
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
	return vueling.CalendarMonth{Price: price, Days: monthDays(days, price["date"]), ReturnDays: monthDays(returnDays, price["returnDate"])}, nil
}

// FlightsFromPage reads only visible result-card text. It never
// reads cookies, storage, request headers, or tokens.
func FlightsFromPage(ctx context.Context, reader PageReader, searchURL, outboundDate, returnDate string) ([]vueling.Flight, error) {
	if _, err := url.ParseRequestURI(searchURL); err != nil {
		return nil, fmt.Errorf("invalid Vueling search URL: %w", err)
	}

	var cards []renderedFlight
	output, err := readPage(ctx, reader, searchURL, renderedFlightsScript, 45*time.Second, "Vueling did not render flight offers; the session may need attention or Vueling may be temporarily unavailable")
	if err != nil {
		return nil, err
	}
	var failure struct{ Error string }
	if json.Unmarshal(output, &failure) == nil && failure.Error != "" {
		return nil, fmt.Errorf("Vueling %s", failure.Error)
	}
	if err := json.Unmarshal(output, &cards); err != nil {
		return nil, fmt.Errorf("could not decode rendered Vueling offers: %w", err)
	}
	if len(cards) == 0 {
		return nil, fmt.Errorf("Vueling did not render flight offers; the session may need attention or Vueling may be temporarily unavailable")
	}

	flights := make([]vueling.Flight, 0, len(cards))
	for _, card := range cards {
		flight, err := parseRenderedFlight(card, outboundDate, returnDate)
		if err != nil {
			return nil, err
		}
		flights = append(flights, flight)
	}
	return flights, nil
}

func parseRenderedFlight(card renderedFlight, outboundDate, returnDate string) (vueling.Flight, error) {
	flightNumber := flightNumberPattern.FindString(card.Label + "\n" + card.Details)
	times := timePattern.FindAllString(card.Label+"\n"+card.Details, -1)
	fare := farePattern.FindStringSubmatch(card.Label)
	if flightNumber == "" || len(times) < 2 || len(fare) != 3 {
		return vueling.Flight{}, fmt.Errorf("could not parse a rendered Vueling flight card")
	}
	direction, date := card.Direction, outboundDate
	switch direction {
	case "return":
		date = returnDate
	case "":
		return vueling.Flight{}, fmt.Errorf("could not determine whether a rendered flight is outbound or return")
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
	return vueling.Flight{
		Direction: direction,
		Date:      date,
		Flight:    flightNumber,
		Departure: times[0],
		Arrival:   times[len(times)-1],
		Fare:      strings.ReplaceAll(fare[1], ",", "."),
		Currency:  currency,
		Itinerary: itinerary,
		Stops:     legs.stops,
		Flights:   legs.flights,
		Via:       legs.via,
	}, nil
}

type legs struct {
	stops   int
	flights []string
	via     []string
}

func readLegs(flightNumber, itinerary string) legs {
	flights := uniqueInOrder(append([]string{flightNumber}, flightNumberPattern.FindAllString(itinerary, -1)...))
	var airports []string
	for _, match := range connectionPattern.FindAllStringSubmatch(itinerary, -1) {
		airports = append(airports, match[1])
	}
	via := uniqueInOrder(airports)
	stops := max(len(flights)-1, len(via))
	if stops == 0 && stopPattern.MatchString(itinerary) {
		stops = 1
	}
	return legs{stops: stops, flights: flights, via: via}
}

func uniqueInOrder(values []string) []string {
	var unique []string
	for _, value := range values {
		if value != "" && !slices.Contains(unique, value) {
			unique = append(unique, value)
		}
	}
	return unique
}
