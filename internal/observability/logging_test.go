package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConfigFromEnv(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		want Config
		warn []string
	}{
		{name: "defaults", want: Config{Environment: "development", Level: slog.LevelInfo, Format: "text"}},
		{name: "production", env: map[string]string{"VUELING_ENV": "production"}, want: Config{Environment: "production", Level: slog.LevelInfo, Format: "json"}},
		{name: "custom", env: map[string]string{"VUELING_ENV": "test", "VUELING_LOG_LEVEL": " TRACE ", "VUELING_LOG_FORMAT": "JSON"}, want: Config{Environment: "test", Level: TraceLevel, Format: "json"}},
		{name: "invalid values", env: map[string]string{"VUELING_LOG_LEVEL": "verbose-secret", "VUELING_LOG_FORMAT": "xml-secret"}, want: Config{Environment: "development", Level: slog.LevelInfo, Format: "text"}, warn: []string{"VUELING_LOG_LEVEL", "VUELING_LOG_FORMAT"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, warnings := ConfigFromEnv(func(key string) string { return tc.env[key] })
			if cfg != tc.want || strings.Join(warnings, ",") != strings.Join(tc.warn, ",") {
				t.Fatalf("ConfigFromEnv = %+v, %v; want %+v, %v", cfg, warnings, tc.want, tc.warn)
			}
		})
	}
	if _, warnings := ConfigFromEnv(nil); len(warnings) != 0 {
		t.Fatalf("ConfigFromEnv(nil) warnings = %v", warnings)
	}
}

func TestJSONHandlerNormalizesAndRedactsAttributes(t *testing.T) {
	var output bytes.Buffer
	logger := New(&output, Config{Environment: "test", Level: TraceLevel, Format: "json"}).With("component", "test")
	logger.LogAttrs(context.Background(), TraceLevel, "untrusted\nmessage",
		slog.String("event_name", "security.test"), slog.String("access_token", "top-secret"),
		slog.String("authorization", "Bearer private"), slog.String("detail", "first\nsecond\x00"),
		slog.String("cookie", "cookie-secret"), slog.String("api_key", "api-secret"),
		slog.String("private_key", "private-key-secret"), slog.String("client_secret", "client-secret"),
		slog.String("passwd", "passwd-secret"), slog.Group("credentials", slog.String("password", "private-pass")),
		slog.Group("session", slog.String("value", "session-secret")))
	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &record); err != nil {
		t.Fatalf("invalid JSON log: %v: %s", err, output.Bytes())
	}
	for key, want := range map[string]any{
		"level": "trace", "message": "untrusted message", "service": "vuelingcli", "environment": "test",
		"component": "test", "event_name": "security.test", "access_token": "[REDACTED]",
		"authorization": "[REDACTED]", "detail": "first second",
	} {
		if record[key] != want {
			t.Errorf("record[%q] = %#v, want %#v", key, record[key], want)
		}
	}
	credentials, ok := record["credentials"].(map[string]any)
	if !ok || credentials["password"] != "[REDACTED]" {
		t.Errorf("credentials = %#v, password was not redacted", record["credentials"])
	}
	session, ok := record["session"].(map[string]any)
	if !ok || session["value"] != "[REDACTED]" {
		t.Errorf("sensitive group value was not redacted: %#v", record["session"])
	}
	if _, ok := record["timestamp"].(string); !ok {
		t.Fatalf("timestamp missing or not a string: %#v", record["timestamp"])
	}
	if _, ok := record["time"]; ok {
		t.Fatal("non-normalized time key was emitted")
	}
	for _, secret := range []string{"top-secret", "private-pass", "cookie-secret", "api-secret", "private-key-secret", "client-secret", "passwd-secret", "session-secret"} {
		if strings.Contains(output.String(), secret) {
			t.Fatalf("sensitive value %q reached logs: %s", secret, output.String())
		}
	}
	if strings.Contains(output.String(), "first\\nsecond") {
		t.Fatalf("secret or untrusted line break reached logs: %s", output.String())
	}
}

func TestLevelFilteringAndFallbacks(t *testing.T) {
	var output bytes.Buffer
	logger := New(&output, Config{Level: slog.LevelInfo, Format: "text"})
	logger.Debug("hidden")
	logger.Info("visible")
	logger.Log(context.Background(), slog.Level(6), "custom")
	if strings.Contains(output.String(), "hidden") || !strings.Contains(output.String(), "visible") || !strings.Contains(output.String(), "custom") {
		t.Fatalf("level filtering output = %q", output.String())
	}
	New(nil, Config{}).Info("discarded safely")
	if got := cleanString(strings.Repeat("x", 257)); len([]rune(got)) != 257 || !strings.HasSuffix(got, "…") {
		t.Fatalf("cleanString did not cap input: length=%d value=%q", len([]rune(got)), got)
	}
	if !sensitiveKey("refresh-token") || !sensitiveKey("api.key") || sensitiveKey("status_code") {
		t.Fatal("sensitive key matching is incorrect")
	}
	if _, ok := parseLevel("fatal"); ok {
		t.Fatal("unsupported fatal level was accepted")
	}
	for name, want := range map[string]slog.Level{
		"debug": slog.LevelDebug, "info": slog.LevelInfo, "warn": slog.LevelWarn, "warning": slog.LevelWarn, "error": slog.LevelError,
	} {
		if got, ok := parseLevel(name); !ok || got != want {
			t.Errorf("parseLevel(%q) = %v, %v; want %v, true", name, got, ok, want)
		}
	}
	group := replaceAttr(nil, slog.Group("nested", slog.String("token", "secret")))
	if group.Value.Kind() != slog.KindGroup {
		t.Fatalf("group attribute changed: %#v", group)
	}
	if got := replaceAttr([]string{"session"}, slog.String("value", "secret")).Value.String(); got != "[REDACTED]" {
		t.Fatalf("sensitive group field = %q", got)
	}
	for _, level := range []slog.Level{slog.LevelDebug, slog.LevelWarn} {
		if got := replaceAttr(nil, slog.Any(slog.LevelKey, level)).Value.String(); got != strings.ToLower(level.String()) {
			t.Errorf("replaceAttr level %v = %q", level, got)
		}
	}
	if !sensitiveKey("apiKey") || !sensitiveKey("privateKey") {
		t.Fatal("camel-case key forms were not recognized")
	}
}

func TestConcurrentWritesAreCompleteJSONRecords(t *testing.T) {
	const count = 100
	var output bytes.Buffer
	logger := New(&output, Config{Environment: "test", Level: slog.LevelInfo, Format: "json"})
	var group sync.WaitGroup
	for i := 0; i < count; i++ {
		group.Add(1)
		go func(id int) {
			defer group.Done()
			logger.Info("concurrent", "event_name", "test.concurrent", "id", id)
		}(i)
	}
	group.Wait()
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != count {
		t.Fatalf("got %d records, want %d", len(lines), count)
	}
	for _, line := range lines {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("concurrent write produced invalid JSON: %v: %s", err, line)
		}
	}
}

func TestWriterFailuresDoNotPanic(t *testing.T) {
	logger := New(failingWriter{}, Config{Environment: "test", Level: slog.LevelInfo, Format: "json"})
	logger.Error("destination unavailable", "event_name", "test.write_failure")
	_, err := failingWriter{}.Write(nil)
	if !errors.Is(err, errWrite) {
		t.Fatal("test writer did not return its configured failure")
	}
}

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

func TestReplaceAttrUsesUTC(t *testing.T) {
	attr := replaceAttr(nil, slog.Time("time", time.Date(2026, 1, 1, 12, 0, 0, 0, time.FixedZone("west", -8*60*60))))
	if attr.Key != "timestamp" || attr.Value.Time().Location() != time.UTC || attr.Value.Time().Hour() != 20 {
		t.Fatalf("replaceAttr time = %s %s", attr.Key, attr.Value.Time())
	}
}
