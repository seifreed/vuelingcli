package search

import (
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	today := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	base := Options{Origin: "BCN", Destination: "LHR", Date: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Adults: 1}
	if err := base.Validate(today); err != nil {
		t.Fatalf("valid search rejected: %v", err)
	}
	tests := []struct {
		name string
		edit func(*Options)
	}{
		{"same airports", func(o *Options) { o.Destination = o.Origin }},
		{"past date", func(o *Options) { o.Date = today.AddDate(0, 0, -1) }},
		{"return before outbound", func(o *Options) { o.ReturnDate = o.Date.AddDate(0, 0, -1) }},
		{"infant without adult", func(o *Options) { o.Infants = 1; o.Adults = 0 }},
		{"invalid time", func(o *Options) { o.After = "24:00" }},
		{"conflicting formats", func(o *Options) { o.JSON = true; o.TOON = true }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := base
			tt.edit(&o)
			if err := o.Validate(today); err == nil {
				t.Fatal("invalid search accepted")
			}
		})
	}
}
