package vueling

import (
	"errors"
	"strings"
)

// QuoteTripTotal adds quote totals and rounds the combined amount to cents.
func QuoteTripTotal(quotes []Quote) float64 {
	total := 0.0
	for _, quote := range quotes {
		total += quote.Total
	}
	return RoundCents(total)
}

// QuoteSelectionURL builds the official selection link for one or two quoted flights.
func QuoteSelectionURL(options LinkOptions, quotes []Quote) (string, error) {
	if len(quotes) < 1 || len(quotes) > 2 {
		return "", errors.New("flight selection URL requires one or two quotes")
	}
	options.OutboundFlight = firstQuotedFlight(quotes[0].Flight)
	if len(quotes) == 2 {
		options.ReturnFlight = firstQuotedFlight(quotes[1].Flight)
	}
	return FlightURL(options)
}

func firstQuotedFlight(flight string) string {
	return strings.SplitN(flight, "+", 2)[0]
}
