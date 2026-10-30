package airports

import "testing"

func TestSearch(t *testing.T) {
	for _, query := range []string{"barcelona", "BCN", "París"} {
		if got := Search(query); len(got) == 0 {
			t.Errorf("Search(%q) returned no matches", query)
		}
	}
	if got := Search("no such airport"); len(got) != 0 {
		t.Fatalf("unknown query returned %v", got)
	}
}
