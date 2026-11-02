package api

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

// passengerConcepts maps the breakdown's per-passenger fare concepts to bundle passenger types.
var passengerConcepts = map[string]string{"JourneyTotalPaxAdult": "ADT", "JourneyTotalPaxChild": "CHD"}

// infantConceptSuffix ends the breakdown's lap-infant concepts, JourneyTotalPaxINFANT and JourneyTotalFeesINFANT;
// the session remembers the INFANT service of the availability search, so the breakdown prices infants itself.
const infantConceptSuffix = "INFANT"

type breakdownResponse struct {
	CurrencyCode string `json:"currencyCode"`
	Flights      []struct {
		FlightInfo []struct {
			DepartureStation string `json:"departureStation"`
			DepartureDate    string `json:"departureDate"`
			ArrivalStation   string `json:"arrivalStation"`
			ArrivalDate      string `json:"arrivalDate"`
		} `json:"flightInfo"`
		Concepts []struct {
			Code          string             `json:"code"`
			ItemCount     int                `json:"itemCount"`
			TotalAmount   float64            `json:"totalAmount"`
			DetailedTaxes map[string]float64 `json:"detailedTaxes"`
		} `json:"concepts"`
	} `json:"flights"`
	TotalAmount float64 `json:"totalAmount"`
}

func breakdownQuote(breakdown breakdownResponse, numbers []string, date time.Time) (vueling.Quote, error) {
	flight := breakdown.Flights[0]
	first, last := flight.FlightInfo[0], flight.FlightInfo[len(flight.FlightInfo)-1]
	quote := vueling.Quote{
		Flight: strings.Join(numbers, "+"), Date: date.Format(time.DateOnly),
		Origin: first.DepartureStation, Destination: last.ArrivalStation,
		Departure: localClock(first.DepartureDate), Arrival: localClock(last.ArrivalDate), ArrivalDate: laterArrivalDate(first.DepartureDate, last.ArrivalDate),
		Currency: breakdown.CurrencyCode, Total: breakdown.TotalAmount,
	}
	for _, concept := range flight.Concepts {
		if _, ok := passengerConcepts[concept.Code]; ok {
			quote.Fare += concept.TotalAmount
			continue
		}
		if strings.HasSuffix(concept.Code, infantConceptSuffix) {
			quote.InfantFee += concept.TotalAmount
			quote.Infants = max(quote.Infants, concept.ItemCount)
			continue
		}
		quote.Taxes += concept.TotalAmount
		for code, amount := range concept.DetailedTaxes {
			if quote.TaxDetail == nil {
				quote.TaxDetail = map[string]float64{}
			}
			quote.TaxDetail[code] += amount
		}
	}
	quote.Fare, quote.Taxes, quote.InfantFee = vueling.RoundCents(quote.Fare), vueling.RoundCents(quote.Taxes), vueling.RoundCents(quote.InfantFee)
	if quote.Total < 0 || quote.Fare < 0 || quote.Taxes < 0 || quote.InfantFee < 0 {
		return vueling.Quote{}, errors.New("Vueling returned a negative amount in the price breakdown")
	}
	for code, amount := range quote.TaxDetail {
		if amount < 0 {
			return vueling.Quote{}, fmt.Errorf("Vueling returned a negative %s tax in the price breakdown", code)
		}
	}
	return quote, nil
}

// breakdownPassengers counts the passengers of each bundle passenger type the breakdown prices.
func breakdownPassengers(breakdown breakdownResponse) (map[string]int, error) {
	passengers := map[string]int{}
	for _, flight := range breakdown.Flights {
		for _, concept := range flight.Concepts {
			_, passenger := passengerConcepts[concept.Code]
			if (passenger || strings.HasSuffix(concept.Code, infantConceptSuffix)) && concept.ItemCount < 0 {
				return nil, fmt.Errorf("Vueling returned a negative passenger count for %s", concept.Code)
			}
			if passengerType, ok := passengerConcepts[concept.Code]; ok {
				passengers[passengerType] = max(passengers[passengerType], concept.ItemCount)
			}
		}
	}
	return passengers, nil
}

// bundlePrices prices every bundle the website offers for fare, cheapest first; a bundle's total adds its
// per-person price for every adult and child to the fare's total; lap infants take no bundle.
func bundlePrices(avy *amsAvailability, fare fareReference, passengers map[string]int, total float64) ([]vueling.Bundle, error) {
	offered := map[string]bool{}
	for _, detail := range fare.Details {
		for _, key := range detail.BundleReferences {
			offered[key] = true
		}
	}
	seen := map[string]bool{}
	var bundles []vueling.Bundle
	for _, offer := range avy.BundleOffers {
		name, shown := vueling.BundleName(offer.Value.BundleCode)
		if !offered[offer.Key] || !shown || seen[offer.Value.BundleCode] {
			continue
		}
		seen[offer.Value.BundleCode] = true
		prices := map[string]float64{}
		for _, price := range offer.Value.BundlePrices {
			if price.TotalPrice < 0 {
				return nil, fmt.Errorf("Vueling returned a negative price for %s bundle and %s passengers", offer.Value.BundleCode, price.PassengerType)
			}
			prices[price.PassengerType] = price.TotalPrice
		}
		if _, ok := prices["ADT"]; !ok {
			return nil, fmt.Errorf("Vueling returned no adult price for %s bundle", offer.Value.BundleCode)
		}
		bundle := vueling.Bundle{Code: offer.Value.BundleCode, Name: name, PricePerPerson: prices["ADT"], Total: total}
		for passengerType, count := range passengers {
			price, ok := prices[passengerType]
			if !ok {
				price = prices["ADT"]
			}
			bundle.Total += price * float64(count)
		}
		bundle.Total = vueling.RoundCents(bundle.Total)
		bundles = append(bundles, bundle)
	}
	slices.SortFunc(bundles, func(a, b vueling.Bundle) int {
		return cmp.Or(cmp.Compare(a.Total, b.Total), cmp.Compare(a.Code, b.Code))
	})
	return bundles, nil
}
