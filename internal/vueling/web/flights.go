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
	farePattern         = regexp.MustCompile(`(?i)([-−]?[0-9]+(?:[.,][0-9]{3})*(?:[.,][0-9]{1,2})?)\s*(EUR|USD|SEK|NOK|DKK|CHF|GBP|€|£|\$)`)
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

// FlightsFromPage reads only visible result-card text. It never
// reads cookies, storage, request headers, or tokens.
func (Source) FlightsFromPage(ctx context.Context, reader PageReader, searchURL, outboundDate, returnDate string) ([]vueling.Flight, error) {
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
	if strings.HasPrefix(fare[1], "-") || strings.HasPrefix(fare[1], "−") {
		return vueling.Flight{}, fmt.Errorf("Vueling returned a negative fare for flight %s", flightNumber)
	}
	amount, ok := vueling.ParseFare(fare[1])
	if !ok {
		return vueling.Flight{}, fmt.Errorf("could not parse rendered Vueling fare %q", fare[1])
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
		Fare:      vueling.FormatCompactAmount(amount),
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
