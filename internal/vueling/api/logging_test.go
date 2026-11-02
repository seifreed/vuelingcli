package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/observability"
)

func TestHTTPLoggingExcludesRequestAndResponseSecrets(t *testing.T) {
	var logs bytes.Buffer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	logger := observability.New(&logs, observability.Config{Environment: "test", Level: observability.TraceLevel, Format: "json"}).With("component", "api")
	client := &Client{HTTP: server.Client(), Logger: logger}
	var result struct{ OK bool }
	err := client.postJSON(context.Background(), server.URL+"/flight/VY1234?token=url-secret", "https://example.test", "header-secret", map[string]string{"password": "body-secret"}, &result)
	if err != nil || !result.OK {
		t.Fatalf("postJSON = %v, result=%+v", err, result)
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d HTTP events, want trace start and debug completion: %s", len(lines), logs.String())
	}
	var start, finish map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &start); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &finish); err != nil {
		t.Fatal(err)
	}
	if start["event_name"] != "http.request.started" || finish["event_name"] != "http.request.finished" ||
		finish["method"] != http.MethodPost || finish["status_code"] != float64(http.StatusOK) || finish["outcome"] != "success" ||
		finish["component"] != "api" || finish["remote_host"] == nil || finish["duration_ms"] == nil {
		t.Fatalf("unexpected HTTP events: %#v %#v", start, finish)
	}
	for _, secret := range []string{"url-secret", "header-secret", "body-secret", "VY1234", "flight/"} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("HTTP log exposed %q: %s", secret, logs.String())
		}
	}
	t.Logf("sample HTTP JSON events:\n%s", logs.String())
}

func TestHTTPLoggingReportsSafeStatusAndCancellation(t *testing.T) {
	t.Run("status", func(t *testing.T) {
		var logs bytes.Buffer
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "access token response-secret", http.StatusForbidden)
		}))
		defer server.Close()
		logger := observability.New(&logs, observability.Config{Environment: "test", Level: slog.LevelDebug, Format: "json"})
		client := &Client{HTTP: server.Client(), Logger: logger}
		_ = client.getJSON(context.Background(), server.URL+"/private/path?token=url-secret", "https://example.test", &struct{}{})
		var event map[string]any
		if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &event); err != nil {
			t.Fatalf("invalid HTTP error event: %v: %s", err, logs.String())
		}
		if event["outcome"] != "error" || event["error_type"] != "http_status" || event["error_code"] != "http_403" || event["status_code"] != float64(http.StatusForbidden) {
			t.Fatalf("unexpected HTTP error event: %#v", event)
		}
		for _, secret := range []string{"url-secret", "response-secret", "private/path"} {
			if strings.Contains(logs.String(), secret) {
				t.Errorf("HTTP error event exposed %q: %s", secret, logs.String())
			}
		}
	})
	t.Run("cancelled", func(t *testing.T) {
		var logs bytes.Buffer
		logger := observability.New(&logs, observability.Config{Environment: "test", Level: slog.LevelDebug, Format: "json"})
		client := &Client{HTTP: &http.Client{}, Logger: logger}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := client.getJSON(ctx, "https://api.example.test/data", "https://example.test", &struct{}{})
		if err == nil || !strings.Contains(logs.String(), `"outcome":"cancelled"`) || !strings.Contains(logs.String(), `"error_type":"context_canceled"`) {
			t.Fatalf("cancelled HTTP request = %v, logs=%s", err, logs.String())
		}
	})
	t.Run("deadline", func(t *testing.T) {
		var logs bytes.Buffer
		logger := observability.New(&logs, observability.Config{Environment: "test", Level: slog.LevelDebug, Format: "json"})
		client := &Client{HTTP: &http.Client{}, Logger: logger}
		ctx, cancel := context.WithTimeout(context.Background(), 0)
		defer cancel()
		_ = client.getJSON(ctx, "https://api.example.test/data", "https://example.test", &struct{}{})
		if !strings.Contains(logs.String(), `"error_type":"deadline_exceeded"`) || !strings.Contains(logs.String(), `"outcome":"timeout"`) {
			t.Fatalf("deadline outcome missing: %s", logs.String())
		}
	})
	t.Run("decode failure", func(t *testing.T) {
		var logs bytes.Buffer
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{")) }))
		defer server.Close()
		logger := observability.New(&logs, observability.Config{Environment: "test", Level: slog.LevelDebug, Format: "json"})
		client := &Client{HTTP: server.Client(), Logger: logger}
		_ = client.getJSON(context.Background(), server.URL, "https://example.test", &struct{}{})
		if !strings.Contains(logs.String(), `"error_type":"*json.SyntaxError"`) {
			t.Fatalf("decode failure type missing: %s", logs.String())
		}
	})
}

func TestEmptySessionStorePathIsANoop(t *testing.T) {
	if err := storeSessionFile("", Session{}, os.CreateTemp); err != nil {
		t.Fatalf("storeSessionFile with empty path = %v", err)
	}
}

func TestSessionCacheLoggingReportsStateWithoutTokenOrPath(t *testing.T) {
	var logs bytes.Buffer
	path := filepath.Join(t.TempDir(), "session.json")
	logger := observability.New(&logs, observability.Config{Environment: "test", Level: slog.LevelDebug, Format: "json"}).With("component", "api")
	client := &Client{SessionFile: path, Logger: logger}
	if _, err := client.StoredSession(); err != nil {
		t.Fatal(err)
	}
	session := Session{Token: "session-private-token", Expires: time.Now().Add(-time.Minute)}
	if err := client.storeSession(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	if stored, err := client.StoredSession(); err != nil || stored.Token != session.Token {
		t.Fatalf("StoredSession = %+v, %v", stored, err)
	}
	if err := client.ClearSession(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, event := range []string{"session.cache.read", "session.cache.write", "session.cache.remove"} {
		if !strings.Contains(logs.String(), event) {
			t.Errorf("cache event %q missing: %s", event, logs.String())
		}
	}
	if !strings.Contains(logs.String(), `"cache_hit":false`) || !strings.Contains(logs.String(), `"cache_hit":true`) {
		t.Errorf("cache hit state missing: %s", logs.String())
	}
	for _, secret := range []string{session.Token, path} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("session log exposed %q: %s", secret, logs.String())
		}
	}
}
