package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
	"github.com/seifreed/vuelingcli/internal/vueling/api"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

const twoDeals = `{"bestPrices":{"value":[
{"destinationCode":"BIO","isDirect":true,"departureDate":"2026-11-30T11:50:00","integerPartPrice":22,"decimalPartPrice":0},
{"destinationCode":"AMS","isDirect":false,"departureDate":"2026-12-09T06:10:00","integerPartPrice":43,"decimalPartPrice":50}],"isSuccess":true}}`

func newDealsCLI(t *testing.T) (*CLI, *bytes.Buffer, *[]string) {
	t.Helper()
	var destinations []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		destinations = strings.Split(r.URL.Query().Get("destinationCodes"), ",")
		_, _ = w.Write([]byte(twoDeals))
	}))
	t.Cleanup(server.Close)
	stdout := new(bytes.Buffer)
	cli := New("test", stdout, new(bytes.Buffer), func(string) web.PageReader { return testPageReader{} })
	return cli.WithAPI(&api.Client{HTTP: server.Client(), APIW: server.URL}), stdout, &destinations
}

func TestDealsListsEveryDestinationCheapestFirst(t *testing.T) {
	cli, stdout, destinations := newDealsCLI(t)
	if err := cli.deals([]string{"Barcelona"}); err != nil {
		t.Fatal(err)
	}
	if len(*destinations) < 100 || strings.Contains(strings.Join(*destinations, ","), "BCN") {
		t.Fatalf("queried %d destinations %v, want every station but the origin", len(*destinations), *destinations)
	}
	want := "BIO  Bilbao                     30/11/2026 22 EUR\nAMS  Ámsterdam                  09/12/2026 43.50 EUR\n"
	if stdout.String() != want {
		t.Fatalf("deals printed %q, want %q", stdout, want)
	}
}

func TestDealsFiltersAndPrintsJSON(t *testing.T) {
	cli, stdout, destinations := newDealsCLI(t)
	if err := cli.deals([]string{"BCN", "--to", "bilbao,AMS", "--direct", "--max", "30", "--json"}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(*destinations, ",") != "BIO,AMS" {
		t.Fatalf("destinations = %v", *destinations)
	}
	var deals []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &deals); err != nil {
		t.Fatal(err)
	}
	if len(deals) != 1 || deals[0]["destination"] != "BIO" || deals[0]["city"] != "Bilbao" || !strings.Contains(deals[0]["calendarUrl"].(string), "dateComplete=30%2F11%2F2026") {
		t.Fatalf("deals = %v", deals)
	}
	if err := cli.deals([]string{"BCN", "--max", "10"}); err == nil || !strings.Contains(err.Error(), "no deals from BCN") {
		t.Fatalf("error = %v, want no deals", err)
	}
}

func TestDealsRejectsBadOptions(t *testing.T) {
	cli, _, _ := newDealsCLI(t)
	for args, want := range map[string]string{
		"":                  "usage: vueling deals",
		"Atlantis":          "airport catalogue",
		"BCN --json --toon": "cannot be used together",
		"BCN --max":         "--max requires a value",
		"BCN --months 13":   "--months must be",
		"BCN --limit -1":    "--limit must be",
		"BCN --max 0":       "--max must be",
		"BCN --currency XX": "currency",
		"BCN --to Narnia":   "Narnia",
		"BCN --from ayer":   "invalid --from",
		"BCN --nope 1":      "unknown option --nope",
	} {
		if err := cli.deals(strings.Fields(args)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("deals %q error = %v, want %q", args, err, want)
		}
	}
}

func TestDealsReportsMissingAPIAndAPIError(t *testing.T) {
	cli, _, _ := newTestCLI()
	if err := cli.deals([]string{"BCN"}); err == nil || !strings.Contains(err.Error(), "could not be initialised") {
		t.Fatalf("deals without API error = %v", err)
	}
	apiErr := errors.New("service unavailable")
	cli.api = &stubAPI{dealsFn: func(context.Context, string, []string, time.Time, int, string) ([]vueling.Deal, error) {
		return nil, apiErr
	}}
	if err := cli.deals([]string{"BCN"}); !errors.Is(err, apiErr) {
		t.Fatalf("deals API error = %v, want %v", err, apiErr)
	}
	cli.api = &stubAPI{dealsFn: func(context.Context, string, []string, time.Time, int, string) ([]vueling.Deal, error) {
		return []vueling.Deal{{Destination: "BIO", Date: "bad date"}}, nil
	}}
	if err := cli.deals([]string{"BCN"}); err == nil || !strings.Contains(err.Error(), "invalid deal date") {
		t.Fatalf("deals invalid date error = %v", err)
	}
}

func TestDealsPropagatesStructuredOutputError(t *testing.T) {
	cli := New("test", errorWriter{err: errors.New("stdout unavailable")}, new(bytes.Buffer), nil).WithAPI(&stubAPI{
		dealsFn: func(context.Context, string, []string, time.Time, int, string) ([]vueling.Deal, error) {
			return []vueling.Deal{{Destination: "BIO", Date: "2026-11-30", Price: 20, Currency: "EUR"}}, nil
		},
	})
	if err := cli.deals([]string{"BCN", "--to", "BIO", "--json"}); err == nil {
		t.Fatal("deals ignored a structured output error")
	}
}

func TestDealsPropagatesTextOutputError(t *testing.T) {
	cli := New("test", errorWriter{err: errors.New("stdout unavailable")}, new(bytes.Buffer), nil).WithAPI(&stubAPI{
		dealsFn: func(context.Context, string, []string, time.Time, int, string) ([]vueling.Deal, error) {
			return []vueling.Deal{{Destination: "BIO", Date: "2026-11-30", Price: 20, Currency: "EUR"}}, nil
		},
	})
	if err := cli.deals([]string{"BCN", "--to", "BIO"}); err == nil {
		t.Fatal("deals ignored a text output error")
	}
}
