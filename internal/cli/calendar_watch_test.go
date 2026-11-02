package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/observability"
	"github.com/seifreed/vuelingcli/internal/vueling/api"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

var watchClock = time.Date(2026, 11, 1, 9, 30, 0, 0, time.Local)

// testWatch counts the waits and stops the watch after maxWaits of them.
func testWatch(threshold float64, asJSON bool, maxWaits int, waits *int) calendarWatch {
	return calendarWatch{
		every: 30 * time.Minute, route: "BCN→BIO", link: "https://www.vueling.com/calendar", currency: "EUR",
		threshold: threshold, asJSON: asJSON, now: func() time.Time { return watchClock },
		wait: func(context.Context, time.Duration) bool {
			*waits++
			return *waits <= maxWaits
		},
	}
}

// fareSequence reads one canned outbound fare per call; an empty fare is a read failure.
func fareSequence(fares ...string) func(context.Context) (map[string]string, error) {
	calls := 0
	return func(context.Context) (map[string]string, error) {
		fare := fares[min(calls, len(fares)-1)]
		calls++
		if fare == "" {
			return nil, errors.New("Vueling did not answer")
		}
		return map[string]string{"date": "10/11/2026", "fare": fare, "returnDate": "", "returnFare": ""}, nil
	}
}

func TestCalendarWatchPrintsChangesUntilBelow(t *testing.T) {
	cli, stdout, stderr := newTestCLI()
	var logs bytes.Buffer
	cli.WithLogger(observability.New(&logs, observability.Config{Environment: "test", Level: slog.LevelWarn, Format: "json"}))
	waits := 0
	err := cli.watchCalendar(context.Background(), testWatch(40, false, 10, &waits), fareSequence("48 €", "48 €", "", "45,50 €", "40 €", "30 €"))
	if err != nil {
		t.Fatal(err)
	}
	want := "09:30 BCN→BIO 10/11/2026: 48 €\n" +
		"09:30 BCN→BIO 10/11/2026: 48 € → 45,50 €\n" +
		"09:30 BCN→BIO 10/11/2026: 45,50 € → 40 €  BELOW 40 EUR\n"
	if stdout.String() != want {
		t.Fatalf("watch printed %q, want %q", stdout, want)
	}
	if waits != 4 || stderr.Len() != 0 || !strings.Contains(logs.String(), `"event_name":"watch.retry"`) || !strings.Contains(logs.String(), `"retry_in_ms":1800000`) || strings.Contains(logs.String(), "Vueling did not answer") {
		t.Fatalf("waits = %d, stderr = %q, logs = %q", waits, stderr, logs.String())
	}
}

func TestCalendarWatchWithoutThresholdRunsUntilStopped(t *testing.T) {
	cli, stdout, _ := newTestCLI()
	waits := 0
	if err := cli.watchCalendar(context.Background(), testWatch(0, false, 2, &waits), fareSequence("48 €", "12 €", "12 €", "99 €")); err != nil {
		t.Fatal(err)
	}
	if want := "09:30 BCN→BIO 10/11/2026: 48 €\n09:30 BCN→BIO 10/11/2026: 48 € → 12 €\n"; stdout.String() != want || waits != 3 {
		t.Fatalf("watch printed %q after %d waits, want %q", stdout, waits, want)
	}
}

func TestCalendarWatchFirstReadFailureIsAnError(t *testing.T) {
	cli, stdout, _ := newTestCLI()
	waits := 0
	err := cli.watchCalendar(context.Background(), testWatch(40, false, 10, &waits), fareSequence(""))
	if err == nil || err.Error() != "Vueling did not answer" || stdout.Len() != 0 || waits != 0 {
		t.Fatalf("watch error = %v, stdout %q, waits %d", err, stdout, waits)
	}
	err = cli.watchCalendar(context.Background(), testWatch(40, false, 10, &waits), fareSequence("gratis"))
	if err == nil || !strings.Contains(err.Error(), "unreadable fare") {
		t.Fatalf("watch error = %v, want an unreadable fare", err)
	}
}

func TestCalendarWatchStopsWhenCancelled(t *testing.T) {
	cli, stdout, _ := newTestCLI()
	var logs bytes.Buffer
	cli.WithLogger(observability.New(&logs, observability.Config{Environment: "test", Level: slog.LevelInfo, Format: "json"}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	waits := 0
	if err := cli.watchCalendar(ctx, testWatch(40, false, 10, &waits), fareSequence("")); err != nil || stdout.Len() != 0 {
		t.Fatalf("cancelled watch = %v, stdout %q", err, stdout)
	}
	if !strings.Contains(logs.String(), `"event_name":"watch.stopped"`) || !strings.Contains(logs.String(), `"outcome":"cancelled"`) {
		t.Fatalf("cancelled watch was not logged: %s", logs.String())
	}
	if sleepOrDone(ctx, time.Hour) || !sleepOrDone(context.Background(), time.Nanosecond) {
		t.Fatal("sleepOrDone ignored the context or the timer")
	}
}

func TestWatchRetryIncludesHTTPStatusWithoutResponseText(t *testing.T) {
	var logs bytes.Buffer
	cli := New("test", new(bytes.Buffer), new(bytes.Buffer), nil).WithLogger(observability.New(&logs, observability.Config{Level: slog.LevelWarn, Format: "json"}))
	err := cli.writeWatchWarning("status", time.Minute, &api.StatusError{Status: http.StatusServiceUnavailable})
	if err != nil || !strings.Contains(logs.String(), `"status_code":503`) || strings.Contains(logs.String(), "HTTP 503") {
		t.Fatalf("watch retry log = %v: %s", err, logs.String())
	}
}

func TestCalendarWatchIgnoresLoggerWriteErrorsAndRetries(t *testing.T) {
	writeErr := errors.New("stderr unavailable")
	var stdout bytes.Buffer
	cli := &CLI{stdout: &stdout, stderr: errorWriter{writeErr}}
	cli.WithLogger(observability.New(errorWriter{writeErr}, observability.Config{Environment: "test", Level: slog.LevelWarn, Format: "json"}))
	waits := 0
	if err := cli.watchCalendar(context.Background(), testWatch(40, false, 1, &waits), fareSequence("48 €", "")); err != nil || waits != 2 {
		t.Fatalf("watch error = %v after %d waits; logger failure stopped polling", err, waits)
	}
}

func TestWatchPollReturnsRetryReporterFailure(t *testing.T) {
	readErr, reporterErr := errors.New("read failed"), errors.New("retry reporter failed")
	calls := 0
	err := watchPoll(context.Background(), time.Second,
		func(context.Context) (int, error) {
			calls++
			if calls == 1 {
				return 1, nil
			}
			return 0, readErr
		},
		func(int, bool) (bool, error) { return false, nil },
		func(error) error { return reporterErr },
		func(context.Context, time.Duration) bool { return true },
	)
	if !errors.Is(err, reporterErr) || calls != 2 {
		t.Fatalf("watchPoll = %v after %d reads, want retry reporter failure", err, calls)
	}
}

// changingFareServer answers the fare-calendar API with the next outbound price on every outbound
// request and a fixed 52,50 return price.
func changingFareServer(t *testing.T, departure, returnDate time.Time, outbound ...float64) *CLI {
	t.Helper()
	var mu sync.Mutex
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		day, price := returnDate, 52.5
		if r.URL.Query().Get("originCode") == "BCN" {
			mu.Lock()
			day, price = departure, outbound[min(calls, len(outbound)-1)]
			calls++
			mu.Unlock()
		}
		row := map[string]any{"arrivalDate": day.Format("2006-01-02T00:00:00"), "price": price, "currency": "EUR"}
		_ = json.NewEncoder(w).Encode(map[string]any{"availability": map[string]any{"value": map[string]any{"data": []any{row}}}})
	}))
	t.Cleanup(server.Close)
	cli := New("test", new(bytes.Buffer), new(bytes.Buffer), func(string) web.PageReader { return testPageReader{} })
	return cli.WithAPI(&api.Client{HTTP: server.Client(), APIW: server.URL})
}

func TestCalendarWatchReadsReturnTotalsFromAPIAsJSON(t *testing.T) {
	departure, _, _, returnDate := calendarTestDates()
	cli := changingFareServer(t, departure, returnDate, 48, 48, 30)
	stdout := new(bytes.Buffer)
	cli.stdout = stdout
	request := calendarRequest{origin: "BCN", destination: "BIO", date: departure, returnDate: returnDate.Format("02/01/2006"), currency: "EUR"}
	read := func(ctx context.Context) (map[string]string, error) {
		calendar, err := cli.readCalendar(ctx, false, testPageReader{}, "", request)
		return calendar.Price, err
	}
	waits := 0
	if err := cli.watchCalendar(context.Background(), testWatch(90, true, 10, &waits), read); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 || waits != 2 {
		t.Fatalf("watch printed %q after %d waits, want two JSON lines", stdout, waits)
	}
	var first, last watchedFare
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &last); err != nil {
		t.Fatal(err)
	}
	if first.Total != 100.5 || first.Below || first.Previous != 0 || first.ReturnFare != "52,50 €" || first.Route != "BCN→BIO" {
		t.Fatalf("first reading = %+v", first)
	}
	if last.Total != 82.5 || !last.Below || last.Previous != 100.5 || last.Threshold != 90 || last.Time != watchClock.Format(time.RFC3339) {
		t.Fatalf("last reading = %+v", last)
	}
}

func TestCalendarWatchCommandStopsWhenFirstReadIsBelow(t *testing.T) {
	departure, _, _, returnDate := calendarTestDates()
	cli := changingFareServer(t, departure, returnDate, 48)
	stdout := new(bytes.Buffer)
	cli.stdout = stdout
	args := []string{"Barcelona", "BIO", "--date", departure.Format(time.DateOnly), "--return", returnDate.Format(time.DateOnly), "--prices", "--watch", "60", "--below", "100,5"}
	if code := cli.Run(append([]string{"calendar"}, args...)); code != 0 {
		t.Fatalf("Run = %d", code)
	}
	want := " BCN→BIO " + departure.Format("02/01/2006") + " + " + returnDate.Format("02/01/2006") + ": 100,50 € (48 € + 52,50 €)  BELOW 100.5 EUR\n"
	if !strings.HasSuffix(stdout.String(), want) {
		t.Fatalf("calendar --watch printed %q, want suffix %q", stdout, want)
	}
}

func TestCalendarWatchOptionErrors(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--prices", "--below", "40"}, "--below requires --watch"},
		{[]string{"--watch", "60"}, "--watch requires --prices"},
		{[]string{"--prices", "--browser", "--watch", "60"}, "cannot be used with --browser"},
		{[]string{"--prices", "--month", "--watch", "60"}, "cannot be used with --month"},
		{[]string{"--prices", "--watch", "60", "--toon"}, "cannot be used with --toon"},
		{[]string{"--prices", "--watch", "29"}, "at least 30"},
		{[]string{"--prices", "--watch=hourly"}, "at least 30"},
		{[]string{"--prices", "--watch", "153722868"}, "too long"},
		{[]string{"--prices", "--watch", "60", "--below=NaN"}, "positive price"},
	} {
		_, err := parseCalendarOptions(append([]string{"BCN", "BIO", "--date", "+30"}, tc.args...), time.Now())
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("parseCalendarOptions(%v) error = %v, want %q", tc.args, err, tc.want)
		}
	}
	options, err := parseCalendarOptions([]string{"BCN", "BIO", "--date", "+30", "--prices", "--watch=30", "--below", "39,99", "--json"}, time.Now())
	if err != nil || options.watch != 30 || options.below != 39.99 {
		t.Fatalf("options = %+v, %v", options, err)
	}
}
