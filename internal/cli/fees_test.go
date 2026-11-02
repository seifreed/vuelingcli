package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/seifreed/vuelingcli/internal/vueling/api"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

func TestFeesFiltersByServiceName(t *testing.T) {
	page, err := os.ReadFile("../vueling/api/testdata/fees.html")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(page) }))
	defer server.Close()
	stdout := new(bytes.Buffer)
	cli := New("test", stdout, new(bytes.Buffer), func(string) web.PageReader { return testPageReader{} }).WithAPI(&api.Client{HTTP: server.Client(), WWW: server.URL})
	if err := cli.fees([]string{"MALETA"}); err != nil {
		t.Fatal(err)
	}
	want := "Maleta facturada de hasta 15 kg\n  online: De 10,0 EUR a 95,0 EUR por pieza y vuelo · atención al cliente: De 10,0 EUR a 95,0 EUR por pieza y vuelo · aeropuerto: —\n"
	if stdout.String() != want {
		t.Fatalf("fees printed %q, want %q", stdout, want)
	}
	stdout.Reset()
	if err := cli.fees([]string{"estandar", "--json"}); err != nil || !strings.Contains(stdout.String(), `"service":"Asientos estándar"`) {
		t.Fatalf("fees --json = %v, %q", err, stdout)
	}
	if err := cli.fees([]string{"unicornio"}); err == nil || !strings.Contains(err.Error(), "no Vueling fee matches") {
		t.Fatalf("error = %v", err)
	}
	if err := cli.fees([]string{"-x"}); err == nil {
		t.Fatal("want an unknown-option error")
	}
}
