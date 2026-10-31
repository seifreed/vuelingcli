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
)

type testPageReader struct{}

func (testPageReader) ReadPage(context.Context, string, string) ([]byte, error) {
	return nil, fmt.Errorf("unexpected page read")
}

func newTestCLI() (*CLI, *bytes.Buffer, *bytes.Buffer) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	readerFactory := func(string) vueling.PageReader { return testPageReader{} }
	return New("test-version", stdout, stderr, readerFactory), stdout, stderr
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

func TestCommandsRejectInvalidArguments(t *testing.T) {
	cli, _, _ := newTestCLI()
	flights, calendar, flightStatus := cli.flights, cli.calendar, cli.flightStatus
	flightLink, airportSearch, showVersion := cli.flightLink, cli.airportSearch, cli.showVersion
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
		{"flights unknown origin", flights, []string{"Atlantis", "AGP", "--date", "+30"}, "origin:"},
		{"flights ambiguous destination", flights, []string{"BCN", "Londres", "--date", "+30"}, "destination:"},
		{"flights invalid date", flights, []string{"BCN", "AGP", "--date", "soon"}, "invalid date"},
		{"flights invalid return", flights, []string{"BCN", "AGP", "--date", "+30", "--return", "later"}, "invalid date"},
		{"flights invalid adults", flights, []string{"BCN", "AGP", "--date", "+30", "--adults", "two"}, "invalid --adults"},
		{"flights invalid children", flights, []string{"BCN", "AGP", "--date", "+30", "--children=x"}, "invalid --children"},
		{"flights invalid infants", flights, []string{"BCN", "AGP", "--date", "+30", "--infants=x"}, "invalid --infants"},
		{"flights zero limit", flights, []string{"BCN", "AGP", "--date", "+30", "--limit", "0"}, "invalid --limit"},
		{"flights invalid after", flights, []string{"BCN", "AGP", "--date", "+30", "--after", "25:00"}, "invalid --after"},
		{"flights invalid before", flights, []string{"BCN", "AGP", "--date", "+30", "--before", "noon"}, "invalid --before"},
		{"flights inverted window", flights, []string{"BCN", "AGP", "--date", "+30", "--after", "18:00", "--before", "08:00"}, "--after must not be later than --before"},
		{"flights invalid return-after", flights, []string{"BCN", "AGP", "--date", "+30", "--return", "+35", "--return-after", "7"}, "invalid --return-after"},
		{"flights invalid return-before", flights, []string{"BCN", "AGP", "--date", "+30", "--return", "+35", "--return-before", "x"}, "invalid --return-before"},
		{"flights inverted return window", flights, []string{"BCN", "AGP", "--date", "+30", "--return", "+35", "--return-after", "20:00", "--return-before", "10:00"}, "--return-after must not be later than --return-before"},
		{"flights return window without return", flights, []string{"BCN", "AGP", "--date", "+30", "--return-after", "10:00"}, "need --return"},
		{"flights no adults", flights, []string{"BCN", "AGP", "--date", "+30", "--adults", "0"}, "passengers must total"},
		{"flights unsupported currency", flights, []string{"BCN", "AGP", "--date", "+30", "--currency", "cad"}, "unsupported currency"},

		{"calendar both formats", calendar, []string{"BCN", "AGP", "--date", "+30", "--json", "--toon"}, "cannot be used together"},
		{"calendar unknown option", calendar, []string{"BCN", "AGP", "--date", "+30", "--cheap"}, "unknown option --cheap"},
		{"calendar missing date value", calendar, []string{"BCN", "AGP", "--date"}, "--date requires a value"},
		{"calendar missing return value", calendar, []string{"BCN", "AGP", "--date", "+30", "--return"}, "--return requires a value"},
		{"calendar missing date", calendar, []string{"BCN", "AGP"}, "usage:"},
		{"calendar month without prices", calendar, []string{"BCN", "AGP", "--date", "+30", "--month"}, "--month requires --prices"},
		{"calendar unknown origin", calendar, []string{"Atlantis", "AGP", "--date", "+30"}, "origin:"},
		{"calendar unknown destination", calendar, []string{"BCN", "Atlantis", "--date", "+30"}, "destination:"},
		{"calendar invalid date", calendar, []string{"BCN", "AGP", "--date=soon"}, "invalid date"},
		{"calendar invalid return", calendar, []string{"BCN", "AGP", "--date", "+30", "--return=later"}, "invalid date"},
		{"calendar return before departure", calendar, []string{"BCN", "AGP", "--date", "+30", "--return", "+10"}, "on or after departure"},
		{"calendar return too far", calendar, []string{"BCN", "AGP", "--date", "+1", "--return", "+340"}, "within 330 days"},
		{"calendar same airport", calendar, []string{"BCN", "BCN", "--date", "+30"}, "different three-letter IATA codes"},

		{"status both formats", flightStatus, []string{"VY1234", "--json", "--toon"}, "cannot be used together"},
		{"status missing value", flightStatus, []string{"VY1234", "--date"}, "--date requires a value"},
		{"status unknown option", flightStatus, []string{"VY1234", "--gate", "A1"}, "unknown option --gate"},
		{"status short watch", flightStatus, []string{"VY1234", "--watch", "4"}, "at least 5"},
		{"status non-numeric watch", flightStatus, []string{"VY1234", "--watch=often"}, "at least 5"},
		{"status watch with toon", flightStatus, []string{"VY1234", "--watch", "5", "--toon"}, "cannot be used with --toon"},
		{"status no flight", flightStatus, nil, "usage:"},
		{"status three positionals", flightStatus, []string{"BCN", "AGP", "MAD"}, "usage:"},
		{"status unknown origin", flightStatus, []string{"Atlantis", "AGP"}, "not in the airport catalogue"},
		{"status ambiguous destination", flightStatus, []string{"BCN", "Londres"}, "matches multiple airports"},
		{"status invalid date", flightStatus, []string{"VY1234", "--date", "soon"}, "invalid --date"},

		{"link both formats", flightLink, []string{"BCN", "AGP", "--date", "+30", "--json", "--toon"}, "cannot be used together"},
		{"link unknown option", flightLink, []string{"BCN", "AGP", "--date", "+30", "--seat", "1A"}, "unknown option --seat"},
		{"link missing value", flightLink, []string{"BCN", "AGP", "--date"}, "--date requires a value"},
		{"link missing date", flightLink, []string{"BCN", "AGP"}, "usage:"},
		{"link unknown origin", flightLink, []string{"Atlantis", "AGP", "--date", "+30"}, "origin:"},
		{"link unknown destination", flightLink, []string{"BCN", "Atlantis", "--date", "+30"}, "destination:"},
		{"link invalid date", flightLink, []string{"BCN", "AGP", "--date", "soon"}, "invalid date"},
		{"link invalid return", flightLink, []string{"BCN", "AGP", "--date", "+30", "--return", "later"}, "invalid date"},
		{"link invalid adults", flightLink, []string{"BCN", "AGP", "--date", "+30", "--adults", "x"}, "invalid --adults"},
		{"link invalid children", flightLink, []string{"BCN", "AGP", "--date", "+30", "--children", "x"}, "invalid --children"},
		{"link invalid infants", flightLink, []string{"BCN", "AGP", "--date", "+30", "--infants", "x"}, "invalid --infants"},
		{"link invalid discount", flightLink, []string{"BCN", "AGP", "--date", "+30", "--discount", "x"}, "invalid --discount"},
		{"link discount out of range", flightLink, []string{"BCN", "AGP", "--date", "+30", "--discount", "9"}, "discount must be between 0 and 5"},

		{"airports both formats", airportSearch, []string{"--json", "--toon"}, "cannot be used together"},
		{"airports unknown option", airportSearch, []string{"--all"}, "unknown option --all"},
		{"airports two terms", airportSearch, []string{"Barcelona", "Madrid"}, "use one city"},

		{"version both formats", showVersion, []string{"--json", "--toon"}, "cannot be used together"},
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
	err := cli.flightLink(strings.Fields("barcelona agp --date +30 --adults=2 --children 1 --flight vy2115 --json"))
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

func TestResolveAirport(t *testing.T) {
	for _, tc := range []struct {
		value, want string
		ok          bool
	}{
		{" bcn ", "BCN", true},
		{"Barcelona", "BCN", true},
		{"Heathrow", "LHR", true},
		{"Londres", "", false},
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
