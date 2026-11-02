package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
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
		var status *StatusError
		if errors.As(err, &status) && status.Status == http.StatusNotFound {
			return nil, fmt.Errorf("Vueling sells no fares from %s to %s", origin, destination)
		}
		return nil, err
	}
	return calendarDays(response.Availability.Value.Data)
}

func calendarDays(rows []availabilityDay) ([]vueling.CalendarDay, error) {
	fares := map[time.Time]availabilityDay{}
	for _, row := range rows {
		if row.IsInvalidPrice {
			continue
		}
		if row.Price < 0 {
			return nil, fmt.Errorf("Vueling returned a negative calendar price for %q", row.ArrivalDate)
		}
		if row.Price == 0 {
			continue
		}
		day, err := parseAPITravelDate(row.ArrivalDate)
		if err != nil {
			return nil, fmt.Errorf("Vueling returned invalid calendar date %q: %w", row.ArrivalDate, err)
		}
		if current, duplicate := fares[day]; duplicate && current.Price <= row.Price {
			continue
		}
		fares[day] = row
	}
	dates := make([]time.Time, 0, len(fares))
	for day := range fares {
		dates = append(dates, day)
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	days := make([]vueling.CalendarDay, len(dates))
	for i, day := range dates {
		fare := fares[day]
		days[i] = vueling.CalendarDay{Date: day.Format("02/01/2006"), Fare: vueling.FormatFare(fare.Price, fare.Currency)}
	}
	return days, nil
}

func parseAPITravelDate(value string) (time.Time, error) {
	return time.Parse(time.DateOnly, strings.SplitN(value, "T", 2)[0])
}
