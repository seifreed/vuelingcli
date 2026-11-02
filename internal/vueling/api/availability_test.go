package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFarePricesRejectsNegativeAmountsAndAllowsZero(t *testing.T) {
	tests := []struct {
		name, amount string
		wantErr      bool
	}{
		{name: "negative", amount: "-1", wantErr: true},
		{name: "zero", amount: "0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var response journeyAvailabilityResponse
			body := `{"data":{"amsAvy":{"faresAvailable":[{"value":{"fareAvailabilityKey":"a","fares":[{"productClass":"BA","passengerFares":[{"amsFareAmount":` + test.amount + `}]}]}}]}}}`
			if err := json.Unmarshal([]byte(body), &response); err != nil {
				t.Fatal(err)
			}

			prices, err := response.Data.AMSAvy.farePrices()
			if test.wantErr {
				if err == nil || !strings.Contains(err.Error(), "negative fare amount") {
					t.Fatalf("farePrices error = %v, want negative fare amount", err)
				}
				return
			}
			if err != nil || len(prices) != 1 || prices["a"].amount != 0 || prices["a"].bundle != "Basic" {
				t.Fatalf("farePrices = %v, %v; want a zero priced Basic fare", prices, err)
			}
		})
	}
}
