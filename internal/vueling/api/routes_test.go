package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

const routeNetwork = `{"stations":{"value":[
{"code":"LCG","name":"A Coruña","routes":[
 {"code":"RVN","name":"Rovaniemi (Laponia)","countryCode":"FI","countryName":"Finlandia","isConnection":true},
 {"code":"PAR","name":"París","countryCode":"FR","countryName":"Francia","isMac":true},
 {"code":"BCN","name":"Barcelona","countryCode":"ES","countryName":"España","isLargeFamily":true,"isResidents":true}]},
{"code":"BCN","name":"Barcelona","routes":[]}]}}`

func routesServer(t *testing.T) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/corporativeCMS/Stations/es-ES.json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(routeNetwork))
	}))
	t.Cleanup(server.Close)
	return &Client{HTTP: server.Client(), Static: server.URL}
}

func TestRoutesListsTheOriginsDestinationsWithoutCityGroups(t *testing.T) {
	routes, err := routesServer(t).Routes(context.Background(), "LCG")
	if err != nil {
		t.Fatal(err)
	}
	want := []vueling.Route{
		{Code: "BCN", Name: "Barcelona", CountryCode: "ES", Country: "España", Direct: true, Residents: true, LargeFamily: true},
		{Code: "RVN", Name: "Rovaniemi (Laponia)", CountryCode: "FI", Country: "Finlandia"},
	}
	if !reflect.DeepEqual(routes, want) {
		t.Fatalf("routes = %+v, want %+v", routes, want)
	}
}

func TestRoutesReportsAnUnknownOrigin(t *testing.T) {
	_, err := routesServer(t).Routes(context.Background(), "XXX")
	if err == nil || !strings.Contains(err.Error(), "no flights from XXX") {
		t.Fatalf("error = %v", err)
	}
}
