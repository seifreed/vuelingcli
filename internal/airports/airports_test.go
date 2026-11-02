package airports

import "testing"

func TestSearch(t *testing.T) {
	for query, code := range map[string]string{"barcelona": "BCN", "BCN": "BCN", "paris": "CDG", "París": "CDG", "London": "LGW", "Heathrow": "LHR", "Rome": "FCO", "Prague": "PRG", "BJL": "BJL"} {
		found := false
		for _, airport := range Search(query) {
			found = found || airport.Code == code
		}
		if !found {
			t.Errorf("Search(%q) did not include %s", query, code)
		}
	}
	if got := Search("no such airport"); len(got) != 0 {
		t.Fatalf("unknown query returned %v", got)
	}
	if got := Search("no such airport"); got == nil {
		t.Fatal("no matches should encode as an empty JSON array, not null")
	}
}

func TestCity(t *testing.T) {
	if city, ok := City("bio"); !ok || city != "Bilbao" {
		t.Fatalf("City(bio) = %q, %v; want Bilbao", city, ok)
	}
	if _, ok := City("ZZZ"); ok {
		t.Fatal("City found an unknown code")
	}
}
