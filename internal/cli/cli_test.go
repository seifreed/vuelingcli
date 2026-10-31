package cli

import (
	"testing"

	"github.com/seifreed/vuelingcli/internal/airports"
	"github.com/seifreed/vuelingcli/internal/vueling"
	toon "github.com/toon-format/toon-go"
)

func TestConnectionSummary(t *testing.T) {
	tests := []struct {
		flight vueling.Flight
		want   string
	}{
		{vueling.Flight{Flights: []string{"VY1000"}}, ""},
		{vueling.Flight{Stops: 1, Flights: []string{"VY1383", "VY1050"}, Via: []string{"BCN"}}, "1 escala vía BCN · VY1383+VY1050 | "},
		{vueling.Flight{Stops: 1, Flights: []string{"VY1001"}}, "1 escala | "},
		{vueling.Flight{Stops: 2, Flights: []string{"VY1", "VY2", "VY3"}, Via: []string{"BCN", "FCO"}}, "2 escalas vía BCN, FCO · VY1+VY2+VY3 | "},
	}
	for _, test := range tests {
		if got := connectionSummary(test.flight); got != test.want {
			t.Errorf("connectionSummary(%+v) = %q, want %q", test.flight, got, test.want)
		}
	}
}

func TestEncodeTOONUsesJSONFields(t *testing.T) {
	encoded, err := encodeTOON([]airports.Airport{{Code: "BCN", City: "Barcelona"}})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := toon.DecodeString(encoded)
	if err != nil {
		t.Fatalf("invalid TOON: %v", err)
	}
	rows, ok := decoded.([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("decoded TOON = %#v, want one airport row", decoded)
	}
	row, ok := rows[0].(map[string]any)
	if !ok || row["code"] != "BCN" || row["city"] != "Barcelona" {
		t.Fatalf("decoded TOON row = %#v, want JSON-tagged airport fields", rows[0])
	}
}
