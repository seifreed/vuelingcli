package vueling

import (
	"strings"
	"testing"
	"time"
)

func TestParseStatusCard(t *testing.T) {
	date := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	got := parseStatusCard([]string{"En Hora", "VY1158", "13:00", "BCNBarcelona", "MaltaMLA", "15:10", "VER DETALLES"}, date)
	want := FlightStatus{Flight: "VY1158", Date: "2026-10-31", Status: "En Hora", Origin: "BCN", Destination: "MLA", Departure: "13:00", Arrival: "15:10", Details: "En Hora VY1158 13:00 BCNBarcelona MaltaMLA 15:10 VER DETALLES"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	got = parseStatusCard([]string{"En Hora", "VY392D", "14:40", "BCNBarcelona", "Palma (Mallorca)PMI", "15:35", "VER DETALLES"}, date)
	if got.Flight != "VY392D" || got.Origin != "BCN" || got.Destination != "PMI" || got.Arrival != "15:35" {
		t.Fatalf("lettered flight card parsed as %+v", got)
	}
}

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
