package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/observability"
	"github.com/seifreed/vuelingcli/internal/vueling/api"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

type routeFares map[string]map[time.Time]float64

// newAPICLI serves each route's day fares, keyed "BCNBIO", from a fake fare-calendar API.
func newAPICLI(t *testing.T, fares routeFares, reader web.PageReader) (*CLI, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/Auth"):
			_ = json.NewEncoder(w).Encode(map[string]string{"accessToken": "jwt"})
			return
		case strings.HasSuffix(r.URL.Path, "/graphql"):
			var request struct {
				Variables struct {
					RequestAVY struct {
						Request struct {
							Criteria []map[string]string
						}
					}
				}
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.Variables.RequestAVY.Request.Criteria) == 0 {
				http.Error(w, "invalid availability request", http.StatusBadRequest)
				return
			}
			criteria := request.Variables.RequestAVY.Request.Criteria[0]
			answer := strings.NewReplacer(`"origin":"BCN"`, `"origin":"`+criteria["origin"]+`"`, `"destination":"BIO"`, `"destination":"`+criteria["destination"]+`"`, "BCN|BIO", criteria["origin"]+"|"+criteria["destination"]).Replace(twoJourneys)
			_, _ = w.Write([]byte(answer))
			return
		}
		route, ok := fares[r.URL.Query().Get("originCode")+r.URL.Query().Get("destinationCode")]
		if !ok {
			http.Error(w, "unknown route", http.StatusNotFound)
			return
		}
		rows := []map[string]any{}
		for day, price := range route {
			rows = append(rows, map[string]any{"arrivalDate": day.Format("2006-01-02T00:00:00"), "price": price, "currency": "EUR"})
		}
		body := map[string]any{"availability": map[string]any{"value": map[string]any{"data": rows}}}
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	cli := New("test", stdout, stderr, func(string) web.PageReader { return reader }).WithWebPageSource(web.Source{})
	return cli.WithAPI(&api.Client{HTTP: server.Client(), APIW: server.URL, AMS: server.URL}), stdout, stderr
}

func calendarTestDates() (departure, sameMonth, nextMonth, returnDate time.Time) {
	now := time.Now()
	departure = time.Date(now.Year(), now.Month()+2, 10, 0, 0, 0, 0, time.Local)
	return departure, departure.AddDate(0, 0, 2), departure.AddDate(0, 1, -7), departure.AddDate(0, 1, 0)
}

func TestCalendarPricesReadsAPI(t *testing.T) {
	departure, sameMonth, _, returnDate := calendarTestDates()
	cli, stdout, _ := newAPICLI(t, routeFares{
		"BCNBIO": {departure: 48, sameMonth: 30},
		"BIOBCN": {returnDate: 52.5},
	}, testPageReader{})
	cli.newReader = nil
	if err := cli.calendar([]string{"BCN", "BIO", "--date", departure.Format(time.DateOnly), "--prices"}); err != nil {
		t.Fatal(err)
	}
	if want := "IDA: BARCELONA " + departure.Format("02/01/2006") + ": 48 €\n"; stdout.String() != want {
		t.Fatalf("calendar printed %q, want %q", stdout, want)
	}
	stdout.Reset()
	if err := cli.calendar([]string{"BCN", "BIO", "--date", departure.Format(time.DateOnly), "--return", returnDate.Format(time.DateOnly), "--prices", "--json"}); err != nil {
		t.Fatal(err)
	}
	var price map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &price); err != nil {
		t.Fatal(err)
	}
	if price["fare"] != "48 €" || price["returnRoute"] != "VUELTA: BILBAO" || price["returnFare"] != "52,50 €" || price["returnDate"] != returnDate.Format("02/01/2006") || !strings.HasPrefix(price["url"], "https://www.vueling.com/") {
		t.Fatalf("calendar JSON = %v", price)
	}
}

func TestCalendarPricesReportsMissingAPIFare(t *testing.T) {
	departure, sameMonth, _, returnDate := calendarTestDates()
	cli, _, _ := newAPICLI(t, routeFares{"BCNBIO": {sameMonth: 30}, "BIOBCN": {}}, testPageReader{})
	err := cli.calendar([]string{"BCN", "BIO", "--date", departure.Format(time.DateOnly), "--prices"})
	if err == nil || err.Error() != "Vueling shows no fare for the requested date" {
		t.Fatalf("calendar error = %v, want no fare for the requested date", err)
	}
	err = cli.calendar([]string{"BCN", "BIO", "--date", sameMonth.Format(time.DateOnly), "--return", returnDate.Format(time.DateOnly), "--prices"})
	if err == nil || err.Error() != "Vueling shows no fare for the requested return date" {
		t.Fatalf("calendar error = %v, want no fare for the requested return date", err)
	}
	err = cli.calendar([]string{"BCN", "MAD", "--date", departure.Format(time.DateOnly), "--prices"})
	if err == nil || err.Error() != "Vueling sells no fares from BCN to MAD" {
		t.Fatalf("calendar error = %v, want an unknown-route message", err)
	}
}

func TestCalendarMonthReadsAPI(t *testing.T) {
	departure, sameMonth, nextMonth, returnDate := calendarTestDates()
	cli, stdout, _ := newAPICLI(t, routeFares{
		"BCNBIO": {departure: 48, sameMonth: 30, nextMonth: 10},
		"BIOBCN": {returnDate: 52},
	}, testPageReader{})
	err := cli.calendar([]string{"BCN", "BIO", "--date", departure.Format(time.DateOnly), "--return", returnDate.Format(time.DateOnly), "--prices", "--month"})
	if err != nil {
		t.Fatal(err)
	}
	want := "IDA: BARCELONA\n" + departure.Format("02/01/2006") + ": 48 €\n" + sameMonth.Format("02/01/2006") + ": 30 € [cheapest]\n" +
		"VUELTA: BILBAO\n" + returnDate.Format("02/01/2006") + ": 52 € [cheapest]\n"
	if stdout.String() != want {
		t.Fatalf("calendar printed %q, want %q", stdout, want)
	}
}

func TestCalendarBrowserFlagReadsPage(t *testing.T) {
	departure, _, _, _ := calendarTestDates()
	page := cannedPageReader(`{"route":"IDA: BARCELONA","date":"` + departure.Format("02/01/2006") + `","fare":"99 €","returnRoute":"","returnDate":"","returnFare":""}`)
	cli, stdout, _ := newAPICLI(t, routeFares{"BCNBIO": {departure: 48}}, page)
	if err := cli.calendar([]string{"BCN", "BIO", "--date", departure.Format(time.DateOnly), "--prices", "--browser"}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(stdout.String(), ": 99 €\n") {
		t.Fatalf("calendar --browser printed %q, want the page fare", stdout)
	}
	if err := cli.calendar([]string{"BCN", "BIO", "--date", departure.Format(time.DateOnly), "--browser"}); err == nil || err.Error() != "--browser requires --prices" {
		t.Fatalf("calendar --browser without --prices = %v", err)
	}
}

func TestCalendarCurrencyRequiresAPIClient(t *testing.T) {
	cli := New("test", new(bytes.Buffer), new(bytes.Buffer), nil).WithWebPageSource(web.Source{})
	departure, _, _, _ := calendarTestDates()
	if err := cli.calendar([]string{"BCN", "BIO", "--date", departure.Format(time.DateOnly), "--prices", "--currency", "EUR"}); err == nil || !strings.Contains(err.Error(), "--currency requires the Vueling API client") {
		t.Fatalf("calendar with browser fallback and explicit currency error = %v, want API requirement", err)
	}
}

func TestFlightsNearbyReadsAPI(t *testing.T) {
	departure, sameMonth, _, returnDate := calendarTestDates()
	reader := &sequencePageReader{err: errors.New("unexpected calendar page read")}
	cli, stdout, stderr := newAPICLI(t, routeFares{
		"BCNAGP": {departure: 48, sameMonth: 30},
		"AGPBCN": {returnDate: 52},
	}, reader)
	err := cli.flights([]string{"BCN", "AGP", "--date", departure.Format(time.DateOnly), "--nearby"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "cheapest nearby: " + sameMonth.Format("02/01/2006") + " from 30 €\n"; !strings.HasSuffix(stdout.String(), want) || stderr.Len() != 0 {
		t.Fatalf("flights printed %q (stderr %q), want suffix %q", stdout, stderr, want)
	}
	if reader.calls != 0 {
		t.Fatalf("flights read %d Chrome pages, want none", reader.calls)
	}
}

func TestNearbyFaresWarnsOnAPIFailure(t *testing.T) {
	departure, _, _, returnDate := calendarTestDates()
	cli, _, stderr := newAPICLI(t, routeFares{"BCNAGP": {departure: 48}}, testPageReader{})
	var logs bytes.Buffer
	cli.WithLogger(observability.New(&logs, observability.Config{Level: slog.LevelWarn, Format: "json"}))
	outbound, inbound := cli.nearbyFares(testPageReader{}, false, "EUR", "BCN", "AGP", departure, returnDate)
	if outbound != nil || inbound != nil || stderr.Len() != 0 || !strings.Contains(logs.String(), `"event_name":"flights.nearby_fares.unavailable"`) || !strings.Contains(logs.String(), `"direction":"outbound_and_return"`) {
		t.Fatalf("nearbyFares = %v, %v (stderr %q, logs %q); want a structured warning", outbound, inbound, stderr, logs.String())
	}
}

func TestCalendarPricesUsesRequestedCurrency(t *testing.T) {
	departure, _, _, _ := calendarTestDates()
	var currencies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currencies = append(currencies, r.URL.Query().Get("currencyCode"))
		row := map[string]any{"arrivalDate": departure.Format("2006-01-02T00:00:00"), "price": 20, "currency": "GBP"}
		_ = json.NewEncoder(w).Encode(map[string]any{"availability": map[string]any{"value": map[string]any{"data": []any{row}}}})
	}))
	defer server.Close()
	stdout := new(bytes.Buffer)
	cli := New("test", stdout, new(bytes.Buffer), func(string) web.PageReader { return testPageReader{} }).WithAPI(&api.Client{HTTP: server.Client(), APIW: server.URL})
	if err := cli.calendar([]string{"BCN", "BIO", "--date", departure.Format(time.DateOnly), "--prices", "--currency", "gbp"}); err != nil {
		t.Fatal(err)
	}
	if len(currencies) != 1 || currencies[0] != "GBP" || !strings.HasSuffix(stdout.String(), ": 20 GBP\n") {
		t.Fatalf("currencies = %v, stdout = %q", currencies, stdout)
	}
}
