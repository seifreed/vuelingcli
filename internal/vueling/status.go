package vueling

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

const statusPage = "https://www.vueling.com/es/servicios-vueling/informacion-de-vuelos/estado-de-vuelos"

type FlightStatus struct {
	Flight      string `json:"flight"`
	Date        string `json:"date"`
	Status      string `json:"status"`
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
	Departure   string `json:"departure"`
	Arrival     string `json:"arrival"`
	Details     string `json:"details"`
}

// The status form has no deep link, so the script fills it step by step on each poll.
const flightStatusScript = `(() => {
  const params = new URL(location.href).searchParams;
  const flight = (params.get('vuelingcliFlight') || '').toUpperCase();
  const day = params.get('vuelingcliDay') || '';
  const state = document.body.dataset;
  const cards = [...document.querySelectorAll('.flight-cards__item')].map(c => (c.innerText || '').split('\n').map(s => s.trim()).filter(Boolean)).filter(c => c.includes(flight));
  if (state.vuelingcliSubmitted === '1') {
    if (!cards.length && document.querySelector('vy-spa-wrapper-flight-status .vy-alert-message--danger')) return {error: 'found no status for that flight number and date'};
    return cards;
  }
  const input = document.getElementById('flightNumberInput');
  if (!input) {
    document.getElementById('TabSearcherFlightStatusByFlightNumberlabel')?.click();
    return [];
  }
  if (input.value !== flight) {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(input, flight);
    input.dispatchEvent(new Event('input', {bubbles: true}));
    input.dispatchEvent(new Event('change', {bubbles: true}));
    return [];
  }
  if (state.vuelingcliDay !== '1') {
    const button = document.querySelector('button.vy-day_button[data-date="' + day + '"]');
    if (!button) {
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
	destinPattern  = regexp.MustCompile(`\p{Ll}([A-Z]{3})$`)
	statusFlightRe = regexp.MustCompile(`^VY\d{3,4}$`)
)

// FlightStatusFromBrowser reads the public flight-status card for one VY flight and day.
func FlightStatusFromBrowser(ctx context.Context, endpoint, flight string, date time.Time) ([]FlightStatus, error) {
	if !validFlightNumber(flight) {
		return nil, fmt.Errorf("flight must be VY followed by three or four digits")
	}
	u, _ := url.Parse(statusPage)
	u.RawQuery = url.Values{"vuelingcliFlight": {flight}, "vuelingcliDay": {fmt.Sprintf("%d-%d", date.Day(), int(date.Month())-1)}}.Encode()
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	var output []byte
	var err error
	if endpoint == "" && runtime.GOOS == "darwin" {
		output, err = readPageWithAppleEvents(ctx, u.String(), flightStatusScript)
	} else {
		output, err = readPageWithDevTools(ctx, endpoint, u.String(), flightStatusScript)
	}
	if err != nil {
		return nil, err
	}
	var failure struct{ Error string }
	if json.Unmarshal(output, &failure) == nil && failure.Error != "" {
		return nil, fmt.Errorf("Vueling %s", failure.Error)
	}
	var cards [][]string
	if err := json.Unmarshal(output, &cards); err != nil {
		return nil, fmt.Errorf("could not decode the rendered Vueling flight status")
	}
	statuses := make([]FlightStatus, 0, len(cards))
	for _, card := range cards {
		statuses = append(statuses, parseStatusCard(card, date))
	}
	return statuses, nil
}

func parseStatusCard(lines []string, date time.Time) FlightStatus {
	status := FlightStatus{Date: date.Format("2006-01-02"), Details: strings.Join(lines, " ")}
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
	return status
}

func readPageWithDevTools(ctx context.Context, endpoint, pageURL, pageScript string) ([]byte, error) {
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
	if err := chromedp.Run(tab, chromedp.Navigate(pageURL)); err != nil {
		return nil, fmt.Errorf("open Vueling in Chrome: %w", err)
	}
	for {
		var result string
		if err := chromedp.Run(tab, chromedp.Evaluate("JSON.stringify("+pageScript+")", &result)); err == nil && result != "" && result != "[]" {
			return []byte(result), nil
		}
		select {
		case <-tab.Done():
			return nil, fmt.Errorf("Vueling did not render results before the timeout")
		case <-time.After(500 * time.Millisecond):
		}
	}
}
