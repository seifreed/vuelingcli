package main

import (
	"testing"

	"github.com/seifreed/vuelingcli/internal/airports"
	toon "github.com/toon-format/toon-go"
)

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
