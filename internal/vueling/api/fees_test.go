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

func feesServer(t *testing.T, page []byte) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != feesPath {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(page)
	}))
	t.Cleanup(server.Close)
	return &Client{HTTP: server.Client(), WWW: server.URL}
}

func TestFeesReadsTheWebsiteFeeTable(t *testing.T) {
	fees, err := feesServer(t, readFixture(t, "fees.html")).Fees(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []vueling.Fee{
		{Service: "Asientos estándar", Online: "De 5,0 EUR a 30,0 EUR", Phone: "De 5,0 EUR a 30,0 EUR", Airport: "De 5,0 EUR a 32,0 EUR"},
		{Service: "Maleta facturada de hasta 15 kg", Online: "De 10,0 EUR a 95,0 EUR por pieza y vuelo", Phone: "De 10,0 EUR a 95,0 EUR por pieza y vuelo"},
		{Service: "Mascota en cabina (nacional)", Online: "50,0 EUR", Phone: "50,0 EUR", Airport: "60,0 EUR"},
	}
	if !reflect.DeepEqual(fees, want) {
		t.Fatalf("fees = %+v, want %+v", fees, want)
	}
}

func TestFeesReportsAPageWithoutTable(t *testing.T) {
	_, err := feesServer(t, []byte("<html></html>")).Fees(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no fee table") {
		t.Fatalf("error = %v", err)
	}
}

func TestFeesReportsAnEmptyFeeTable(t *testing.T) {
	page := []byte(`<vy-spa-table settings='{"dataSource":{"bodyData":[]}}'>`)
	_, err := feesServer(t, page).Fees(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no fee rows") {
		t.Fatalf("error = %v, want an empty fee table error", err)
	}
}

func TestFeesRejectsRowsWithoutAService(t *testing.T) {
	page := []byte(`<vy-spa-table settings='{"dataSource":{"bodyData":[{"1":{"contentText":"5 EUR"}}]}}'>`)
	_, err := feesServer(t, page).Fees(context.Background())
	if err == nil || !strings.Contains(err.Error(), "row without a service") {
		t.Fatalf("error = %v, want a missing service error", err)
	}
}
