package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
	"github.com/seifreed/vuelingcli/internal/vueling/api"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

// availabilityJSON renders an availability response with one fare per day offset from base.
func availabilityJSON(base time.Time, fares map[int]float64) string {
	var rows []string
	for offset, price := range fares {
		rows = append(rows, fmt.Sprintf(`{"arrivalDate":"%sT00:00:00","price":%v,"currency":"EUR","isInvalidPrice":false}`, base.AddDate(0, 0, offset).Format(time.DateOnly), price))
	}
	return `{"availability":{"value":{"data":[` + strings.Join(rows, ",") + `]}}}`
}

func newTripsCLI(t *testing.T, base time.Time) (*CLI, *bytes.Buffer, *[]string) {
	t.Helper()
	var routes []string
	outbound := availabilityJSON(base, map[int]float64{0: 22, 1: 30, 2: 15.5})
	inbound := availabilityJSON(base, map[int]float64{3: 31, 4: 20, 5: 40, 6: 9.99})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		routes = append(routes, query.Get("originCode")+"-"+query.Get("destinationCode"))
		if query.Get("originCode") == "BCN" {
			_, _ = w.Write([]byte(outbound))
			return
		}
		_, _ = w.Write([]byte(inbound))
	}))
	t.Cleanup(server.Close)
	stdout := new(bytes.Buffer)
	cli := New("test", stdout, new(bytes.Buffer), func(string) web.PageReader { return testPageReader{} })
	return cli.WithAPI(&api.Client{HTTP: server.Client(), APIW: server.URL}), stdout, &routes
}

func tripsBase() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 10)
}

func TestTripsListsCheapestCombinationsFirst(t *testing.T) {
	base := tripsBase()
	cli, stdout, routes := newTripsCLI(t, base)
	day := func(offset int) string { return base.AddDate(0, 0, offset).Format("02/01/2006") }
	if err := cli.trips([]string{"Barcelona", "BIO", "--from", base.Format(time.DateOnly), "--to", base.AddDate(0, 0, 2).Format(time.DateOnly), "--nights", "3-4", "--limit", "3"}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(*routes, ",") != "BCN-BIO,BIO-BCN" {
		t.Fatalf("queried %v, want one call per direction", *routes)
	}
	want := fmt.Sprintf("%s → %s  4 noches  out 15.50 + ret 9.99 = 25.49 EUR\n%s → %s  4 noches  out 22 + ret 20 = 42 EUR\n%s → %s  3 noches  out 30 + ret 20 = 50 EUR\n", day(2), day(6), day(0), day(4), day(1), day(4))
	if stdout.String() != want {
		t.Fatalf("trips printed\n%s\nwant\n%s", stdout, want)
	}
}

func TestTripsFiltersByMaxAndPrintsJSON(t *testing.T) {
	base := tripsBase()
	cli, stdout, _ := newTripsCLI(t, base)
	if err := cli.trips([]string{"BCN", "BIO", "--from", base.Format(time.DateOnly), "--nights", "4", "--max", "42", "--json"}); err != nil {
		t.Fatal(err)
	}
	var trips []vueling.Trip
	if err := json.Unmarshal(stdout.Bytes(), &trips); err != nil {
		t.Fatal(err)
	}
	if len(trips) != 2 || trips[0].Total != 25.49 || trips[0].Nights != 4 || trips[1].Total != 42 || trips[1].Currency != "EUR" {
		t.Fatalf("trips = %+v", trips)
	}
	wantLink := "dd=" + base.AddDate(0, 0, 2).Format(time.DateOnly) + "&o=BCN&rd=" + base.AddDate(0, 0, 6).Format(time.DateOnly)
	if !strings.Contains(trips[0].URL, wantLink) {
		t.Fatalf("url = %q, want it to contain %q", trips[0].URL, wantLink)
	}
	if err := cli.trips([]string{"BCN", "BIO", "--from", base.Format(time.DateOnly), "--nights", "4", "--max", "10"}); err == nil || !strings.Contains(err.Error(), "no BCN–BIO trips") {
		t.Fatalf("error = %v, want no trips", err)
	}
}

func TestTripsRejectsBadOptions(t *testing.T) {
	cli, _, _ := newTripsCLI(t, tripsBase())
	for args, want := range map[string]string{
		"BCN BIO":                              "usage: vueling trips",
		"BCN --nights 3":                       "usage: vueling trips",
		"BCN BCN --nights 3":                   "must be different",
		"Narnia BIO --nights 3":                "origin:",
		"BCN Narnia --nights 3":                "destination:",
		"BCN BIO --nights 0":                   "--nights must be",
		"BCN BIO --nights 31":                  "--nights must be",
		"BCN BIO --nights 5-3":                 "--nights must be",
		"BCN BIO --nights x":                   "--nights must be",
		"BCN BIO --nights 3 --from ayer":       "invalid --from",
		"BCN BIO --nights 3 --to nope":         "invalid --to",
		"BCN BIO --nights 3 --from +5 --to +2": "--to must be on or after --from",
		"BCN BIO --nights 3 --to +400":         "--to must be by",
		"BCN BIO --nights 3 --from +400":       "end of Vueling's fare calendar",
		"BCN BIO --nights 3 --currency XX":     "currency",
		"BCN BIO --nights 3 --nope 1":          "unknown option --nope",
		"BCN BIO --nights 3 --json --toon":     "cannot be used together",
		"BCN BIO --nights":                     "requires a value",
	} {
		if err := cli.trips(strings.Fields(args)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("trips %q error = %v, want %q", args, err, want)
		}
	}
}

func TestParseTripsOptionsDefaultsWindowAndClampsToHorizon(t *testing.T) {
	now := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	options, err := parseTripsOptions([]string{"BCN", "BIO", "--nights", "3-5"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if options.from.Format(time.DateOnly) != "2026-11-01" || options.to.Format(time.DateOnly) != "2026-12-01" || options.limit != 10 || options.minNights != 3 || options.maxNights != 5 || options.currency != "EUR" {
		t.Fatalf("options = %+v", options)
	}
	options, err = parseTripsOptions([]string{"BCN", "BIO", "--nights", "5", "--from", "2027-09-20"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if options.to.Format(time.DateOnly) != "2027-09-26" {
		t.Fatalf("to = %v, want the window clamped so the return is by 2027-10-01", options.to)
	}
}

func TestTripsNeedsAPIAndPrintsHelp(t *testing.T) {
	stdout := new(bytes.Buffer)
	cli := New("test", stdout, new(bytes.Buffer), func(string) web.PageReader { return testPageReader{} })
	if err := cli.trips([]string{"BCN", "BIO", "--nights", "3"}); err == nil || !strings.Contains(err.Error(), "HTTP API") {
		t.Fatalf("error = %v, want missing API", err)
	}
	if code := cli.Run([]string{"trips", "--help"}); code != 0 || !strings.Contains(stdout.String(), "--nights N[-M]") {
		t.Fatalf("help exit %d printed %q", code, stdout)
	}
	if code := cli.Run([]string{"trips", "BCN"}); code != 2 {
		t.Fatalf("usage error exit %d, want 2", code)
	}
}
