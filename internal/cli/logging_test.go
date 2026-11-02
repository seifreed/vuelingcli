package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/seifreed/vuelingcli/internal/observability"
	"github.com/seifreed/vuelingcli/internal/vueling/api"
	"github.com/seifreed/vuelingcli/internal/vueling/ports"
)

func TestRunLogsCommandLifecycleWithoutChangingStdout(t *testing.T) {
	var logs, stdout, stderr bytes.Buffer
	logger := observability.New(&logs, observability.Config{Environment: "test", Level: observability.TraceLevel, Format: "json"})
	program := New("test", &stdout, &stderr, nil).WithLogger(logger.With("component", "cli"))
	if code := program.Run([]string{"version"}); code != 0 {
		t.Fatalf("Run exit code = %d", code)
	}
	if stdout.String() != "vueling test\n" || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d logs, want command start and completion: %s", len(lines), logs.String())
	}
	var first, last map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &last); err != nil {
		t.Fatal(err)
	}
	if first["level"] != "trace" || first["event_name"] != "command.started" ||
		last["level"] != "info" || last["event_name"] != "command.finished" || last["outcome"] != "success" ||
		last["operation"] != "version" || last["component"] != "cli" {
		t.Fatalf("unexpected lifecycle events: %#v %#v", first, last)
	}
	t.Logf("sample CLI JSON events:\n%s", logs.String())
}

func TestRunLogsSafeCommandFailureWithHTTPStatus(t *testing.T) {
	var logs, stderr bytes.Buffer
	logger := observability.New(&logs, observability.Config{Environment: "test", Level: slog.LevelInfo, Format: "json"})
	status := &api.StatusError{Status: 403}
	program := New("test", new(bytes.Buffer), &stderr, nil).
		WithAPIError(status).
		WithLogger(logger.With("component", "cli"))
	if code := program.Run([]string{"session"}); code != 2 {
		t.Fatalf("Run exit code = %d, want 2", code)
	}
	var event map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &event); err != nil {
		t.Fatalf("invalid failure event: %v: %s", err, logs.String())
	}
	if event["level"] != "error" || event["event_name"] != "command.finished" || event["error_type"] != "http_status" || event["status_code"] != float64(403) {
		t.Fatalf("unexpected failure event: %#v", event)
	}
	for _, secret := range []string{"https://", "access token", "private/path"} {
		if strings.Contains(logs.String()+stderr.String(), secret) {
			t.Fatalf("sensitive API detail %q was exposed in logs or CLI error: %s %s", secret, logs.String(), stderr.String())
		}
	}
}

func TestRunLogsWrappedNonHTTPFailureWithoutRawErrorText(t *testing.T) {
	var logs bytes.Buffer
	logger := observability.New(&logs, observability.Config{Environment: "test", Level: slog.LevelInfo, Format: "json"})
	program := New("test", new(bytes.Buffer), new(bytes.Buffer), nil).
		WithAPIError(errors.New("offline secret-value")).
		WithLogger(logger)
	if code := program.Run([]string{"session"}); code != 2 {
		t.Fatalf("Run exit code = %d, want 2", code)
	}
	var event map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &event); err != nil {
		t.Fatalf("invalid failure event: %v", err)
	}
	if event["error_type"] != "*errors.errorString" || event["status_code"] != nil || strings.Contains(logs.String(), "secret-value") {
		t.Fatalf("failure event leaked or misclassified the error: %#v %s", event, logs.String())
	}
}

func TestRunLogsRejectedCommandWithoutControlCharacters(t *testing.T) {
	var logs, stderr bytes.Buffer
	logger := observability.New(&logs, observability.Config{Environment: "test", Level: slog.LevelWarn, Format: "json"})
	program := New("test", new(bytes.Buffer), &stderr, nil).WithLogger(logger)
	if code := program.Run([]string{"bad\ncommand"}); code != 2 {
		t.Fatalf("Run exit code = %d", code)
	}
	if strings.Contains(logs.String(), "bad command") || !strings.Contains(logs.String(), "unknown_command") {
		t.Fatalf("rejected-command log exposed the raw command: %q", logs.String())
	}
}

func TestBrowserOperationsLogOutcomeWithoutPageDataOrURL(t *testing.T) {
	var logs bytes.Buffer
	logger := observability.New(&logs, observability.Config{Environment: "test", Level: slog.LevelDebug, Format: "json"})
	readErr := errors.New("page read failed")
	program := New("test", new(bytes.Buffer), new(bytes.Buffer), func(string) ports.PageReader {
		return loggingPageReader{err: readErr}
	}).WithLogger(logger)
	reader, err := program.pageReader("http://127.0.0.1:9222/?secret=private")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadPage(context.Background(), "https://example.test/page?token=private", "document.body"); !errors.Is(err, readErr) {
		t.Fatalf("ReadPage error = %v", err)
	}
	cancelledProgram := New("test", new(bytes.Buffer), new(bytes.Buffer), func(string) ports.PageReader {
		return loggingPageReader{err: context.Canceled}
	}).WithLogger(logger)
	cancelledReader, err := cancelledProgram.pageReader("")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := cancelledReader.ReadPage(ctx, "https://example.test/", "document.title"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled ReadPage error = %v", err)
	}
	if err := program.WithLinkOpener(func(string) error { return nil }).openLink("https://example.test/?token=private"); err != nil {
		t.Fatal(err)
	}
	if err := program.WithLinkOpener(func(string) error { return readErr }).openLink("https://example.test/?token=private"); !errors.Is(err, readErr) {
		t.Fatalf("openLink error = %v", err)
	}
	for _, forbidden := range []string{"127.0.0.1", "document.body", "page?token", "private"} {
		if strings.Contains(logs.String(), forbidden) {
			t.Fatalf("browser log exposed %q: %s", forbidden, logs.String())
		}
	}
	if strings.Count(logs.String(), "browser.page_read.finished") != 2 || strings.Count(logs.String(), "browser.open.finished") != 2 || !strings.Contains(logs.String(), `"outcome":"error"`) || !strings.Contains(logs.String(), `"outcome":"cancelled"`) {
		t.Fatalf("browser operation events missing outcomes: %s", logs.String())
	}
}

type loggingPageReader struct{ err error }

func (reader loggingPageReader) ReadPage(context.Context, string, string) ([]byte, error) {
	return []byte("page contents are not logged"), reader.err
}
