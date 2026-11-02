package cli

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/observability"
	"github.com/seifreed/vuelingcli/internal/vueling"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

type errorWriter struct{ err error }

func (writer errorWriter) Write([]byte) (int, error) { return 0, writer.err }

type sequencePageReader struct {
	calls int
	err   error
}

func (reader *sequencePageReader) ReadPage(context.Context, string, string) ([]byte, error) {
	reader.calls++
	if reader.calls == 1 {
		return []byte(`[{"direction":"outbound","label":"Seleccionar vuelo por 49 €","details":"07:15 BCN VY1234 Vueling 09:05 AGP"}]`), nil
	}
	return nil, reader.err
}

func TestRunReturnsFailureWhenStderrCannotBeWritten(t *testing.T) {
	writeErr := errors.New("stderr unavailable")
	for _, args := range [][]string{{"unknown"}, {"version", "--bad"}} {
		cli := New("test", new(bytes.Buffer), errorWriter{writeErr}, func(string) web.PageReader { return testPageReader{} })
		if code := cli.Run(args); code != 1 {
			t.Errorf("Run(%v) = %d, want 1 when stderr write fails", args, code)
		}
	}
}

func TestWatchFlightStatusContinuesWhenLoggerWriteFails(t *testing.T) {
	writeErr := errors.New("stderr unavailable")
	cli := &CLI{stdout: new(bytes.Buffer), stderr: errorWriter{writeErr}}
	cli.WithLogger(observability.New(errorWriter{writeErr}, observability.Config{Level: slog.LevelWarn, Format: "json"}))
	calls := 0
	err := cli.watchFlightStatus(time.Millisecond, false, func(context.Context) ([]vueling.FlightStatus, error) {
		calls++
		if calls == 2 {
			return nil, errors.New("page read failed")
		}
		status := "En Hora"
		if calls > 2 {
			status = "Aterrizado"
		}
		return []vueling.FlightStatus{{Flight: "VY1234", Date: "2026-10-31", Status: status}}, nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("watchFlightStatus = %v after %d reads, want completion after retry", err, calls)
	}
}

func TestFlightsLogsNearbyWarningAndKeepsPrimaryResult(t *testing.T) {
	writeErr := errors.New("stderr unavailable")
	reader := &sequencePageReader{err: errors.New("calendar read failed")}
	var logs bytes.Buffer
	cli := New("test", new(bytes.Buffer), errorWriter{writeErr}, func(string) web.PageReader { return reader }).
		WithWebPageSource(web.Source{}).
		WithLogger(observability.New(&logs, observability.Config{Level: slog.LevelWarn, Format: "json"}))
	if err := cli.flights([]string{"BCN", "AGP", "--date", "+30", "--nearby"}); err != nil {
		t.Fatalf("flights failed after an optional nearby warning: %v", err)
	}
	if !strings.Contains(logs.String(), `"event_name":"flights.nearby_fares.unavailable"`) || strings.Contains(logs.String(), "calendar read failed") {
		t.Fatalf("nearby warning missing or exposed its cause: %s", logs.String())
	}
}
