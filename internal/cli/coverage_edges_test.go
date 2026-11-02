package cli

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/observability"
	"github.com/seifreed/vuelingcli/internal/usecase"
	"github.com/seifreed/vuelingcli/internal/vueling"
	"github.com/seifreed/vuelingcli/internal/vueling/ports"
)

type failAtWriter struct {
	calls int
	fail  int
	err   error
}

func (writer *failAtWriter) Write(data []byte) (int, error) {
	writer.calls++
	if writer.calls == writer.fail {
		return 0, writer.err
	}
	return len(data), nil
}

func TestReadFlightsReportsSourceAndAPIErrors(t *testing.T) {
	options, err := parseFlightOptions([]string{"BCN", "BIO", "--date", "+30", "--browser"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (&CLI{}).readFlights(nil, "", options); err == nil || err.Error() != "browser page source is not configured" {
		t.Fatalf("readFlights without page source error = %v", err)
	}
	apiErr := errors.New("availability failed")
	cli := &CLI{api: &stubAPI{flightsFn: func(context.Context, usecase.FlightQuery) ([]vueling.Flight, error) {
		return nil, apiErr
	}}}
	options.browser = false
	if _, err := cli.readFlights(nil, "", options); !errors.Is(err, apiErr) {
		t.Fatalf("readFlights API error = %v", err)
	}
	cli.api = &stubAPI{flightsFn: func(context.Context, usecase.FlightQuery) ([]vueling.Flight, error) {
		return []vueling.Flight{{Direction: "outbound"}}, nil
	}}
	options.returnDate = options.date.AddDate(0, 0, 1)
	if _, err := cli.readFlights(nil, "", options); err == nil || !strings.Contains(err.Error(), "no return flights") {
		t.Fatalf("readFlights without return error = %v", err)
	}
}

func TestFlightsPropagatesAvailabilityFilterAndOutputErrors(t *testing.T) {
	args := []string{"BCN", "BIO", "--date", "+30"}
	if err := New("test", errorWriter{err: errors.New("stdout failed")}, new(bytes.Buffer), nil).flights([]string{"--help"}); err == nil {
		t.Fatal("flights help ignored output error")
	}
	if err := New("test", new(bytes.Buffer), new(bytes.Buffer), nil).flights([]string{"BCN", "BIO"}); err == nil {
		t.Fatal("flights accepted a missing date")
	}
	apiErr := errors.New("availability failed")
	cli := New("test", new(bytes.Buffer), new(bytes.Buffer), nil).WithAPI(&stubAPI{
		flightsFn: func(context.Context, usecase.FlightQuery) ([]vueling.Flight, error) { return nil, apiErr },
	})
	if err := cli.flights(args); !errors.Is(err, apiErr) {
		t.Fatalf("flights API error = %v", err)
	}

	cli.api = &stubAPI{flightsFn: func(context.Context, usecase.FlightQuery) ([]vueling.Flight, error) {
		return []vueling.Flight{{Direction: "outbound", Flight: "VY1234", Departure: "bad"}}, nil
	}}
	if err := cli.flights(append(append([]string{}, args...), "--after", "08:00")); err == nil {
		t.Fatalf("flights invalid offer error = %v", err)
	}

	flight := vueling.Flight{Direction: "outbound", Date: time.Now().AddDate(0, 0, 30).Format(time.DateOnly), Flight: "VY1234", Departure: "10:00", Arrival: "11:00", Fare: "20.00", Currency: "EUR", Itinerary: "10:00 BCN → 11:00 BIO VY1234 Directo"}
	for _, fail := range []int{1, 2} {
		writer := &failAtWriter{fail: fail, err: errors.New("stdout failed")}
		cli = New("test", writer, new(bytes.Buffer), nil).WithAPI(&stubAPI{flightsFn: func(context.Context, usecase.FlightQuery) ([]vueling.Flight, error) {
			return []vueling.Flight{flight}, nil
		}})
		if err := cli.flights(args); err == nil {
			t.Errorf("flights ignored stdout failure on write %d", fail)
		}
	}
	cli = New("test", errorWriter{err: errors.New("stdout failed")}, new(bytes.Buffer), nil).WithAPI(&stubAPI{flightsFn: func(context.Context, usecase.FlightQuery) ([]vueling.Flight, error) {
		return []vueling.Flight{flight}, nil
	}})
	if err := cli.flights(append(append([]string{}, args...), "--json")); err == nil {
		t.Fatal("flights ignored structured output error")
	}
}

func TestFlightsNearbyReturnOutputErrors(t *testing.T) {
	date := time.Now().AddDate(0, 2, 0)
	returnDate := date.AddDate(0, 0, 5)
	flight := []vueling.Flight{
		{Direction: "outbound", Date: date.Format(time.DateOnly), Flight: "VY1234", Departure: "10:00", Arrival: "11:00", Fare: "20", Currency: "EUR"},
		{Direction: "return", Date: returnDate.Format(time.DateOnly), Flight: "VY5678", Departure: "12:00", Arrival: "13:00", Fare: "25", Currency: "EUR"},
	}
	for _, fail := range []int{3, 4, 5} {
		writer := &failAtWriter{fail: fail, err: errors.New("stdout failed")}
		cli := New("test", writer, new(bytes.Buffer), nil).WithAPI(&stubAPI{
			flightsFn: func(context.Context, usecase.FlightQuery) ([]vueling.Flight, error) { return flight, nil },
			calendarFn: func(_ context.Context, origin, destination, _ string) ([]vueling.CalendarDay, error) {
				day := date
				if origin == "BIO" && destination == "BCN" {
					day = returnDate
				}
				return []vueling.CalendarDay{{Date: day.Format("02/01/2006"), Fare: "30 €"}}, nil
			},
		})
		args := []string{"BCN", "BIO", "--date", date.Format(time.DateOnly), "--return", returnDate.Format(time.DateOnly), "--nearby"}
		if err := cli.flights(args); err == nil {
			t.Errorf("flights ignored nearby output error on write %d", fail)
		}
	}
}

func TestNearbyFaresWarningsAreStructuredAndLoggingFailuresDoNotAbort(t *testing.T) {
	departure := time.Now().AddDate(0, 0, 30)
	stderr := new(bytes.Buffer)
	var logs bytes.Buffer
	cli := New("test", new(bytes.Buffer), stderr, nil).WithAPI(&stubAPI{calendarFn: func(context.Context, string, string, string) ([]vueling.CalendarDay, error) {
		return nil, nil
	}}).WithLogger(observability.New(&logs, observability.Config{Level: slog.LevelWarn, Format: "json"}))
	out, back := cli.nearbyFares(nil, false, "EUR", "BCN", "BIO", departure, time.Time{})
	if out != nil || back != nil || !strings.Contains(logs.String(), `"event_name":"flights.nearby_fares.unavailable"`) || !strings.Contains(logs.String(), `"direction":"outbound"`) {
		t.Fatalf("nearbyFares empty calendar = %v, %v; logs %q", out, back, logs.String())
	}

	logs.Reset()
	cli.api = &stubAPI{calendarFn: func(_ context.Context, origin, destination, _ string) ([]vueling.CalendarDay, error) {
		if origin == "BIO" && destination == "BCN" {
			return nil, nil
		}
		return []vueling.CalendarDay{{Date: departure.Format("02/01/2006"), Fare: "25 €"}}, nil
	}}
	out, back = cli.nearbyFares(nil, false, "EUR", "BCN", "BIO", departure, departure.AddDate(0, 0, 4))
	if out == nil || back != nil || !strings.Contains(logs.String(), `"direction":"return"`) {
		t.Fatalf("nearbyFares without return prices = %v, %v; logs %q", out, back, logs.String())
	}

	writeErr := errors.New("stderr failed")
	cli.stderr = errorWriter{err: writeErr}
	cli.WithLogger(observability.New(errorWriter{err: writeErr}, observability.Config{Level: slog.LevelWarn, Format: "json"}))
	if out, back := cli.nearbyFares(nil, false, "EUR", "BCN", "BIO", time.Time{}, time.Time{}); out != nil || back != nil {
		t.Fatalf("invalid calendar request returned fares: %v, %v", out, back)
	}
	cli.api = &stubAPI{calendarFn: func(context.Context, string, string, string) ([]vueling.CalendarDay, error) { return nil, nil }}
	if out, back := cli.nearbyFares(nil, false, "EUR", "BCN", "BIO", departure, time.Time{}); out != nil || back != nil {
		t.Fatalf("missing outbound calendar returned fares: %v, %v", out, back)
	}
	cli.api = &stubAPI{calendarFn: func(_ context.Context, origin, destination, _ string) ([]vueling.CalendarDay, error) {
		if origin == "BCN" && destination == "BIO" {
			return []vueling.CalendarDay{{Date: departure.Format("02/01/2006"), Fare: "25 €"}}, nil
		}
		return nil, nil
	}}
	if out, back := cli.nearbyFares(nil, false, "EUR", "BCN", "BIO", departure, departure.AddDate(0, 0, 4)); out == nil || back != nil {
		t.Fatalf("missing return calendar = %v, %v", out, back)
	}
}

func TestFlightLinkReportsOpenerAndConflictingFormatErrors(t *testing.T) {
	openErr := errors.New("browser unavailable")
	cli := New("test", new(bytes.Buffer), new(bytes.Buffer), nil).WithLinkOpener(func(string) error { return openErr })
	args := []string{"BCN", "BIO", "--date", "+30", "--open"}
	if err := cli.flightLink("link", args); !errors.Is(err, openErr) {
		t.Fatalf("flightLink opener error = %v", err)
	}
	if _, err := parseFlightLinkOptions("link", []string{"BCN", "BIO", "--date", "+30", "--json", "--toon"}); err == nil {
		t.Fatal("parseFlightLinkOptions accepted conflicting output formats")
	}
}

func TestEncodeTOONReportsInvalidJSON(t *testing.T) {
	if _, err := encodeJSONTOON([]byte("{")); err == nil {
		t.Fatal("encodeJSONTOON accepted malformed JSON")
	}
}

func TestSelectFlightReportsWriterErrorsAtEachOutput(t *testing.T) {
	date := time.Now().AddDate(0, 0, 30)
	returnDate := date.AddDate(0, 0, 5)
	quote := func(flight string, day time.Time) vueling.Quote {
		return vueling.Quote{Flight: flight, Date: day.Format(time.DateOnly), Departure: "10:00", Arrival: "11:00", Origin: "BCN", Destination: "BIO", Fare: 20, Currency: "EUR"}
	}
	args := []string{"BCN", "BIO", "--date", date.Format(time.DateOnly), "--flight", "VY1234", "--return", returnDate.Format(time.DateOnly), "--return-flight", "VY5678"}
	for _, fail := range []int{1, 2, 3, 4} {
		writer := &failAtWriter{fail: fail, err: errors.New("stdout failed")}
		cli := New("test", writer, new(bytes.Buffer), nil).WithAPI(&stubAPI{quoteTripFn: func(context.Context, usecase.FlightQuery, usecase.FlightPick, usecase.FlightPick) ([]vueling.Quote, error) {
			return []vueling.Quote{quote("VY1234", date), quote("VY5678", returnDate)}, nil
		}})
		if err := cli.selectFlight(args); err == nil {
			t.Errorf("selectFlight ignored stdout failure on write %d", fail)
		}
	}
	cli := New("test", errorWriter{err: errors.New("stdout failed")}, new(bytes.Buffer), nil).WithAPI(&stubAPI{quoteTripFn: func(context.Context, usecase.FlightQuery, usecase.FlightPick, usecase.FlightPick) ([]vueling.Quote, error) {
		return []vueling.Quote{quote("VY1234", date)}, nil
	}})
	if err := cli.selectFlight([]string{"BCN", "BIO", "--date", date.Format(time.DateOnly), "--flight", "VY1234", "--toon"}); err == nil {
		t.Fatal("selectFlight ignored structured output error")
	}
}

func TestFlightStatusAndWatchErrorPaths(t *testing.T) {
	date := time.Now().AddDate(0, 0, 1).Format(time.DateOnly)
	status := vueling.FlightStatus{Flight: "VY1234", Date: date, Status: "Aterrizado", Departure: "10:00", Arrival: "11:00", Origin: "BCN", Destination: "BIO"}
	api := &stubAPI{statusFn: func(context.Context, string, string, string, time.Time) ([]vueling.FlightStatus, error) {
		return []vueling.FlightStatus{status}, nil
	}}
	for _, format := range []string{"", "--json"} {
		cli := New("test", errorWriter{err: errors.New("stdout failed")}, new(bytes.Buffer), nil).WithAPI(api)
		args := []string{"VY1234", "--date", date}
		if format != "" {
			args = append(args, format)
		}
		if err := cli.flightStatus(args); err == nil {
			t.Errorf("flightStatus(%q) ignored output error", format)
		}
	}

	apiErr := errors.New("status unavailable")
	cli := New("test", new(bytes.Buffer), new(bytes.Buffer), nil).WithAPI(&stubAPI{statusFn: func(context.Context, string, string, string, time.Time) ([]vueling.FlightStatus, error) {
		return nil, apiErr
	}})
	if err := cli.flightStatus([]string{"VY1234", "--date", date}); !errors.Is(err, apiErr) {
		t.Fatalf("flightStatus API error = %v", err)
	}

	cli = New("test", new(bytes.Buffer), new(bytes.Buffer), nil)
	options := flightStatusOptions{date: time.Now(), browser: true, flight: "VY1234"}
	if _, err := cli.statusSource(options); err == nil {
		t.Fatal("statusSource accepted missing browser reader")
	}

	stderr := new(bytes.Buffer)
	cli = New("test", new(bytes.Buffer), stderr, nil).WithLogger(observability.New(stderr, observability.Config{Environment: "test", Level: slog.LevelWarn, Format: "json"}))
	calls := 0
	err := cli.watchFlightStatus(0, false, func(context.Context) ([]vueling.FlightStatus, error) {
		calls++
		if calls == 2 {
			return nil, apiErr
		}
		current := status
		if calls == 1 {
			current.Status = "En Hora"
		}
		return []vueling.FlightStatus{current}, nil
	})
	if err != nil || calls != 3 || !strings.Contains(stderr.String(), `"event_name":"watch.retry"`) || !strings.Contains(stderr.String(), `"retry_in_ms":0`) {
		t.Fatalf("watchFlightStatus retry = %v after %d calls, logs %q", err, calls, stderr)
	}

	cli.stdout = errorWriter{err: errors.New("stdout failed")}
	if err := cli.watchFlightStatus(0, true, func(context.Context) ([]vueling.FlightStatus, error) { return []vueling.FlightStatus{status}, nil }); err == nil {
		t.Fatal("watchFlightStatus ignored JSON output error")
	}
	cli.stdout = errorWriter{err: errors.New("stdout failed")}
	if err := cli.watchFlightStatus(0, false, func(context.Context) ([]vueling.FlightStatus, error) { return []vueling.FlightStatus{status}, nil }); err == nil {
		t.Fatal("watchFlightStatus ignored text output error")
	}
	if err := cli.watchFlightStatus(0, false, func(context.Context) ([]vueling.FlightStatus, error) { return nil, apiErr }); !errors.Is(err, apiErr) {
		t.Fatalf("watchFlightStatus initial error = %v", err)
	}
	if err := cli.flightStatus([]string{"--help"}); err == nil {
		t.Fatal("flightStatus help ignored output error")
	}
	if err := cli.flightStatus([]string{"VY1234", "--date"}); err == nil {
		t.Fatal("flightStatus accepted missing date")
	}

	cli = New("test", new(bytes.Buffer), new(bytes.Buffer), func(string) ports.PageReader { return testPageReader{} })
	read, err := cli.statusSource(flightStatusOptions{browser: true, date: time.Now(), flight: "VY1234"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := read(context.Background()); err == nil || err.Error() != "browser page source is not configured" {
		t.Fatalf("browser status page source error = %v", err)
	}
}

func TestWatchFlightStatusStopsOnSignals(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows cannot send SIGTERM to the current process")
	}
	cli := New("test", new(bytes.Buffer), new(bytes.Buffer), nil)
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	err = cli.watchFlightStatus(time.Hour, false, func(ctx context.Context) ([]vueling.FlightStatus, error) {
		if err := process.Signal(syscall.SIGTERM); err != nil {
			t.Error(err)
		}
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatalf("watchFlightStatus did not stop cleanly during a read: %v", err)
	}

	timer := time.AfterFunc(10*time.Millisecond, func() {
		if err := process.Signal(syscall.SIGTERM); err != nil {
			t.Error(err)
		}
	})
	t.Cleanup(func() { timer.Stop() })
	cli = New("test", new(bytes.Buffer), new(bytes.Buffer), nil)
	err = cli.watchFlightStatus(time.Hour, false, func(context.Context) ([]vueling.FlightStatus, error) {
		return []vueling.FlightStatus{{Flight: "VY1234", Status: "En Hora"}}, nil
	})
	if err != nil {
		t.Fatalf("watchFlightStatus did not stop cleanly while waiting: %v", err)
	}
}

func TestRemainingCLIOptionValidationBranches(t *testing.T) {
	if _, err := parseFlightStatusOptions([]string{"VY1234", "--json", "--toon"}); err == nil {
		t.Fatal("status accepted conflicting formats")
	}
	if _, err := parseFlightStatusOptions([]string{"VY1234", "--date"}); err == nil {
		t.Fatal("status accepted a missing date")
	}
	if _, err := parseTripsOptions([]string{"BCN", "BIO", "--nights", "1", "--limit", "bad"}, time.Now()); err == nil {
		t.Fatal("trips accepted an invalid limit")
	}
	if _, err := parseTripsOptions([]string{"BCN", "BIO", "--nights", "1", "--max", "bad"}, time.Now()); err == nil {
		t.Fatal("trips accepted an invalid maximum price")
	}
	options := flightOptions{flightInput: flightInput{date: time.Now(), returnDate: time.Now().AddDate(0, 0, -1)}}
	if err := checkReturnPick(options, true, false); err == nil || !strings.Contains(err.Error(), "before --date") {
		t.Fatalf("checkReturnPick past return error = %v", err)
	}
	if err := tripResultError(errors.New("search failed"), nil, "BCN", "BIO"); err == nil || err.Error() != "search failed" {
		t.Fatalf("tripResultError domain error = %v", err)
	}
}
