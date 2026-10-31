package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostJSONSendsChromeHeadersAndToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Header.Get("Sec-Fetch-Mode") != "cors" || r.Header.Get("Origin") != ticketsOrigin || r.Header.Get("Authorization") != "Bearer tok" || string(body) != `{"a":1}` {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	client := &Client{HTTP: server.Client()}
	var out struct{ OK bool }
	if err := client.postJSON(context.Background(), server.URL, ticketsOrigin, "tok", map[string]int{"a": 1}, &out); err != nil || !out.OK {
		t.Fatalf("postJSON = %v, %+v", err, out)
	}
}

func TestGetJSONReportsCDNBlock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "<H1>Access Denied</H1>", http.StatusForbidden)
	}))
	defer server.Close()
	err := (&Client{HTTP: server.Client()}).getJSON(context.Background(), server.URL+"/x?secret=1", wwwOrigin, &struct{}{})
	var status *StatusError
	if !errors.As(err, &status) || status.Status != http.StatusForbidden || !strings.Contains(err.Error(), "--browser") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("getJSON error = %v, want a CDN block without the query string", err)
	}
}

func TestChromeSpecPinsHTTP1(t *testing.T) {
	if _, err := newChromeTransport(); err != nil {
		t.Fatal(err)
	}
}
