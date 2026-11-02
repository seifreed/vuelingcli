package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

type bestPricesResponse struct {
	BestPrices struct {
		Value []struct {
			DestinationCode  string `json:"destinationCode"`
			IsDirect         bool   `json:"isDirect"`
			DepartureDate    string `json:"departureDate"`
			IntegerPartPrice int    `json:"integerPartPrice"`
			DecimalPartPrice int    `json:"decimalPartPrice"`
		} `json:"value"`
		IsSuccess bool `json:"isSuccess"`
		Error     struct {
			Description string `json:"description"`
		} `json:"error"`
	} `json:"bestPrices"`
}

// Deals returns the cheapest fare to each destination Vueling flies to from origin, cheapest first.
// Vueling answers only for the destinations it sells from origin, so callers may pass every known station.
func (c *Client) Deals(ctx context.Context, origin string, destinations []string, from time.Time, months int, currency string) ([]vueling.Deal, error) {
	query := url.Values{
		"originCode":       {origin},
		"destinationCodes": {strings.Join(destinations, ",")},
		"months":           {strconv.Itoa(months)},
		"startDate":        {from.Format(time.DateOnly) + "T00:00:00"},
		"currencyCode":     {currency},
	}
	var response bestPricesResponse
	if err := c.getJSON(ctx, c.APIW+"/api/v1/bestPrices?"+query.Encode(), wwwOrigin, &response); err != nil {
		return nil, fmt.Errorf("read Vueling deals: %w", err)
	}
	if !response.BestPrices.IsSuccess {
		if description := response.BestPrices.Error.Description; description != "" {
			return nil, fmt.Errorf("Vueling deals: %s", description)
		}
		return nil, errors.New("Vueling returned no deals")
	}
	deals := make([]vueling.Deal, 0, len(response.BestPrices.Value))
	byDestination := map[string]int{}
	for _, row := range response.BestPrices.Value {
		destination := strings.ToUpper(strings.TrimSpace(row.DestinationCode))
		if !vueling.IsIATACode(destination) || destination == strings.ToUpper(strings.TrimSpace(origin)) {
			return nil, fmt.Errorf("Vueling returned invalid deal destination %q", row.DestinationCode)
		}
		departure, err := parseAPITravelDate(row.DepartureDate)
		if err != nil {
			return nil, fmt.Errorf("Vueling returned invalid deal date %q: %w", row.DepartureDate, err)
		}
		if row.IntegerPartPrice < 0 || row.DecimalPartPrice < 0 || row.DecimalPartPrice > 99 {
			return nil, fmt.Errorf("Vueling returned invalid deal price %d.%02d", row.IntegerPartPrice, row.DecimalPartPrice)
		}
		deal := vueling.Deal{
			Destination: destination,
			// The timestamp is in UTC; the day matches Vueling's fare calendar.
			Date:     departure.Format(time.DateOnly),
			Price:    float64(row.IntegerPartPrice) + float64(row.DecimalPartPrice)/100,
			Currency: currency,
			Direct:   row.IsDirect,
		}
		if i, ok := byDestination[destination]; ok {
			if deal.Price < deals[i].Price {
				deals[i] = deal
			}
			continue
		}
		byDestination[destination] = len(deals)
		deals = append(deals, deal)
	}
	sort.SliceStable(deals, func(i, j int) bool { return deals[i].Price < deals[j].Price })
	return deals, nil
}
