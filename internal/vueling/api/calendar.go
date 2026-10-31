package api

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

type availabilityResponse struct {
	Availability struct {
		Value struct {
			Data []availabilityDay `json:"data"`
		} `json:"value"`
	} `json:"availability"`
}

type availabilityDay struct {
	ArrivalDate    string  `json:"arrivalDate"`
	Price          float64 `json:"price"`
	Currency       string  `json:"currency"`
	IsInvalidPrice bool    `json:"isInvalidPrice"`
}

// Calendar returns the cheapest fare of each day Vueling sells one way on a route, about eleven months ahead.
func (c *Client) Calendar(ctx context.Context, origin, destination, currency string) ([]vueling.CalendarDay, error) {
	query := url.Values{
		"originCode":                       {origin},
		"destinationCode":                  {destination},
		"providerName":                     {"DmpsRetrieveAvailabilityProvider"},
		"currencyCode":                     {currency},
		"lowPriceClassificationPercentage": {"25"},
		"includeSummary":                   {"true"},
	}
	var response availabilityResponse
	if err := c.getJSON(ctx, c.APIW+"/api/v2/availability?"+query.Encode(), wwwOrigin, &response); err != nil {
		return nil, err
	}
	return calendarDays(response.Availability.Value.Data), nil
}

func calendarDays(rows []availabilityDay) []vueling.CalendarDay {
	fares := map[time.Time]string{}
	for _, row := range rows {
		day, err := time.Parse(time.DateOnly, strings.SplitN(row.ArrivalDate, "T", 2)[0])
		if _, duplicate := fares[day]; err != nil || row.IsInvalidPrice || row.Price <= 0 || duplicate {
			continue
		}
		fares[day] = formatFare(row.Price, row.Currency)
	}
	dates := make([]time.Time, 0, len(fares))
	for day := range fares {
		dates = append(dates, day)
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	days := make([]vueling.CalendarDay, len(dates))
	for i, day := range dates {
		days[i] = vueling.CalendarDay{Date: day.Format("02/01/2006"), Fare: fares[day]}
	}
	return days
}

// formatFare renders an amount the way Vueling's calendar page shows it, such as "48 €" or "48,99 €".
func formatFare(amount float64, currency string) string {
	text := strconv.FormatFloat(amount, 'f', 2, 64)
	text = strings.TrimSuffix(text, ".00")
	text = strings.Replace(text, ".", ",", 1)
	symbol := currency
	if currency == "EUR" || currency == "" {
		symbol = "€"
	}
	return fmt.Sprintf("%s %s", text, symbol)
}
