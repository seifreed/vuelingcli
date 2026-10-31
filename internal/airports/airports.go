// Package airports resolves city names and IATA codes for Vueling stations.
package airports

import "strings"

// Airport is a Vueling station.
type Airport struct {
	Code string `json:"code"`
	City string `json:"city"`
}

// shortcut: this is a local snapshot of Vueling's airport-help and destination pages; refresh it when those lists change.
var catalogue = []Airport{
	{"BCN", "Barcelona"}, {"MAD", "Madrid"}, {"AGP", "Málaga"}, {"ALC", "Alicante"},
	{"BIO", "Bilbao"}, {"VLC", "Valencia"}, {"SVQ", "Sevilla"}, {"PMI", "Palma de Mallorca"},
	{"IBZ", "Ibiza"}, {"MAH", "Menorca"}, {"LPA", "Gran Canaria"}, {"TFS", "Tenerife Sur"},
	{"FUE", "Fuerteventura"}, {"ACE", "Lanzarote"}, {"SCQ", "Santiago de Compostela"}, {"VGO", "Vigo"},
	{"LCG", "A Coruña"}, {"LEI", "Almería"}, {"OVD", "Asturias"}, {"GRX", "Granada"},
	{"XRY", "Jerez"}, {"SDR", "Santander"}, {"TFN", "Tenerife Norte"}, {"SPC", "La Palma"},
	{"ORY", "París Orly"}, {"CDG", "París Charles de Gaulle"},
	{"FCO", "Roma Fiumicino"}, {"MXP", "Milán Malpensa"},
	{"AMS", "Ámsterdam"}, {"LGW", "Londres Gatwick"}, {"BRU", "Bruselas"}, {"BSL", "Basilea"},
	{"BER", "Berlín"}, {"BHX", "Birmingham"}, {"BUD", "Budapest"}, {"CMN", "Casablanca"},
	{"LIS", "Lisboa"}, {"OPO", "Oporto"}, {"FRA", "Fráncfort"}, {"FNC", "Funchal"}, {"MLA", "Malta"}, {"MUC", "Múnich"},
	{"DUB", "Dublín"}, {"GVA", "Ginebra"}, {"ZRH", "Zúrich"}, {"EDI", "Edimburgo"}, {"CPH", "Copenhague"},
	{"AGA", "Agadir"}, {"ALG", "Argel"}, {"ATH", "Atenas"}, {"BJL", "Banjul"}, {"BRI", "Bari"}, {"BLQ", "Bolonia"}, {"FEZ", "Fez"},
	{"BOD", "Burdeos"}, {"CAI", "El Cairo"}, {"CAG", "Cagliari"}, {"CTA", "Catania"}, {"HER", "Heraclión"},
	{"DSS", "Dakar"}, {"DBV", "Dubrovnik"}, {"DUS", "Düsseldorf"}, {"FAO", "Faro"},
	{"FLR", "Florencia"}, {"GOA", "Génova"}, {"HAM", "Hamburgo"}, {"HAJ", "Hanóver"},
	{"LYS", "Lyon"}, {"LJU", "Liubliana"}, {"MAN", "Mánchester"}, {"RAK", "Marrakech"}, {"MRS", "Marsella"},
	{"NTE", "Nantes"}, {"NAP", "Nápoles"}, {"NCE", "Niza"}, {"NUE", "Núremberg"},
	{"OLB", "Olbia"}, {"OSL", "Oslo"}, {"PMO", "Palermo"}, {"PRG", "Praga"},
	{"KEF", "Reikiavik"}, {"RMI", "Rímini"}, {"JTR", "Santorini"}, {"SPU", "Split"}, {"PDL", "Ponta Delgada"}, {"PNA", "Pamplona"}, {"REU", "Reus"},
	{"CDT", "Castellón"}, {"CFU", "Corfú"}, {"CIY", "Comiso"}, {"TIA", "Tirana"},
	{"EAS", "San Sebastián"}, {"ORN", "Orán"}, {"ZAG", "Zagreb"}, {"RJL", "Logroño"},
	{"VLL", "Valladolid"}, {"ZAZ", "Zaragoza"}, {"JMK", "Mikonos"},
	{"ODB", "Córdoba"}, {"IST", "Estambul"}, {"ESU", "Esauira"}, {"IVL", "Ivalo"}, {"RVN", "Rovaniemi"},
	{"MLN", "Melilla"}, {"LLA", "Luleå"}, {"SID", "Sal"}, {"TIV", "Tivat"}, {"TOS", "Tromsø"},
	{"BIA", "Bastia"}, {"LIN", "Milán Linate"}, {"QSR", "Salerno"}, {"PSA", "Pisa"}, {"LCY", "Londres City"}, {"LHR", "Londres Heathrow"},
	{"NDR", "Nador"}, {"GOT", "Gotemburgo"}, {"AMM", "Amán"}, {"BEY", "Beirut"}, {"BLL", "Billund"}, {"CZL", "Constantina"},
	{"ARN", "Estocolmo"}, {"CWL", "Cardiff"}, {"SXB", "Estrasburgo"}, {"STR", "Stuttgart"}, {"TNG", "Tánger"}, {"TUN", "Túnez"},
	{"TRN", "Turín"}, {"TLS", "Toulouse"}, {"VCE", "Venecia"}, {"VIE", "Viena"},
}

var aliases = map[string][]string{
	"ORY": {"Paris Orly"}, "CDG": {"Paris Charles de Gaulle"},
	"CAI": {"Cairo"}, "SXB": {"Strasbourg"},
	"ODB": {"Cordoba"}, "IST": {"Istanbul"}, "ESU": {"Essaouira"}, "LLA": {"Lulea"}, "TOS": {"Tromso"},
	"BIA": {"Bastia Corsica"}, "LIN": {"Milan", "Milan Linate", "Milano"}, "QSR": {"Naples Salerno", "Salerno"}, "PSA": {"Pisa"},
	"LCY": {"London", "London City"}, "LHR": {"London", "London Heathrow"}, "NDR": {"Nador"}, "GOT": {"Gothenburg", "Goteborg"},
	"AMM": {"Amman"}, "BEY": {"Beirut"}, "BLL": {"Billund"}, "CZL": {"Constantine"},
	"FCO": {"Rome", "Rome Fiumicino"}, "MXP": {"Milan", "Milan Malpensa"},
	"BSL": {"Basel"}, "LJU": {"Ljubljana"},
	"LGW": {"London", "London Gatwick"}, "BRU": {"Brussels"},
	"CFU": {"Corfu"}, "EAS": {"Donostia", "San Sebastian"}, "JMK": {"Mykonos"}, "ORN": {"Oran"}, "TIA": {"Tirane"},
	"LIS": {"Lisbon"}, "OPO": {"Porto"}, "FRA": {"Frankfurt"}, "MUC": {"Munich"},
	"LCG": {"La Coruna"}, "MAH": {"Minorca"}, "PMI": {"Mallorca"},
	"ALG": {"Algiers"}, "ATH": {"Athens"}, "BLQ": {"Bologna"}, "BOD": {"Bordeaux"},
	"HER": {"Crete", "Heraklion"}, "FLR": {"Florence"}, "GOA": {"Genoa"}, "HAJ": {"Hannover"},
	"MAN": {"Manchester"}, "RAK": {"Marrakesh"}, "MRS": {"Marseille"}, "NAP": {"Naples"},
	"NCE": {"Nice"}, "NUE": {"Nuremberg"}, "PRG": {"Prague"}, "KEF": {"Reykjavik"},
	"RMI": {"Rimini"}, "ARN": {"Stockholm"}, "TNG": {"Tangier"}, "TUN": {"Tunis"},
	"TRN": {"Turin"}, "VCE": {"Venice"}, "VIE": {"Vienna"},
}

// Search returns the airports whose code, city, or name match query.
func Search(query string) []Airport {
	q := normalize(query)
	matches := []Airport{}
	for _, airport := range catalogue {
		if q == "" || strings.EqualFold(airport.Code, q) || matchesName(q, airport.City, aliases[airport.Code]) {
			matches = append(matches, airport)
		}
	}
	return matches
}

func matchesName(query, city string, aliases []string) bool {
	if strings.Contains(normalize(city), query) {
		return true
	}
	for _, alias := range aliases {
		if strings.Contains(normalize(alias), query) {
			return true
		}
	}
	return false
}

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u").Replace(s)
}
