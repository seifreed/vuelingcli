package vueling

import (
	"fmt"
	"sort"
	"time"
)

// TripSearch contains validated filters for pairing outbound and return calendar fares.
type TripSearch struct {
	Origin, Destination  string
	From, To             time.Time
	MinNights, MaxNights int
	MaxPrice             float64
	Limit                int
	Currency             string
}

// CheapestTrips pairs each available departure with the allowed return days, cheapest total first.
func CheapestTrips(outbound, inbound []CalendarDay, search TripSearch) ([]Trip, error) {
	outboundFares := dayFares(outbound)
	inboundFares := dayFares(inbound)
	var trips []Trip
	for day := search.From; !day.After(search.To); day = day.AddDate(0, 0, 1) {
		departure := day.Format("02/01/2006")
		out, ok := outboundFares[departure]
		if !ok {
			continue
		}
		for nights := search.MinNights; nights <= search.MaxNights; nights++ {
			returnDay := day.AddDate(0, 0, nights)
			ret, ok := inboundFares[returnDay.Format("02/01/2006")]
			total := RoundCents(out + ret)
			if !ok || (search.MaxPrice > 0 && total > search.MaxPrice) {
				continue
			}
			link, err := FlightURL(LinkOptions{Origin: search.Origin, Destination: search.Destination, Date: day, ReturnDate: returnDay, Adults: 1, Currency: search.Currency})
			if err != nil {
				return nil, fmt.Errorf("build trip selection link: %w", err)
			}
			trips = append(trips, Trip{Departure: departure, Return: returnDay.Format("02/01/2006"), Nights: nights, Outbound: out, ReturnFare: ret, Total: total, Currency: search.Currency, URL: link})
		}
	}
	sort.SliceStable(trips, func(i, j int) bool { return trips[i].Total < trips[j].Total })
	if search.Limit > 0 && len(trips) > search.Limit {
		trips = trips[:search.Limit]
	}
	return trips, nil
}

func dayFares(days []CalendarDay) map[string]float64 {
	fares := make(map[string]float64, len(days))
	for _, day := range days {
		if amount, ok := ParseFare(day.Fare); ok {
			fares[day.Date] = amount
		}
	}
	return fares
}
