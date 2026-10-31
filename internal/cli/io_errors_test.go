package cli

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

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

func TestWatchFlightStatusReturnsStderrWriteError(t *testing.T) {
	writeErr := errors.New("stderr unavailable")
	cli := &CLI{stdout: new(bytes.Buffer), stderr: errorWriter{writeErr}}
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
	if !errors.Is(err, writeErr) {
		t.Fatalf("watchFlightStatus error = %v, want stderr write error", err)
	}
}

func TestFlightsReturnsNearbyWarningWriteError(t *testing.T) {
	writeErr := errors.New("stderr unavailable")
	reader := &sequencePageReader{err: errors.New("calendar read failed")}
	cli := New("test", new(bytes.Buffer), errorWriter{writeErr}, func(string) web.PageReader { return reader })
	err := cli.flights([]string{"BCN", "AGP", "--date", "+30", "--nearby"})
	if !errors.Is(err, writeErr) {
		t.Fatalf("flights error = %v, want nearby warning write error", err)
	}
}
