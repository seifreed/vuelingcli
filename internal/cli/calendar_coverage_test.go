package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

type failOnWrite struct {
	writes int
	failAt int
	err    error
}

func (w *failOnWrite) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == w.failAt {
		return 0, w.err
	}
	return len(p), nil
}

func TestCalendarCommandReportsOpenAndReaderFailures(t *testing.T) {
	cli, _, _ := newTestCLI()
	openErr := errors.New("browser unavailable")
	cli.WithLinkOpener(func(string) error { return openErr })
	if err := cli.calendar([]string{"BCN", "AGP", "--date", "+30", "--open"}); !errors.Is(err, openErr) {
		t.Fatalf("calendar open error = %v", err)
	}
	cli.newReader = nil
	if err := cli.calendar([]string{"BCN", "AGP", "--date", "+30", "--prices"}); err == nil || !strings.Contains(err.Error(), "page reader is not configured") {
		t.Fatalf("calendar reader error = %v", err)
	}
	if err := cli.calendar([]string{"BCN", "AGP", "--date", "+30", "--prices", "--watch", "30"}); err == nil || !strings.Contains(err.Error(), "page reader is not configured") {
		t.Fatalf("calendar watch reader error = %v", err)
	}
	cli, stdout, _ := newTestCLI()
	if err := cli.calendar([]string{"BCN", "AGP", "--date", "+30", "--json"}); err != nil || !strings.Contains(stdout.String(), `"url"`) {
		t.Fatalf("calendar structured link = %q, %v", stdout, err)
	}
}

func TestCalendarPricesAndMonthOutputErrors(t *testing.T) {
	date := time.Date(2027, time.February, 3, 0, 0, 0, 0, time.UTC)
	options := calendarOptions{date: date, returnDate: "08/02/2027"}
	cli := New("test", errorWriter{err: errors.New("stdout failed")}, nil, nil)
	if err := cli.calendarPrices("url", map[string]string{"route": "IDA", "fare": "10 €", "returnRoute": "VUELTA", "returnFare": "20 €"}, options); err == nil {
		t.Fatal("calendarPrices ignored the stdout error")
	}
	if err := cli.calendarPrices("url", map[string]string{"route": "IDA", "fare": "10 €"}, calendarOptions{date: date, format: "json"}); err == nil {
		t.Fatal("structured calendarPrices ignored the stdout error")
	}

	month := vueling.CalendarMonth{Price: map[string]string{"route": "IDA", "returnRoute": "VUELTA"}, Days: []vueling.CalendarDay{{Date: "03/02/2027", Fare: "10 €"}, {Date: "04/02/2027", Fare: "12 €"}}}
	if err := cli.calendarMonth("url", month, "", ""); err == nil {
		t.Fatal("calendarMonth ignored a print failure")
	}
	if err := New("test", &failOnWrite{failAt: 2, err: errors.New("fare write failed")}, nil, nil).calendarMonth("url", month, "", ""); err == nil {
		t.Fatal("calendarMonth ignored a day-row write failure")
	}
	returnMonth := month
	returnMonth.ReturnDays = []vueling.CalendarDay{{Date: "10/02/2027", Fare: "20 €"}}
	if err := New("test", &failOnWrite{failAt: 4, err: errors.New("return route write failed")}, nil, nil).calendarMonth("url", returnMonth, "10/02/2027", ""); err == nil {
		t.Fatal("calendarMonth ignored a return-route write failure")
	}
	if err := New("test", errorWriter{err: errors.New("stdout failed")}, nil, nil).calendarMonth("url", month, "", "json"); err == nil {
		t.Fatal("structured calendarMonth ignored the stdout error")
	}
}

func TestCalendarMonthRejectsMissingFaresAndPrintsReturnDays(t *testing.T) {
	cli, stdout, _ := newTestCLI()
	if err := cli.calendarMonth("url", vueling.CalendarMonth{}, "", ""); err == nil || !strings.Contains(err.Error(), "readable calendar fares") {
		t.Fatalf("calendarMonth without days error = %v", err)
	}
	month := vueling.CalendarMonth{Price: map[string]string{"returnRoute": "VUELTA: BIO"}, Days: []vueling.CalendarDay{{Date: "03/02/2027", Fare: "10 €"}}}
	if err := cli.calendarMonth("url", month, "04/02/2027", ""); err == nil || !strings.Contains(err.Error(), "readable return calendar fares") {
		t.Fatalf("calendarMonth without return days error = %v", err)
	}
	month.ReturnDays = []vueling.CalendarDay{{Date: "04/02/2027", Fare: "20 €"}}
	if err := cli.calendarMonth("url", month, "04/02/2027", ""); err != nil || !strings.Contains(stdout.String(), "VUELTA") {
		t.Fatalf("calendarMonth with return days = %v, output %q", err, stdout)
	}
}

func TestCalendarSourceHelpersHandleMissingValues(t *testing.T) {
	cli, _, _ := newTestCLI()
	cli.webSource = nil
	if _, err := cli.readCalendar(context.Background(), true, nil, "url", calendarRequest{}); err == nil {
		t.Fatal("readCalendar succeeded without a page source")
	}
	if _, err := requestedFare(nil, "03/02/2027", false, "fare missing"); err == nil || err.Error() != "fare missing" {
		t.Fatalf("requestedFare missing error = %v", err)
	}
	if got, err := requestedFare(nil, "03/02/2027", true, "fare missing"); err != nil || got != "" {
		t.Fatalf("requestedFare missing month fare = %q, %v", got, err)
	}
	if got := routeLabel("IDA", "XXX"); got != "IDA: XXX" {
		t.Fatalf("routeLabel unknown airport = %q", got)
	}
}

func TestCalendarOptionsRejectConflictingOutputFormats(t *testing.T) {
	if _, err := parseCalendarOptions([]string{"BCN", "BIO", "--date", "+30", "--json", "--toon"}, time.Now()); err == nil || !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("parseCalendarOptions conflict error = %v", err)
	}
}

func TestCalendarWatchPrintErrors(t *testing.T) {
	cli := New("test", errorWriter{err: errors.New("stdout unavailable")}, nil, nil)
	waits := 0
	watch := testWatch(0, false, 0, &waits)
	err := cli.watchCalendar(context.Background(), watch, fareSequence("10 €"))
	if err == nil || !strings.Contains(err.Error(), "stdout unavailable") {
		t.Fatalf("watchCalendar write error = %v", err)
	}
}
