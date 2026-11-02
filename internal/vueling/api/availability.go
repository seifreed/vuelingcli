package api

import (
	"errors"
	"fmt"
	"strings"
)

type journeyAvailabilityResponse struct {
	Data struct {
		AMSAvy *amsAvailability `json:"amsAvy"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type amsAvailability struct {
	CurrencyCode string `json:"currencyCode"`
	Trips        []struct {
		Trips []struct {
			JourneysAvailableByMarket []struct {
				Value []journey `json:"value"`
			} `json:"journeysAvailableByMarket"`
		} `json:"trips"`
	} `json:"trips"`
	FaresAvailable []struct {
		Value struct {
			FareAvailabilityKey string `json:"fareAvailabilityKey"`
			Fares               []struct {
				ProductClass   string `json:"productClass"`
				PassengerFares []struct {
					AMSFareAmount float64 `json:"amsFareAmount"`
				} `json:"passengerFares"`
			} `json:"fares"`
		} `json:"value"`
	} `json:"faresAvailable"`
	BundleOffers []struct {
		Key   string `json:"key"`
		Value struct {
			BundleCode   string `json:"bundleCode"`
			BundlePrices []struct {
				TotalPrice    float64 `json:"totalPrice"`
				PassengerType string  `json:"passengerType"`
			} `json:"bundlePrices"`
		} `json:"value"`
	} `json:"bundleOffers"`
}

func (avy *amsAvailability) journeys() []journey {
	var journeys []journey
	for _, trip := range avy.Trips {
		for _, inner := range trip.Trips {
			for _, market := range inner.JourneysAvailableByMarket {
				journeys = append(journeys, market.Value...)
			}
		}
	}
	return journeys
}

// farePrices prices each fare option; an option lists one fare per segment, so a connection costs their sum.
func (avy *amsAvailability) farePrices() (map[string]farePrice, error) {
	prices := map[string]farePrice{}
	for _, entry := range avy.FaresAvailable {
		for _, fare := range entry.Value.Fares {
			if len(fare.PassengerFares) > 0 {
				if fare.PassengerFares[0].AMSFareAmount < 0 {
					return nil, fmt.Errorf("Vueling returned a negative fare amount for fare %q", entry.Value.FareAvailabilityKey)
				}
				price := prices[entry.Value.FareAvailabilityKey]
				price.amount += fare.PassengerFares[0].AMSFareAmount
				price.bundle = bundleName(fare.ProductClass)
				prices[entry.Value.FareAvailabilityKey] = price
			}
		}
	}
	return prices, nil
}

type designator struct {
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
	Departure   string `json:"departure"`
	Arrival     string `json:"arrival"`
}

type flightNumber struct {
	CarrierCode string `json:"carrierCode"`
	Identifier  string `json:"identifier"`
}

type leg struct {
	LegInfo struct {
		OperatedByText *string `json:"operatedByText"`
	} `json:"legInfo"`
}

type segment struct {
	Identifier flightNumber `json:"identifier"`
	Designator designator   `json:"designator"`
	Legs       []leg        `json:"legs"`
}

type fareReference struct {
	FareAvailabilityKey string `json:"fareAvailabilityKey"`
	FareSellKey         string `json:"fareSellKey"`
	Details             []struct {
		BundleReferences []string `json:"bundleReferences"`
	} `json:"details"`
}

type journey struct {
	JourneyKey string          `json:"journeyKey"`
	Designator designator      `json:"designator"`
	Segments   []segment       `json:"segments"`
	Fares      []fareReference `json:"fares"`
}

func availabilityError(response journeyAvailabilityResponse) error {
	var messages []string
	for _, e := range response.Errors {
		if e.Message != "" {
			messages = append(messages, e.Message)
		}
	}
	if len(messages) == 0 {
		return errors.New("Vueling returned no availability data")
	}
	return fmt.Errorf("Vueling availability: %s", strings.Join(messages, "; "))
}
