package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling/api"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

func newScheduleCLI(t *testing.T, body []byte) (*CLI, *bytes.Buffer) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/Auth") {
			_, _ = w.Write([]byte(`{"accessToken":"jwt","expiration":1199}`))
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	stdout := new(bytes.Buffer)
	cli := New("test", stdout, new(bytes.Buffer), func(string) web.PageReader { return testPageReader{} })
	return cli.WithAPI(&api.Client{HTTP: server.Client(), AMS: server.URL}), stdout
}

func readAllFlightsFixture(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile("../vueling/api/testdata/allflights-bcn-esu.json")
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestSchedulePrintsOperatingDaysPerMonth(t *testing.T) {
	cli, stdout := newScheduleCLI(t, readAllFlightsFixture(t))
	if err := cli.schedule([]string{"BCN", "ESU", "--from", "2027-03-05", "--months", "2", "--days"}); err != nil {
		t.Fatal(err)
	}
	want := "BCN → ESU · 2027-03-05 a 2027-04-30\n" +
		"7 días · mié sáb · primer vuelo 2027-03-06 · último 2027-03-27\n" +
		"mar 2027   7 días  mié sáb                      6 10 13 17 20 24 27\n" +
		"abr 2027  sin vuelos\n" +
		"2027-03-06 sáb  VY8502  18:00  desde 27,78 €\n"
	if !strings.HasPrefix(stdout.String(), want) || strings.Count(stdout.String(), "\n") != 11 {
		t.Fatalf("schedule printed %q, want prefix %q", stdout, want)
	}
}

func TestScheduleWritesJSONAndReportsRoutesWithoutFlights(t *testing.T) {
	cli, stdout := newScheduleCLI(t, readAllFlightsFixture(t))
	if err := cli.schedule([]string{"BCN", "ESU", "--from=2027-03-01", "--months=1", "--json"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"firstDate":"2027-03-03"`, `"weekdays":["mié","sáb"]`, `"days":[3,6,10,13,17,20,24,27]`, `"flight":"VY8502"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("schedule --json = %s, want %s", stdout, want)
		}
	}
	empty, stdout := newScheduleCLI(t, nil)
	if err := empty.schedule([]string{"BCN", "FEZ", "--months", "1"}); err != nil || !strings.Contains(stdout.String(), "sin vuelos a la venta") {
		t.Fatalf("schedule without flights = %v, %q", err, stdout)
	}
}

func TestScheduleHelpAndErrors(t *testing.T) {
	cli, stdout := newScheduleCLI(t, nil)
	if err := cli.schedule([]string{"--help"}); err != nil || !strings.Contains(stdout.String(), "vueling schedule <origin") {
		t.Fatalf("help = %v, %q", err, stdout)
	}
	for args, want := range map[string]string{
		"BCN":                      "usage: vueling schedule",
		"BCN BCN":                  "must differ",
		"BCN BIO --months 13":      "--months must be a number from 1 to 12",
		"BCN BIO --months x":       "--months must be",
		"BCN BIO --nope 1":         "unknown option --nope",
		"BCN BIO -x":               "unknown option -x",
		"BCN BIO --currency ZZZ":   "currency",
		"BCN BIO --from yesterday": "invalid date",
		"BCN BIO --json --toon":    "cannot be used together",
		"BCN BIO --from":           "requires a value",
		"Atlantida BIO":            "origin",
		"BCN Atlantida":            "destination",
	} {
		if _, err := parseScheduleOptions(strings.Fields(args), time.Now()); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("schedule %q error = %v, want %q", args, err, want)
		}
	}
	if err := New("test", stdout, stdout, nil).schedule([]string{"BCN", "BIO"}); err == nil || !strings.Contains(err.Error(), "HTTP API") {
		t.Fatalf("error without API = %v", err)
	}
}
