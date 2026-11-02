package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	utls "github.com/refraction-networking/utls"
)

func TestPostJSONSendsChromeHeadersAndToken(t *testing.T) {
	version := utls.HelloChrome_Auto.Version
	wantUserAgent := fmt.Sprintf("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s.0.0.0 Safari/537.36", version)
	wantClientHints := fmt.Sprintf(`"Google Chrome";v="%s", "Not?A_Brand";v="8", "Chromium";v="%s"`, version, version)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Header.Get("User-Agent") != wantUserAgent || r.Header.Get("Sec-Ch-Ua") != wantClientHints || r.Header.Get("Sec-Fetch-Mode") != "cors" || r.Header.Get("Origin") != ticketsOrigin || r.Header.Get("Authorization") != "Bearer tok" || string(body) != `{"a":1}` {
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
	if !errors.As(err, &status) || status.Status != http.StatusForbidden || !status.CDNBlocked || !strings.Contains(err.Error(), "--browser") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("getJSON error = %v, want a CDN block without the query string", err)
	}
}

func TestStatusErrorDoesNotExposeURLOrResponseBody(t *testing.T) {
	status := &StatusError{Status: http.StatusBadGateway}
	err := status.Error()
	if !strings.Contains(err, "HTTP 502") || strings.Contains(err, "https://") {
		t.Fatalf("StatusError exposed request details: %q", err)
	}
	if status.HTTPStatusCode() != http.StatusBadGateway {
		t.Fatalf("HTTPStatusCode = %d", status.HTTPStatusCode())
	}
}

func TestDoPreservesTransportCauseWithoutExposingRequestURL(t *testing.T) {
	cause := errors.New("connection refused")
	client := &Client{HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, cause })}}
	err := client.getJSON(context.Background(), "https://api.example.test/private/path?token=private-token", wwwOrigin, &struct{}{})
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "call Vueling API failed") || strings.Contains(err.Error(), "private-token") || strings.Contains(err.Error(), "/private/path") {
		t.Fatalf("transport error leaked request details or lost its cause: %v", err)
	}
}

func TestGetJSONRejectsOversizedRawResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("x", maxBody+1))
	}))
	defer server.Close()
	var body []byte
	err := (&Client{HTTP: server.Client()}).getJSON(context.Background(), server.URL, wwwOrigin, &body)
	if err == nil || !strings.Contains(err.Error(), "exceeds") || body != nil {
		t.Fatalf("getJSON error = %v, response length = %d; want explicit size error and no truncated body", err, len(body))
	}
}

func TestClientDoesNotFollowAPIRedirects(t *testing.T) {
	targetHit := make(chan struct{}, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetHit <- struct{}{}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.RedirectHandler(target.URL, http.StatusFound))
	defer redirect.Close()

	client, err := newClient(func() (*http.Transport, error) { return &http.Transport{}, nil })
	if err != nil {
		t.Fatal(err)
	}
	err = client.do(context.Background(), http.MethodGet, redirect.URL, ticketsOrigin, "session-token", nil, new([]byte))
	var status *StatusError
	if !errors.As(err, &status) || status.Status != http.StatusFound {
		t.Fatalf("redirect response error = %v; want HTTP 302", err)
	}
	select {
	case <-targetHit:
		t.Fatal("API client followed a redirect to another host")
	default:
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type closeErrorBody struct {
	io.Reader
	err error
}

func (body closeErrorBody) Close() error { return body.err }

func TestDoReportsResponseCloseErrors(t *testing.T) {
	closeErr := errors.New("response close failed")
	for _, body := range []string{`{"ok":true}`, "{"} {
		var out any = new([]byte)
		if body == "{" {
			out = new(struct{})
		}
		client := &Client{HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: closeErrorBody{Reader: strings.NewReader(body), err: closeErr}}, nil
		})}}
		err := client.do(context.Background(), http.MethodGet, "https://example.test", wwwOrigin, "", nil, out)
		if !errors.Is(err, closeErr) {
			t.Fatalf("do error = %v; want response close error", err)
		}
		if body == "{" && !strings.Contains(err.Error(), "decode Vueling API response") {
			t.Fatalf("do error = %v; want decode and close errors", err)
		}
	}
}

func TestChromeSpecPinsHTTP1(t *testing.T) {
	if _, err := newChromeTransport(); err != nil {
		t.Fatal(err)
	}
}

type failingCloseConn struct {
	net.Conn
	err error
}

func (conn failingCloseConn) Close() error { return conn.err }

func TestCloseAfterErrorReportsCloseFailure(t *testing.T) {
	primaryErr, closeErr := errors.New("handshake failed"), errors.New("close failed")
	err := closeAfterError(failingCloseConn{err: closeErr}, primaryErr)
	if !errors.Is(err, primaryErr) || !errors.Is(err, closeErr) {
		t.Fatalf("closeAfterError = %v; want both the primary and close errors", err)
	}
}
