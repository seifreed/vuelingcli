package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

func TestAirportSearchPropagatesTextWriteError(t *testing.T) {
	cli := New("test", errorWriter{err: errors.New("stdout failed")}, nil, nil)
	if err := cli.airportSearch([]string{"Barcelona"}); err == nil {
		t.Fatal("airportSearch ignored a text output error")
	}
}

func TestFeesHandlesMissingAPIFailuresAndOutputErrors(t *testing.T) {
	cli, _, _ := newTestCLI()
	if err := cli.fees(nil); err == nil || !strings.Contains(err.Error(), "could not be initialised") {
		t.Fatalf("fees without API error = %v", err)
	}
	apiErr := errors.New("fee service failed")
	cli.api = &stubAPI{feesFn: func(context.Context) ([]vueling.Fee, error) { return nil, apiErr }}
	if err := cli.fees(nil); !errors.Is(err, apiErr) {
		t.Fatalf("fees API error = %v", err)
	}
	cli.api = &stubAPI{feesFn: func(context.Context) ([]vueling.Fee, error) { return []vueling.Fee{{Service: "Maletas"}}, nil }}
	if err := cli.fees([]string{"unknown"}); err == nil || !strings.Contains(err.Error(), "no Vueling fee matches") {
		t.Fatalf("fees missing match error = %v", err)
	}
	cli.stdout = errorWriter{err: errors.New("stdout failed")}
	if err := cli.fees(nil); err == nil {
		t.Fatal("fees ignored a text output error")
	}
	if err := cli.fees([]string{"--json"}); err == nil {
		t.Fatal("fees ignored a structured output error")
	}
}

func TestRoutesHandlesParseAPIEmptyAndOutputErrors(t *testing.T) {
	cli, _, _ := newTestCLI()
	for _, args := range [][]string{{"BCN", "--country"}, {"Atlantis"}} {
		if err := cli.routes(args); err == nil {
			t.Errorf("routes(%v) succeeded", args)
		}
	}
	if err := cli.routes([]string{"BCN"}); err == nil || !strings.Contains(err.Error(), "could not be initialised") {
		t.Fatalf("routes without API error = %v", err)
	}
	apiErr := errors.New("route service failed")
	cli.api = &stubAPI{routesFn: func(context.Context, string) ([]vueling.Route, error) { return nil, apiErr }}
	if err := cli.routes([]string{"BCN"}); !errors.Is(err, apiErr) {
		t.Fatalf("routes API error = %v", err)
	}
	cli.api = &stubAPI{routesFn: func(context.Context, string) ([]vueling.Route, error) { return nil, nil }}
	if err := cli.routes([]string{"BCN"}); err == nil || !strings.Contains(err.Error(), "no destination") {
		t.Fatalf("routes empty result error = %v", err)
	}
	cli.api = &stubAPI{routesFn: func(context.Context, string) ([]vueling.Route, error) {
		return []vueling.Route{{Code: "BIO", Name: "Bilbao", Country: "España", Direct: true}}, nil
	}}
	cli.stdout = errorWriter{err: errors.New("stdout failed")}
	if err := cli.routes([]string{"BCN"}); err == nil {
		t.Fatal("routes ignored a text output error")
	}
	if err := cli.routes([]string{"BCN", "--json"}); err == nil {
		t.Fatal("routes ignored a structured output error")
	}
}

func TestScheduleHandlesParseAPIAndOutputErrors(t *testing.T) {
	cli, _, _ := newTestCLI()
	if err := cli.schedule([]string{"BCN", "BIO", "--months"}); err == nil {
		t.Fatal("schedule accepted a missing months value")
	}
	if err := cli.schedule([]string{"BCN", "BIO"}); err == nil || !strings.Contains(err.Error(), "could not be initialised") {
		t.Fatalf("schedule without API error = %v", err)
	}
	apiErr := errors.New("schedule service failed")
	cli.api = &stubAPI{scheduleFn: func(context.Context, string, string, string, time.Time, int) ([]vueling.ScheduledDay, error) {
		return nil, apiErr
	}}
	if err := cli.schedule([]string{"BCN", "BIO"}); !errors.Is(err, apiErr) {
		t.Fatalf("schedule API error = %v", err)
	}
	cli.api = &stubAPI{scheduleFn: func(context.Context, string, string, string, time.Time, int) ([]vueling.ScheduledDay, error) {
		return []vueling.ScheduledDay{{Date: "2026-11-03", Fare: "20 €", Connection: true}}, nil
	}}
	cli.stdout = errorWriter{err: errors.New("stdout failed")}
	if err := cli.schedule([]string{"BCN", "BIO", "--json"}); err == nil {
		t.Fatal("schedule ignored a structured output error")
	}
	if err := cli.schedule([]string{"BCN", "BIO"}); err == nil {
		t.Fatal("schedule ignored a text output error")
	}
}

func TestPrintSchedulePropagatesEveryWriteError(t *testing.T) {
	schedule := vueling.Schedule{
		Origin: "BCN", Destination: "BIO", From: "2026-11-01", To: "2026-11-30",
		Dates:  []vueling.ScheduledDay{{Date: "2026-11-03", Weekday: "mar", Flight: "VY1234", Departure: "07:00", Fare: "20 €", Connection: true}},
		Months: []vueling.ScheduleMonth{{Month: "2026-11", Days: []int{3}, Weekdays: []string{"mar"}}},
	}
	for _, failAt := range []int{1, 2, 3, 4} {
		cli := New("test", &failOnWrite{failAt: failAt, err: errors.New("write failed")}, nil, nil)
		if err := cli.printSchedule(schedule, true); err == nil {
			t.Errorf("printSchedule ignored write failure %d", failAt)
		}
	}
	if got := dayLine(schedule.Dates[0]); !strings.Contains(got, "desde 20 €") || !strings.Contains(got, "conexión") {
		t.Fatalf("dayLine connection = %q", got)
	}
}
