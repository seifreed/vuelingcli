package vueling

import (
	"strings"
	"testing"
)

func TestChangedStatusesAndAllFinal(t *testing.T) {
	onTime := FlightStatus{Flight: "VY1158", Date: "2026-10-31", Status: "En Hora", Departure: "13:00", Arrival: "15:10", Details: "a"}
	other := FlightStatus{Flight: "VY1160", Date: "2026-10-31", Status: "En Hora", Departure: "18:00", Arrival: "20:10"}
	withStatus := func(s FlightStatus, status string) FlightStatus {
		s.Status = status
		return s
	}
	delayed, detailsOnly := onTime, onTime
	delayed.Departure = "13:40"
	detailsOnly.Details = "b"
	tests := []struct {
		name           string
		previous, next []FlightStatus
		changed        []string
		final          bool
	}{
		{"first read", nil, []FlightStatus{onTime, other}, []string{"VY1158", "VY1160"}, false},
		{"unchanged", []FlightStatus{onTime, other}, []FlightStatus{onTime, other}, nil, false},
		{"details only", []FlightStatus{onTime}, []FlightStatus{detailsOnly}, nil, false},
		{"new departure", []FlightStatus{onTime, other}, []FlightStatus{delayed, other}, []string{"VY1158"}, false},
		{"one landed", []FlightStatus{onTime, other}, []FlightStatus{withStatus(onTime, "Aterrizado"), other}, []string{"VY1158"}, false},
		{"all final", []FlightStatus{onTime, other}, []FlightStatus{withStatus(onTime, "ATERRIZADO"), withStatus(other, "Cancelled")}, []string{"VY1158", "VY1160"}, true},
		{"english landed", nil, []FlightStatus{withStatus(onTime, "landed")}, []string{"VY1158"}, true},
		{"empty", []FlightStatus{onTime}, nil, nil, false},
		{"not operated and diverted landing", nil, []FlightStatus{withStatus(onTime, "No operado"), withStatus(other, "Aterrizado en VLC")}, []string{"VY1158", "VY1160"}, true},
		{"diverted in flight", nil, []FlightStatus{withStatus(onTime, "Desviado a VLC")}, []string{"VY1158"}, false},
	}
	for _, tt := range tests {
		var got []string
		for _, s := range ChangedStatuses(tt.previous, tt.next) {
			got = append(got, s.Flight)
		}
		if strings.Join(got, ",") != strings.Join(tt.changed, ",") {
			t.Errorf("%s: changed = %v, want %v", tt.name, got, tt.changed)
		}
		if final := AllFinal(tt.next); final != tt.final {
			t.Errorf("%s: AllFinal = %v, want %v", tt.name, final, tt.final)
		}
	}
}

func TestIsFlightNumber(t *testing.T) {
	for input, want := range map[string]bool{"VY1008": true, "VY392D": true, "VY1": false, "IB1008": false, "VY1008/../x": false} {
		if got := IsFlightNumber(input); got != want {
			t.Errorf("IsFlightNumber(%q) = %v, want %v", input, got, want)
		}
	}
}
