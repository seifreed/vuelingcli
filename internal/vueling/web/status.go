package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

const statusPage = "https://www.vueling.com/es/servicios-vueling/informacion-de-vuelos/estado-de-vuelos"

// The status form has no deep link, so the script fills it step by step on each poll.
const flightStatusScript = `(() => {
  const params = new URL(location.href).searchParams;
  const flight = (params.get('vuelingcliFlight') || '').toUpperCase();
  const route = [['originInput', params.get('vuelingcliFrom') || ''], ['destinationInput', params.get('vuelingcliTo') || '']];
  const day = params.get('vuelingcliDay') || '';
  const state = document.body.dataset;
  const setValue = (input, value) => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(input, value);
    input.dispatchEvent(new Event('input', {bubbles: true}));
    input.dispatchEvent(new Event('change', {bubbles: true}));
  };
  const cards = [...document.querySelectorAll('.flight-cards__item')].map(c => (c.innerText || '').split('\n').map(s => s.trim()).filter(Boolean)).filter(c => !flight || c.includes(flight));
  if (state.vuelingcliSubmitted === '1') {
    if (!cards.length && document.querySelector('vy-spa-wrapper-flight-status .vy-alert-message--danger')) return {error: 'found no flight status for that search and date'};
    return cards;
  }
  if (flight) {
    const input = document.getElementById('flightNumberInput');
    if (!input) {
      document.getElementById('TabSearcherFlightStatusByFlightNumberlabel')?.click();
      return [];
    }
    if (input.value !== flight) {
      setValue(input, flight);
      return [];
    }
  } else {
    for (const [id, code] of route) {
      if (state[id] === '1') continue;
      const input = document.getElementById(id);
      if (!input) {
        document.getElementById('TabSearcherFlightStatusByStationlabel')?.click();
        return [];
      }
      const option = [...document.querySelectorAll('button.vy-list-dropdown_item')].find(b => b.offsetParent && new RegExp('\\b' + code + '$').test((b.innerText || '').trim()));
      if (option && input.value === code) {
        option.click();
        state[id] = '1';
      } else if (Number(state[id + 'Polls'] || 0) % 6 === 0) {
        input.click();
        input.focus();
        setValue(input, '');
        setValue(input, code);
      }
      state[id + 'Polls'] = Number(state[id + 'Polls'] || 0) + 1;
      return [];
    }
  }
  if (state.vuelingcliDay !== '1') {
    const button = document.querySelector('button.vy-day_button[data-date="' + day + '"]');
    if (!button) {
      if (document.querySelector('button.vy-day_button')) return {error: 'does not offer flight status for that date'};
      document.getElementById('outboundDate')?.click();
      return [];
    }
    if (button.getAttribute('aria-disabled') === 'true') return {error: 'does not offer flight status for that date'};
    button.click();
    state.vuelingcliDay = '1';
    return [];
  }
  const search = [...document.querySelectorAll('button')].find(b => b.offsetParent && /^BUSCAR/i.test((b.innerText || '').trim()));
  if (search) {
    search.click();
    state.vuelingcliSubmitted = '1';
  }
  return [];
})()`

var (
	clockPattern   = regexp.MustCompile(`^\d{1,2}:\d{2}$`)
	originPattern  = regexp.MustCompile(`^([A-Z]{3})\p{Lu}\p{Ll}`)
	destinPattern  = regexp.MustCompile(`[\p{Ll})]([A-Z]{3})$`)
	statusFlightRe = regexp.MustCompile(`^VY\d{2,4}[A-Z]?$`)
)

// FlightStatusFromPage reads public flight-status cards for one VY flight, or for a route when flight is empty.
func FlightStatusFromPage(ctx context.Context, reader PageReader, flight, origin, destination string, date time.Time) ([]vueling.FlightStatus, error) {
	query := url.Values{"vuelingcliDay": {fmt.Sprintf("%d-%d", date.Day(), int(date.Month())-1)}}
	switch {
	case statusFlightRe.MatchString(flight):
		query.Set("vuelingcliFlight", flight)
	case flight != "":
		return nil, fmt.Errorf("flight must be VY followed by its number, such as VY1158 or VY392D")
	case vueling.IsIATACode(origin) && vueling.IsIATACode(destination) && origin != destination:
		query.Set("vuelingcliFrom", origin)
		query.Set("vuelingcliTo", destination)
	default:
		return nil, fmt.Errorf("route needs two different three-letter IATA codes")
	}
	pageURL := statusPage + "?" + query.Encode()
	output, err := readPage(ctx, reader, pageURL, flightStatusScript, 90*time.Second, "Vueling did not render flight status before the timeout")
	if err != nil {
		return nil, err
	}
	var failure struct{ Error string }
	if json.Unmarshal(output, &failure) == nil && failure.Error != "" {
		return nil, fmt.Errorf("Vueling %s", failure.Error)
	}
	var cards [][]string
	if err := json.Unmarshal(output, &cards); err != nil {
		return nil, fmt.Errorf("could not decode the rendered Vueling flight status: %w", err)
	}
	if len(cards) == 0 {
		return nil, fmt.Errorf("Vueling did not render any flight status cards")
	}
	statuses := make([]vueling.FlightStatus, 0, len(cards))
	for i, card := range cards {
		status, err := parseStatusCard(card, date)
		if err != nil {
			return nil, fmt.Errorf("flight status card %d: %w", i+1, err)
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func parseStatusCard(lines []string, date time.Time) (vueling.FlightStatus, error) {
	status := vueling.FlightStatus{Date: date.Format("2006-01-02"), Details: strings.Join(lines, " ")}
	if len(lines) > 0 {
		status.Status = lines[0]
	}
	var times []string
	for _, line := range lines {
		switch {
		case statusFlightRe.MatchString(line):
			status.Flight = line
		case clockPattern.MatchString(line):
			times = append(times, line)
		case status.Origin == "" && originPattern.MatchString(line):
			status.Origin = originPattern.FindStringSubmatch(line)[1]
		case destinPattern.MatchString(line):
			status.Destination = destinPattern.FindStringSubmatch(line)[1]
		}
	}
	if len(times) > 0 {
		status.Departure, status.Arrival = times[0], times[len(times)-1]
	}
	if status.Flight == "" || status.Status == "" {
		return vueling.FlightStatus{}, fmt.Errorf("could not parse a rendered Vueling flight status card")
	}
	return status, nil
}
