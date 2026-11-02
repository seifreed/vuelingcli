package web

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

type calendarPageReader struct {
	output []byte
	err    error
	url    string
	script string
}

func (reader *calendarPageReader) ReadPage(_ context.Context, pageURL, script string) ([]byte, error) {
	reader.url, reader.script = pageURL, script
	return reader.output, reader.err
}

func TestCalendarPricesFromPage(t *testing.T) {
	reader := &calendarPageReader{output: []byte(`{"route":"IDA: BCN - BIO","date":"03/11/2026","fare":"49,99 €"}`)}
	got, err := (Source{}).CalendarPricesFromPage(context.Background(), reader, "https://www.vueling.com/calendar?dateComplete=03%2F11%2F2026", "04/11/2026")
	if err != nil || got["fare"] != "49,99 €" {
		t.Fatalf("CalendarPricesFromPage = %v, %v", got, err)
	}
	u, err := url.Parse(reader.url)
	if err != nil || u.Query().Get("vuelingcliReturnDate") != "04/11/2026" || reader.script != renderedPriceScript {
		t.Fatalf("calendar request = %q, script match = %v, parse error = %v", reader.url, reader.script == renderedPriceScript, err)
	}
}

func TestCalendarMonthFromPage(t *testing.T) {
	reader := &calendarPageReader{output: []byte(`{"route":"IDA: BCN - BIO","date":"03/11/2026","fare":"49,99 €","days":[{"day":"3","fare":"49,99 €"}]}`)}
	got, err := (Source{}).CalendarMonthFromPage(context.Background(), reader, "https://www.vueling.com/calendar", "")
	if err != nil || len(got.Days) != 1 || got.Days[0].Date != "03/11/2026" {
		t.Fatalf("CalendarMonthFromPage = %+v, %v", got, err)
	}
	u, err := url.Parse(reader.url)
	if err != nil || u.Query().Get("vuelingcliMonth") != "1" {
		t.Fatalf("month request URL = %q, parse error = %v", reader.url, err)
	}
}

func TestCalendarPageReadAndDecodeErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		url    string
		reader *calendarPageReader
	}{
		{"invalid URL", "://bad", &calendarPageReader{}},
		{"read failure", "https://www.vueling.com/calendar", &calendarPageReader{err: errors.New("browser failed")}},
		{"decode failure", "https://www.vueling.com/calendar", &calendarPageReader{output: []byte("{")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := (Source{}).CalendarPricesFromPage(context.Background(), tc.reader, tc.url, ""); err == nil {
				t.Fatal("CalendarPricesFromPage succeeded after an invalid input or page failure")
			}
		})
	}
}

func TestMonthDays(t *testing.T) {
	days := monthDays([]renderedDay{{"14", "59,99 €"}, {"3", "49,99 €"}, {"9", "120 €"}, {"3", "10 €"}, {"31", "5 €"}, {"bad", "5 €"}, {"20", "49,99 €"}}, "14/11/2026")
	want := []vueling.CalendarDay{
		{Date: "03/11/2026", Fare: "49,99 €"},
		{Date: "09/11/2026", Fare: "120 €"},
		{Date: "14/11/2026", Fare: "59,99 €"},
		{Date: "20/11/2026", Fare: "49,99 €"},
	}
	if !reflect.DeepEqual(days, want) {
		t.Fatalf("monthDays = %v, want %v", days, want)
	}
	if got := monthDays([]renderedDay{{"1", "9 €"}}, ""); got != nil {
		t.Fatalf("monthDays without reference = %v", got)
	}
}

func TestDecodeCalendarRejectsNegativeFares(t *testing.T) {
	tests := []struct {
		name, output string
	}{
		{"selected fare", `{"route":"IDA: BCN - BIO","date":"03/11/2026","fare":"-49,99 €"}`},
		{"month day", `{"route":"IDA: BCN - BIO","date":"03/11/2026","fare":"49,99 €","days":[{"day":"3","fare":"-49,99 €"}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeCalendar([]byte(test.output)); err == nil || !strings.Contains(err.Error(), "unreadable calendar fare") {
				t.Fatalf("decodeCalendar error = %v, want unreadable negative fare error", err)
			}
		})
	}
}

func TestRenderedCalendarFarePatternSupportsGroupedAmounts(t *testing.T) {
	pattern := `[-−]?[0-9]+(?:[.,][0-9]{3})*(?:[.,][0-9]{1,2})?`
	if strings.Count(renderedPriceScript, pattern) != 2 {
		t.Fatal("rendered calendar script must capture full grouped fares for selected and monthly prices")
	}
}
