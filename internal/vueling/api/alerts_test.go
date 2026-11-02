package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

func TestAlertStripsTheBannerHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/api/AlertsApi/getPaeAlert" || r.URL.Query().Get("culture") != "en-GB" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"pae":true,"text":"<p>Strike in <b>France</b> &amp; delays</p>","url":"https://www.vueling.com/en/alert"}`))
	}))
	defer server.Close()
	alert, err := (&Client{HTTP: server.Client(), WWW: server.URL}).Alert(context.Background(), "en-GB")
	want := vueling.Alert{Active: true, Text: "Strike in France & delays", URL: "https://www.vueling.com/en/alert"}
	if err != nil || alert != want {
		t.Fatalf("Alert = %+v, %v; want %+v", alert, err, want)
	}
}

func TestAlertRejectsMissingStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	_, err := (&Client{HTTP: server.Client(), WWW: server.URL}).Alert(context.Background(), "es-ES")
	if err == nil || err.Error() != "Vueling's alert response has no pae status" {
		t.Fatalf("Alert error = %v; want missing status error", err)
	}
}

func TestAlertRejectsActiveNoticeWithoutDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"pae":true}`))
	}))
	defer server.Close()
	_, err := (&Client{HTTP: server.Client(), WWW: server.URL}).Alert(context.Background(), "es-ES")
	if err == nil || err.Error() != "Vueling's active alert response has no text or URL" {
		t.Fatalf("Alert error = %v; want missing alert details error", err)
	}
}
