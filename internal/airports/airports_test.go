package airports

import "testing"

func TestSearch(t *testing.T) {
	for query, code := range map[string]string{"barcelona": "BCN", "BCN": "BCN", "paris": "CDG", "París": "CDG", "London": "LGW", "Rome": "FCO", "Prague": "PRG", "BJL": "BJL"} {
		found := false
		for _, airport := range Search(query) {
			found = found || airport.Code == code
		}
		if !found {
			t.Errorf("Search(%q) did not include %s", query, code)
		}
	}
	for _, airport := range Search("London") {
		if airport.Code == "LHR" {
			t.Fatalf("returned Heathrow, which is not on the current Vueling destination list")
		}
	}
	if got := Search("no such airport"); len(got) != 0 {
		t.Fatalf("unknown query returned %v", got)
	}
	if got := Search("no such airport"); got == nil {
		t.Fatal("no matches should encode as an empty JSON array, not null")
	}
}
