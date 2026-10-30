package airports

import "strings"

type Airport struct {
	Code string `json:"code"`
	City string `json:"city"`
}

// shortcut: this starter catalogue covers common Vueling stations only; replace with the airline's full route catalogue when a stable source is available.
var catalogue = []Airport{
	{"BCN", "Barcelona"}, {"MAD", "Madrid"}, {"AGP", "Málaga"}, {"ALC", "Alicante"},
	{"BIO", "Bilbao"}, {"VLC", "Valencia"}, {"SVQ", "Sevilla"}, {"PMI", "Palma de Mallorca"},
	{"IBZ", "Ibiza"}, {"MAH", "Menorca"}, {"LPA", "Gran Canaria"}, {"TFS", "Tenerife Sur"},
	{"FUE", "Fuerteventura"}, {"ACE", "Lanzarote"}, {"SCQ", "Santiago de Compostela"}, {"VGO", "Vigo"},
	{"ORY", "París Orly"}, {"CDG", "París Charles de Gaulle"}, {"FCO", "Roma Fiumicino"}, {"MXP", "Milán Malpensa"},
	{"AMS", "Ámsterdam"}, {"LHR", "Londres Heathrow"}, {"LGW", "Londres Gatwick"}, {"BRU", "Bruselas"},
	{"LIS", "Lisboa"}, {"OPO", "Oporto"}, {"FRA", "Fráncfort"}, {"MUC", "Múnich"},
	{"GVA", "Ginebra"}, {"ZRH", "Zúrich"}, {"EDI", "Edimburgo"}, {"CPH", "Copenhague"},
}

func Search(query string) []Airport {
	q := normalize(query)
	matches := []Airport{}
	for _, airport := range catalogue {
		if q == "" || strings.Contains(normalize(airport.City), q) || strings.EqualFold(airport.Code, q) {
			matches = append(matches, airport)
		}
	}
	return matches
}

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u").Replace(s)
}
