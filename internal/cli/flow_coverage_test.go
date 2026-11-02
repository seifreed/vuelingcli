package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/usecase"
	"github.com/seifreed/vuelingcli/internal/vueling"
)

func TestAlertsHandlesValidationAPIErrorsAndWriterErrors(t *testing.T) {
	cli, _, _ := newTestCLI()
	for _, args := range [][]string{{"--json", "--toon"}, {"--culture"}, {"plain"}} {
		if err := cli.alerts(args); err == nil {
			t.Errorf("alerts(%v) succeeded", args)
		}
	}
	if err := cli.alerts(nil); err == nil || !strings.Contains(err.Error(), "could not be initialised") {
		t.Fatalf("alerts without API error = %v", err)
	}
	apiErr := errors.New("alert service failed")
	cli.api = &stubAPI{alertFn: func(context.Context, string) (vueling.Alert, error) { return vueling.Alert{}, apiErr }}
	if err := cli.alerts(nil); !errors.Is(err, apiErr) {
		t.Fatalf("alerts API error = %v", err)
	}
	cli.api = &stubAPI{alertFn: func(context.Context, string) (vueling.Alert, error) {
		return vueling.Alert{Active: true, Text: "notice"}, nil
	}}
	cli.stdout = errorWriter{err: errors.New("stdout failed")}
	if err := cli.alerts(nil); err == nil {
		t.Fatal("alerts ignored a text output error")
	}
	if err := cli.alerts([]string{"--json"}); err == nil {
		t.Fatal("alerts ignored a structured output error")
	}
	if err := New("test", errorWriter{err: errors.New("stdout failed")}, nil, nil).alerts([]string{"--help"}); err == nil {
		t.Fatal("alerts ignored a help output error")
	}
}

func TestExtrasHandlesMissingAPIAndAPIError(t *testing.T) {
	cli, _, _ := newTestCLI()
	args := []string{"BCN", "BIO", "--date", "+30", "--flight", "VY1234"}
	if err := cli.extras(args); err == nil || !strings.Contains(err.Error(), "could not be initialised") {
		t.Fatalf("extras without API error = %v", err)
	}
	apiErr := errors.New("extras service failed")
	cli.api = &stubAPI{extrasFn: func(context.Context, usecase.FlightQuery, usecase.FlightPick, string) (vueling.Extras, error) {
		return vueling.Extras{}, apiErr
	}}
	if err := cli.extras(args); !errors.Is(err, apiErr) {
		t.Fatalf("extras API error = %v", err)
	}
}

func TestFlightLinkReportsURLAndWriterErrors(t *testing.T) {
	args := []string{"BCN", "BIO", "--date", "+30", "--currency", "CAD"}
	if err := New("test", nil, nil, nil).flightLink("link", args); err == nil {
		t.Fatal("flightLink accepted an unsupported currency")
	}
	cli := New("test", errorWriter{err: errors.New("stdout failed")}, nil, nil)
	if err := cli.flightLink("link", []string{"BCN", "BIO", "--date", "+30"}); err == nil {
		t.Fatal("flightLink ignored a text output error")
	}
	if err := cli.flightLink("link", []string{"BCN", "BIO", "--date", "+30", "--json"}); err == nil {
		t.Fatal("flightLink ignored a structured output error")
	}
	if err := New("test", nil, nil, nil).flightLink("link", []string{"BCN", "BIO", "--date"}); err == nil {
		t.Fatal("flightLink accepted a missing date value")
	}
}

func TestSelectHandlesAPIAndQuoteErrors(t *testing.T) {
	args := []string{"BCN", "BIO", "--date", "+30", "--flight", "VY1234"}
	cli, _, _ := newTestCLI()
	if err := cli.selectFlight(args); err == nil || !strings.Contains(err.Error(), "could not be initialised") {
		t.Fatalf("select without API error = %v", err)
	}
	apiErr := errors.New("quote failed")
	cli.api = &stubAPI{quoteTripFn: func(context.Context, usecase.FlightQuery, usecase.FlightPick, usecase.FlightPick) ([]vueling.Quote, error) {
		return nil, apiErr
	}}
	if err := cli.selectFlight(args); !errors.Is(err, apiErr) {
		t.Fatalf("select API error = %v", err)
	}
	cli.api = &stubAPI{quoteTripFn: func(context.Context, usecase.FlightQuery, usecase.FlightPick, usecase.FlightPick) ([]vueling.Quote, error) {
		return nil, nil
	}}
	if err := cli.selectFlight(args); err == nil {
		t.Fatal("select accepted an empty quote response")
	}
}

func TestSessionPropagatesStorageAndOperationErrors(t *testing.T) {
	readErr, refreshErr, clearErr := errors.New("read failed"), errors.New("refresh failed"), errors.New("clear failed")
	cli, _, _ := newTestCLI()
	cli.api = &stubAPI{storedFn: func() (vueling.Session, error) { return vueling.Session{}, readErr }}
	if err := cli.session(nil); !errors.Is(err, readErr) {
		t.Fatalf("session stored error = %v", err)
	}
	cli.api = &stubAPI{refreshFn: func(context.Context) (vueling.Session, error) { return vueling.Session{}, refreshErr }}
	if err := cli.session([]string{"refresh"}); !errors.Is(err, refreshErr) {
		t.Fatalf("session refresh error = %v", err)
	}
	cli.api = &stubAPI{clearSessionFn: func(context.Context) error { return clearErr }}
	if err := cli.session([]string{"clear"}); !errors.Is(err, clearErr) {
		t.Fatalf("session clear error = %v", err)
	}
	if err := New("test", errorWriter{err: errors.New("stdout failed")}, nil, nil).session([]string{"--help"}); err == nil {
		t.Fatal("session ignored a help output error")
	}
}

func TestTripsHandlesCalendarErrorsEmptyResultsAndOutputErrors(t *testing.T) {
	departure := time.Now().AddDate(0, 0, 30)
	departText := departure.Format(time.DateOnly)
	args := []string{"BCN", "BIO", "--from", departText, "--to", departText, "--nights", "1"}
	cli, _, _ := newTestCLI()
	if err := cli.trips(args); err == nil || !strings.Contains(err.Error(), "could not be initialised") {
		t.Fatalf("trips without API error = %v", err)
	}
	apiErr := errors.New("calendar failed")
	cli.api = &stubAPI{calendarFn: func(context.Context, string, string, string) ([]vueling.CalendarDay, error) { return nil, apiErr }}
	if err := cli.trips(args); !errors.Is(err, apiErr) {
		t.Fatalf("trips outbound API error = %v", err)
	}
	calls := 0
	cli.api = &stubAPI{calendarFn: func(context.Context, string, string, string) ([]vueling.CalendarDay, error) {
		calls++
		if calls == 2 {
			return nil, apiErr
		}
		return []vueling.CalendarDay{{Date: departure.Format("02/01/2006"), Fare: "10 €"}}, nil
	}}
	if err := cli.trips(args); !errors.Is(err, apiErr) {
		t.Fatalf("trips inbound API error = %v", err)
	}
	cli.api = &stubAPI{}
	if err := cli.trips(args); err == nil || !strings.Contains(err.Error(), "no BCN–BIO trips") {
		t.Fatalf("trips without fares error = %v", err)
	}
	cli.api = &stubAPI{calendarFn: func(_ context.Context, origin, _, _ string) ([]vueling.CalendarDay, error) {
		fare := "10 €"
		day := departure
		if origin == "BIO" {
			fare = "20 €"
			day = departure.AddDate(0, 0, 1)
		}
		return []vueling.CalendarDay{{Date: day.Format("02/01/2006"), Fare: fare}}, nil
	}}
	cli.stdout = errorWriter{err: errors.New("stdout failed")}
	if err := cli.trips(args); err == nil {
		t.Fatal("trips ignored a text output error")
	}
	if err := cli.trips(append(append([]string{}, args...), "--json")); err == nil {
		t.Fatal("trips ignored a structured output error")
	}
	if got := nightsLabel(1); got != "noche" {
		t.Fatalf("nightsLabel(1) = %q", got)
	}
	if err := New("test", errorWriter{err: errors.New("stdout failed")}, nil, nil).trips([]string{"--help"}); err == nil {
		t.Fatal("trips ignored a help output error")
	}
}
