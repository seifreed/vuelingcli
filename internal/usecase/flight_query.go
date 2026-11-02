package usecase

import "time"

// FlightQuery contains the route, dates, passengers, discounts, and currency for flight pricing.
type FlightQuery struct {
	Origin, Destination string
	Date, ReturnDate    time.Time
	Adults, Children    int
	Infants, Discount   int
	Currency            string
}

// FlightPick identifies a flight explicitly or asks for the cheapest direct or connecting flight.
type FlightPick struct {
	Flight string
	Direct bool
}
