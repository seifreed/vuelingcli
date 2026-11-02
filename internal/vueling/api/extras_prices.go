package api

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

// offeredBundles lists the codes of the bundles the website shows for fare.
func offeredBundles(avy *amsAvailability, fare fareReference) []string {
	referenced := map[string]bool{}
	for _, detail := range fare.Details {
		for _, key := range detail.BundleReferences {
			referenced[key] = true
		}
	}
	var codes []string
	for _, offer := range avy.BundleOffers {
		code := offer.Value.BundleCode
		if _, shown := vueling.BundleName(code); shown && referenced[offer.Key] && !slices.Contains(codes, code) {
			codes = append(codes, code)
		}
	}
	return codes
}

type seatMapResponse struct {
	SeatMaps []struct {
		PassengerFees []struct {
			Value struct {
				Groups []struct {
					GroupNumber int     `json:"groupNumber"`
					Amount      float64 `json:"amount"`
				} `json:"groups"`
			} `json:"value"`
		} `json:"passengerFees"`
		SeatMap struct {
			Decks []struct {
				Value struct {
					Compartments []struct {
						Value struct {
							Sections []seatSection `json:"sections"`
						} `json:"value"`
					} `json:"compartments"`
				} `json:"value"`
			} `json:"decks"`
		} `json:"seatMap"`
	} `json:"seatMaps"`
}

type seatSection struct {
	SeatFam struct {
		Name string `json:"name"`
	} `json:"seatFam"`
	Units []struct {
		Type            string `json:"type"`
		Group           int    `json:"group"`
		Assignable      bool   `json:"assignable"`
		IsSeatAvailable bool   `json:"isSeatAvailable"`
	} `json:"units"`
}

// seatPrices spans each seat type's prices for the first passenger, per flight leg; Vueling numbers seat
// maps in the order of the journey's segments, and prices every seat of a type by its price group.
func (m seatMapResponse) seatPrices(j journey, currency string) ([]vueling.ExtraPrice, error) {
	var prices []vueling.ExtraPrice
	for i, seatMap := range m.SeatMaps {
		if len(seatMap.PassengerFees) == 0 {
			continue
		}
		groups := map[int]float64{}
		for _, group := range seatMap.PassengerFees[0].Value.Groups {
			if group.Amount < 0 {
				return nil, fmt.Errorf("Vueling returned a negative seat price for group %d", group.GroupNumber)
			}
			groups[group.GroupNumber] = group.Amount
		}
		segment := ""
		if len(m.SeatMaps) > 1 && i < len(j.Segments) {
			s := j.Segments[i]
			segment = s.Identifier.CarrierCode + s.Identifier.Identifier + " " + s.Designator.Origin + "-" + s.Designator.Destination
		}
		var types []vueling.ExtraPrice
		for _, deck := range seatMap.SeatMap.Decks {
			for _, compartment := range deck.Value.Compartments {
				for _, section := range compartment.Value.Sections {
					types = addSeatSection(types, section, groups, segment, currency)
				}
			}
		}
		prices = append(prices, types...)
	}
	return prices, nil
}

// addSeatSection widens the price span of the section's seat type with its assignable seats.
func addSeatSection(types []vueling.ExtraPrice, section seatSection, groups map[int]float64, segment, currency string) []vueling.ExtraPrice {
	name, shown := vueling.SeatTypeName(section.SeatFam.Name)
	if !shown {
		return types
	}
	for _, unit := range section.Units {
		price, priced := groups[unit.Group]
		if unit.Type != "NormalSeat" || !unit.Assignable || !priced {
			continue
		}
		i := slices.IndexFunc(types, func(p vueling.ExtraPrice) bool { return p.Code == section.SeatFam.Name })
		if i < 0 {
			types = append(types, vueling.ExtraPrice{Code: section.SeatFam.Name, Name: name, Segment: segment, Price: price, MaxPrice: price, Available: new(int), Currency: currency, PerPassenger: true})
			i = len(types) - 1
		}
		types[i].Price, types[i].MaxPrice = math.Min(types[i].Price, price), math.Max(types[i].MaxPrice, price)
		if unit.IsSeatAvailable {
			*types[i].Available++
		}
	}
	return types
}

type servicesResponse struct {
	CurrencyCode       string `json:"currencyCode"`
	PaxSegmentsService []struct {
		ServiceCode        string   `json:"serviceCode"`
		ApplyPax           string   `json:"applyPax"`
		Weight             int      `json:"weight"`
		Price              *float64 `json:"price"`
		PaxJourneySegments []struct {
			PaxPrice        *float64              `json:"paxPrice"`
			JourneySegments []journeySegmentPrice `json:"journeySegments"`
		} `json:"paxJourneySegments"`
	} `json:"paxSegmentsService"`
}

// prices splits the paid services into bags and everything else; free services, such as assistance or
// what the bundle includes, are left out. A service sold per passenger is priced for the first passenger.
func (s servicesResponse) prices(currency string) (bags, services []vueling.ExtraPrice, err error) {
	for _, service := range s.PaxSegmentsService {
		amount := 0.0
		switch {
		case service.Price != nil:
			amount = *service.Price
		case len(service.PaxJourneySegments) > 0:
			amount = passengerPrice(service.PaxJourneySegments[0].PaxPrice, service.PaxJourneySegments[0].JourneySegments)
		}
		if amount < 0 {
			return nil, nil, fmt.Errorf("Vueling returned a negative price for service %s", service.ServiceCode)
		}
		if amount == 0 {
			continue
		}
		price := vueling.ExtraPrice{Code: service.ServiceCode, Name: vueling.ServiceName(service.ServiceCode), Price: vueling.RoundCents(amount), Currency: currency, PerPassenger: service.ApplyPax != "AllPax"}
		if service.Weight > 0 {
			price.Name = fmt.Sprintf("Checked bag (%d kg)", service.Weight)
		}
		if isBag(service.ServiceCode, service.Weight) {
			bags = append(bags, price)
		} else {
			services = append(services, price)
		}
	}
	slices.SortStableFunc(bags, func(a, b vueling.ExtraPrice) int { return cmp.Compare(a.Price, b.Price) })
	return bags, services, nil
}

type journeySegmentPrice struct {
	JourneyPrice *float64 `json:"journeyPrice"`
	SegmentPrice *float64 `json:"segmentPrice"`
}

// passengerPrice is a passenger's price for the whole journey: one price, or the sum of its journeys' or segments' prices.
func passengerPrice(whole *float64, parts []journeySegmentPrice) float64 {
	if whole != nil {
		return *whole
	}
	total := 0.0
	for _, part := range parts {
		switch {
		case part.JourneyPrice != nil:
			total += *part.JourneyPrice
		case part.SegmentPrice != nil:
			total += *part.SegmentPrice
		}
	}
	return total
}

func isBag(code string, weight int) bool {
	return weight > 0 || code == "GCBG" || code == "CBHG" || code == "HBBG"
}
