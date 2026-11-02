package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

type testPageReader struct{}

func (testPageReader) ReadPage(context.Context, string, string) ([]byte, error) {
	return nil, fmt.Errorf("unexpected page read")
}

func newTestCLI() (*CLI, *bytes.Buffer, *bytes.Buffer) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	readerFactory := func(string) web.PageReader { return testPageReader{} }
	return New("test-version", stdout, stderr, readerFactory).WithWebPageSource(web.Source{}), stdout, stderr
}

func TestRunReturnsProcessStatus(t *testing.T) {
	cli, stdout, stderr := newTestCLI()
	if code := cli.Run(nil); code != 0 || !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("Run(nil) = %d, stdout %q; want help and exit 0", code, stdout)
	}
	stdout.Reset()
	if code := cli.Run([]string{"unknown"}); code != 2 || stderr.String() != "vueling: unknown command \"unknown\" (run 'vueling help')\n" {
		t.Fatalf("Run(unknown) = %d, stderr %q; want unknown-command error and exit 2", code, stderr)
	}
	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"version", "--bad"}); code != 2 || stderr.String() != "version: unknown option --bad\n" {
		t.Fatalf("Run(version --bad) = %d, stderr %q; want usage error and exit 2", code, stderr)
	}
}

func TestRunDispatchesEveryCommandHelp(t *testing.T) {
	for _, command := range []string{"version", "--version", "-v", "search", "airports", "calendar", "deals", "trips", "routes", "schedule", "alerts", "fees", "session", "flights", "status", "extras", "select", "link"} {
		t.Run(command, func(t *testing.T) {
			cli, stdout, _ := newTestCLI()
			if code := cli.Run([]string{command, "--help"}); code != 0 || stdout.Len() == 0 {
				t.Fatalf("Run(%q --help) = %d, output %q", command, code, stdout.String())
			}
		})
	}
}

func TestRunExecutesSearchAndAirportCommands(t *testing.T) {
	cli, stdout, _ := newTestCLI()
	if code := cli.Run([]string{"search", "BCN", "AGP", "--date", "+30"}); code != 0 || !strings.Contains(stdout.String(), "tickets.vueling.com") {
		t.Fatalf("Run(search) = %d, output %q", code, stdout.String())
	}
	stdout.Reset()
	if code := cli.Run([]string{"airports", "heathrow"}); code != 0 || !strings.Contains(stdout.String(), "LHR") {
		t.Fatalf("Run(airports) = %d, output %q", code, stdout.String())
	}
}

func TestRunHelpReportsOutputErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"search", "--help"}, {"airports", "--help"}} {
		cli := New("test", errorWriter{err: fmt.Errorf("write failed")}, nil, nil)
		if code := cli.Run(args); code != 2 {
			t.Errorf("Run(%v) = %d, want 2 on help output failure", args, code)
		}
	}
}

func TestCommandsRejectInvalidArguments(t *testing.T) {
	cli, _, _ := newTestCLI()
	flights, calendar, flightStatus := cli.flights, cli.calendar, cli.flightStatus
	airportSearch, showVersion, session := cli.airportSearch, cli.showVersion, cli.session
	flightLink := func(args []string) error { return cli.flightLink("link", args) }
	flightSearch := func(args []string) error { return cli.flightLink("search", args) }
	for _, tc := range []struct {
		name    string
		command func([]string) error
		args    []string
		want    string
	}{
		{"flights both formats", flights, []string{"BCN", "AGP", "--date", "+30", "--json", "--toon"}, "cannot be used together"},
		{"flights unknown long option", flights, []string{"BCN", "AGP", "--date", "+30", "--seats", "2"}, "unknown option --seats"},
		{"flights unknown short option", flights, []string{"BCN", "AGP", "--date", "+30", "-x"}, "unknown option -x"},
		{"flights missing value", flights, []string{"BCN", "AGP", "--date"}, "--date requires a value"},
		{"flights missing date", flights, []string{"BCN", "AGP"}, "usage:"},
		{"flights one airport", flights, []string{"BCN", "--date", "+30"}, "usage:"},
		{"flights zero limit", flights, []string{"BCN", "AGP", "--date", "+30", "--limit", "0"}, "invalid --limit"},
		{"flights invalid after", flights, []string{"BCN", "AGP", "--date", "+30", "--after", "25:00"}, "invalid --after"},
		{"flights invalid before", flights, []string{"BCN", "AGP", "--date", "+30", "--before", "noon"}, "invalid --before"},
		{"flights inverted window", flights, []string{"BCN", "AGP", "--date", "+30", "--after", "18:00", "--before", "08:00"}, "--after must not be later than --before"},
		{"flights invalid return-after", flights, []string{"BCN", "AGP", "--date", "+30", "--return", "+35", "--return-after", "7"}, "invalid --return-after"},
		{"flights invalid return-before", flights, []string{"BCN", "AGP", "--date", "+30", "--return", "+35", "--return-before", "x"}, "invalid --return-before"},
		{"flights inverted return window", flights, []string{"BCN", "AGP", "--date", "+30", "--return", "+35", "--return-after", "20:00", "--return-before", "10:00"}, "--return-after must not be later than --return-before"},
		{"flights return window without return", flights, []string{"BCN", "AGP", "--date", "+30", "--return-after", "10:00"}, "need --return"},
		{"flights no adults", flights, []string{"BCN", "AGP", "--date", "+30", "--adults", "0"}, "passengers must total"},
		{"flights discount out of range", flights, []string{"BCN", "PMI", "--date", "+30", "--discount", "6"}, "invalid --discount"},
		{"flights discount outside EUR", flights, []string{"BCN", "PMI", "--date", "+30", "--discount", "1", "--currency", "GBP"}, "--discount needs EUR"},
		{"flights unsupported currency", flights, []string{"BCN", "AGP", "--date", "+30", "--currency", "cad"}, "unsupported currency"},

		{"calendar unknown option", calendar, []string{"BCN", "AGP", "--date", "+30", "--cheap"}, "unknown option --cheap"},
		{"calendar missing date", calendar, []string{"BCN", "AGP"}, "usage:"},
		{"calendar currency without prices", calendar, []string{"BCN", "AGP", "--date", "+30", "--currency", "GBP"}, "--currency requires --prices"},
		{"calendar currency with browser", calendar, []string{"BCN", "AGP", "--date", "+30", "--prices", "--browser", "--currency=GBP"}, "cannot be used with --browser"},
		{"calendar unsupported currency", calendar, []string{"BCN", "AGP", "--date", "+30", "--prices", "--currency", "CAD"}, "unsupported currency"},
		{"calendar month without prices", calendar, []string{"BCN", "AGP", "--date", "+30", "--month"}, "--month requires --prices"},
		{"calendar unknown origin", calendar, []string{"Atlantis", "AGP", "--date", "+30"}, "origin:"},
		{"calendar unknown destination", calendar, []string{"BCN", "Atlantis", "--date", "+30"}, "destination:"},
		{"calendar invalid date", calendar, []string{"BCN", "AGP", "--date=soon"}, "invalid date"},
		{"calendar invalid return", calendar, []string{"BCN", "AGP", "--date", "+30", "--return=later"}, "invalid date"},
		{"calendar return before departure", calendar, []string{"BCN", "AGP", "--date", "+30", "--return", "+10"}, "on or after departure"},
		{"calendar return too far", calendar, []string{"BCN", "AGP", "--date", "+1", "--return", "+340"}, "within 330 days"},
		{"calendar same airport", calendar, []string{"BCN", "BCN", "--date", "+30"}, "different three-letter IATA codes"},

		{"status unknown option", flightStatus, []string{"VY1234", "--gate", "A1"}, "unknown option --gate"},
		{"status short watch", flightStatus, []string{"VY1234", "--watch", "4"}, "at least 5"},
		{"status non-numeric watch", flightStatus, []string{"VY1234", "--watch=often"}, "at least 5"},
		{"status overflowing watch", flightStatus, []string{"VY1234", "--watch", "153722868"}, "too long"},
		{"status watch with toon", flightStatus, []string{"VY1234", "--watch", "5", "--toon"}, "cannot be used with --toon"},
		{"status no flight", flightStatus, nil, "usage:"},
		{"status three positionals", flightStatus, []string{"BCN", "AGP", "MAD"}, "usage:"},
		{"status unknown origin", flightStatus, []string{"Atlantis", "AGP"}, "not in the airport catalogue"},
		{"status ambiguous destination", flightStatus, []string{"BCN", "Londres"}, "matches multiple airports"},
		{"status invalid date", flightStatus, []string{"VY1234", "--date", "soon"}, "invalid --date"},

		{"link unknown option", flightLink, []string{"BCN", "AGP", "--date", "+30", "--seat", "1A"}, "unknown option --seat"},
		{"link missing date", flightLink, []string{"BCN", "AGP"}, "usage: vueling link"},
		{"search missing date", flightSearch, []string{"BCN", "AGP"}, "usage: vueling search"},
		{"link invalid discount", flightLink, []string{"BCN", "AGP", "--date", "+30", "--discount", "x"}, "invalid --discount"},
		{"link discount out of range", flightLink, []string{"BCN", "AGP", "--date", "+30", "--discount", "9"}, "discount must be between 0 and 5"},

		{"airports unknown option", airportSearch, []string{"--all"}, "unknown option --all"},
		{"airports no match", airportSearch, []string{"Atlantis"}, "no Vueling airport matches"},
		{"airports two terms", airportSearch, []string{"Barcelona", "Madrid"}, "use one city"},

		{"session unknown action", session, []string{"renew"}, "usage: vueling session"},
		{"session two actions", session, []string{"refresh", "clear"}, "usage: vueling session"},
		{"session both formats", session, []string{"--json", "--toon"}, "cannot be used together"},
		{"session without API", session, nil, "could not be initialised"},

		{"version unknown option", showVersion, []string{"--short"}, "unknown option --short"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.command(tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%v error = %v, want it to contain %q", tc.args, err, tc.want)
			}
		})
	}
}

func TestFlightLinkPrintsURL(t *testing.T) {
	cli, output, _ := newTestCLI()
	err := cli.flightLink("link", strings.Fields("barcelona agp --date +30 --adults=2 --children 1 --flight vy2115 --json"))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]string
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON %q: %v", output, err)
	}
	u, err := url.Parse(result["url"])
	if err != nil {
		t.Fatal(err)
	}
	want := time.Now().AddDate(0, 0, 30).Format(time.DateOnly)
	for k, v := range map[string]string{"o": "BCN", "d": "AGP", "dd": want, "adt": "2", "chd": "1", "ofn": "VY2115"} {
		if got := u.Query().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestCalendarPrintsLink(t *testing.T) {
	cli, output, _ := newTestCLI()
	err := cli.calendar(strings.Fields("BCN AGP --date +30 --return +35"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := vueling.CalendarURL("BCN", "AGP", time.Now().AddDate(0, 0, 30))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(output.String()) != want {
		t.Fatalf("calendar printed %q, want %q", output, want)
	}
}

func TestCommandsOpenLinksThroughInjectedOpener(t *testing.T) {
	cli, _, _ := newTestCLI()
	var opened []string
	cli.WithLinkOpener(func(link string) error {
		opened = append(opened, link)
		return nil
	})

	if err := cli.flightLink("link", strings.Fields("BCN AGP --date +30 --open")); err != nil {
		t.Fatal(err)
	}
	if err := cli.calendar(strings.Fields("BCN AGP --date +30 --open")); err != nil {
		t.Fatal(err)
	}
	if len(opened) != 2 || !strings.Contains(opened[0], "vueling.com") || !strings.Contains(opened[1], "calendario-de-precios") {
		t.Fatalf("opened URLs = %v", opened)
	}
}

func TestAirportSearchOutputs(t *testing.T) {
	cli, output, _ := newTestCLI()
	err := cli.airportSearch([]string{"heathrow", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"LHR"`) {
		t.Fatalf("JSON airport search = %q, want LHR", output)
	}
	output.Reset()
	err = cli.airportSearch([]string{"heathrow"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output.String(), "LHR ") {
		t.Fatalf("text airport search = %q, want an LHR row", output)
	}
}

func TestShowVersion(t *testing.T) {
	cli, output, _ := newTestCLI()
	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, "vueling " + cli.version + "\n"},
		{[]string{"--json"}, `{"version":"` + cli.version + `"}` + "\n"},
		{[]string{"--toon", "--toon"}, "version: " + cli.version + "\n"},
	} {
		output.Reset()
		err := cli.showVersion(tc.args)
		if err != nil {
			t.Fatal(err)
		}
		if output.String() != tc.want {
			t.Errorf("showVersion(%v) = %q, want %q", tc.args, output, tc.want)
		}
	}
}

func TestShowVersionReportsFormatAndWriterErrors(t *testing.T) {
	cli := New("test", new(bytes.Buffer), nil, nil)
	if err := cli.showVersion([]string{"--json", "--toon"}); err == nil || !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("showVersion conflicting formats error = %v", err)
	}
	for _, args := range [][]string{{"--help"}, nil} {
		cli := New("test", errorWriter{err: fmt.Errorf("write failed")}, nil, nil)
		if err := cli.showVersion(args); err == nil {
			t.Errorf("showVersion(%v) ignored the writer error", args)
		}
	}
}

func TestResolveAirport(t *testing.T) {
	for _, tc := range []struct {
		value, want string
		ok          bool
	}{
		{" bcn ", "BCN", true},
		{"Barcelona", "BCN", true},
		{"Heathrow", "LHR", true},
		{"Londres", "", false},
		{"sal", "SID", true},
		{"Salerno", "QSR", true},
		{"zzz", "ZZZ", true},
		{"Atlantis", "", false},
	} {
		got, err := resolveAirport(tc.value)
		if (err == nil) != tc.ok || got != tc.want {
			t.Errorf("resolveAirport(%q) = %q, %v; want %q, ok=%v", tc.value, got, err, tc.want, tc.ok)
		}
	}
}

func TestStatusLine(t *testing.T) {
	got := statusLine(vueling.FlightStatus{Flight: "VY1234", Date: "2027-02-03", Status: "Aterrizado", Departure: "07:15", Arrival: "09:05", Origin: "BCN", Destination: "AGP"})
	if want := "VY1234 2027-02-03 Aterrizado 07:15–09:05 BCN→AGP"; got != want {
		t.Fatalf("statusLine = %q, want %q", got, want)
	}
}

func TestPrintCalendarDaysMarksCheapest(t *testing.T) {
	cli, output, _ := newTestCLI()
	days := []vueling.CalendarDay{{Date: "03/01/2027", Fare: "29 €"}, {Date: "04/01/2027", Fare: "40 €"}}
	if err := cli.printCalendarDays("IDA: Barcelona - Málaga", days, days[0]); err != nil {
		t.Fatal(err)
	}
	want := "IDA: Barcelona - Málaga\n03/01/2027: 29 € [cheapest]\n04/01/2027: 40 €\n"
	if output.String() != want {
		t.Fatalf("printCalendarDays = %q, want %q", output, want)
	}
}

type cannedPageReader string

func (r cannedPageReader) ReadPage(context.Context, string, string) ([]byte, error) {
	return []byte(r), nil
}

func TestCalendarPricesPrintsReturnFare(t *testing.T) {
	stdout := new(bytes.Buffer)
	page := cannedPageReader(`{"route":"IDA: BARCELONA","date":"20/11/2026","fare":"112 €","returnRoute":"VUELTA: MADRID","returnDate":"25/11/2026","returnFare":"90 €"}`)
	cli := New("test", stdout, new(bytes.Buffer), func(string) web.PageReader { return page }).WithWebPageSource(web.Source{})
	date := time.Now().AddDate(0, 0, 20)
	err := cli.calendar([]string{"BCN", "MAD", "--date", date.Format("2006-01-02"), "--return", date.AddDate(0, 0, 5).Format("2006-01-02"), "--prices"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "VUELTA: MADRID " + date.AddDate(0, 0, 5).Format("02/01/2006") + ": 90 €\n"; !strings.HasSuffix(stdout.String(), want) {
		t.Fatalf("calendar printed %q, want suffix %q", stdout, want)
	}
}
