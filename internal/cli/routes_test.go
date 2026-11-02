package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/seifreed/vuelingcli/internal/vueling/api"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

func newRoutesCLI(t *testing.T) (*CLI, *bytes.Buffer) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"stations":{"value":[{"code":"LCG","routes":[
{"code":"RVN","name":"Rovaniemi (Laponia)","countryCode":"FI","countryName":"Finlandia","isConnection":true},
{"code":"BCN","name":"Barcelona","countryCode":"ES","countryName":"España"}]}]}}`))
	}))
	t.Cleanup(server.Close)
	stdout := new(bytes.Buffer)
	cli := New("test", stdout, new(bytes.Buffer), func(string) web.PageReader { return testPageReader{} })
	return cli.WithAPI(&api.Client{HTTP: server.Client(), Static: server.URL}), stdout
}

func TestRoutesPrintsDirectAndConnectingDestinations(t *testing.T) {
	cli, stdout := newRoutesCLI(t)
	if err := cli.routes([]string{"A Coruña"}); err != nil {
		t.Fatal(err)
	}
	want := "BCN  Barcelona                    España           directo\nRVN  Rovaniemi (Laponia)          Finlandia        conexión\n"
	if stdout.String() != want {
		t.Fatalf("routes printed %q, want %q", stdout, want)
	}
}

func TestRoutesFilters(t *testing.T) {
	cli, stdout := newRoutesCLI(t)
	if err := cli.routes([]string{"LCG", "--country", "fi", "--json"}); err != nil || !strings.Contains(stdout.String(), `"code":"RVN"`) || strings.Contains(stdout.String(), "BCN") {
		t.Fatalf("routes --country = %v, %q", err, stdout)
	}
	for args, want := range map[string]string{
		"LCG --direct --country Finlandia": "no destination from LCG",
		"":                                 "usage: vueling routes",
		"LCG --nope 1":                     "unknown option --nope",
		"LCG --json --toon":                "cannot be used together",
	} {
		if err := cli.routes(strings.Fields(args)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("routes %q error = %v, want %q", args, err, want)
		}
	}
}
