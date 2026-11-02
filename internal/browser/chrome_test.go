package browser

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

func installCommand(t *testing.T, name, contents string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+contents), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

func TestChromeReaderUsesAppleEvents(t *testing.T) {
	requireAppleEvents(t)
	installCommand(t, "osascript", "printf '%s' '{\"ok\":true}'\n")
	reader := NewChromeReader("")
	if reader.endpoint != "" {
		t.Fatalf("NewChromeReader endpoint = %q", reader.endpoint)
	}
	got, err := readPageWithAppleEvents(context.Background(), "https://example.test", "document.title")
	if err != nil || string(got) != `{"ok":true}` {
		t.Fatalf("readPageWithAppleEvents = %q, %v", got, err)
	}
	if runtime.GOOS == "darwin" {
		got, err = reader.ReadPage(context.Background(), "https://example.test", "document.title")
		if err != nil || string(got) != `{"ok":true}` {
			t.Fatalf("ReadPage via Apple Events = %q, %v", got, err)
		}
	}
}

func TestChromeReaderReportsAppleEventsFailuresAndCancellation(t *testing.T) {
	requireAppleEvents(t)
	for _, tc := range []struct {
		name, output, want string
	}{
		{"empty output", "exit 1", "could not read the Vueling tab"},
		{"no window", "echo VUELING_NO_CHROME_WINDOW; exit 1", "no open window"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			installCommand(t, "osascript", tc.output+"\n")
			_, err := readPageWithAppleEvents(context.Background(), "https://example.test", "script")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("readPageWithAppleEvents error = %v, want %q", err, tc.want)
			}
		})
	}
	installCommand(t, "osascript", "exit 0\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readPageWithAppleEvents(ctx, "https://example.test", "script"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Apple Events read error = %v", err)
	}
}

func requireAppleEvents(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Apple Events requires a Unix command fixture")
	}
}

func TestReadPageWithDevToolsRejectsInvalidEndpoint(t *testing.T) {
	if _, err := readPageWithDevTools(context.Background(), "https://example.test", "https://page.test", "script"); err == nil || !strings.Contains(err.Error(), "local http URL") {
		t.Fatalf("readPageWithDevTools error = %v", err)
	}
	if _, err := NewChromeReader("https://example.test").ReadPage(context.Background(), "https://page.test", "script"); err == nil {
		t.Fatal("ReadPage accepted a non-local endpoint")
	}
}

func TestReadPageWithDevToolsRunsAgainstLocalCDP(t *testing.T) {
	server := newMockDevToolsServer(t, false)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	got, err := readPageWithDevTools(ctx, server.URL, "https://example.test", `[]`)
	if err != nil || string(got) != `{"ok":true}` {
		t.Fatalf("readPageWithDevTools = %q, %v", got, err)
	}
}

func TestReadPageWithDevToolsReportsNavigationError(t *testing.T) {
	server := newMockDevToolsServer(t, true)
	if _, err := readPageWithDevTools(context.Background(), server.URL, "https://example.test", `[]`); err == nil || !strings.Contains(err.Error(), "open Vueling in Chrome") {
		t.Fatalf("readPageWithDevTools error = %v; want navigation error", err)
	}
}

func newMockDevToolsServer(t *testing.T, failNavigation bool) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/version" {
			_ = json.NewEncoder(w).Encode(map[string]string{"webSocketDebuggerUrl": "ws://" + server.Listener.Addr().String() + "/devtools/browser/mock"})
			return
		}
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		for {
			payload, _, err := wsutil.ReadClientData(conn)
			if err != nil {
				return
			}
			var command struct {
				ID        int64  `json:"id"`
				Method    string `json:"method"`
				SessionID string `json:"sessionId"`
				Params    struct {
					Expression string `json:"expression"`
				} `json:"params"`
			}
			if err := json.Unmarshal(payload, &command); err != nil {
				return
			}
			result := any(map[string]any{})
			switch command.Method {
			case "Target.createTarget":
				result = map[string]string{"targetId": "mock-target"}
			case "Target.attachToTarget":
				result = map[string]string{"sessionId": "mock-session"}
			case "Runtime.evaluate":
				if command.Params.Expression == "self" {
					result = map[string]any{"result": map[string]string{"type": "object", "className": "Window"}}
				} else {
					result = map[string]any{"result": map[string]string{"type": "string", "value": `{"ok":true}`}}
				}
			case "Page.navigate":
				result = map[string]string{"frameId": "mock-target", "loaderId": "mock-loader"}
			}
			response := map[string]any{"id": command.ID, "result": result}
			if command.Method == "Page.navigate" && failNavigation {
				delete(response, "result")
				response["error"] = map[string]any{"code": -1, "message": "navigation failed"}
			}
			if command.SessionID != "" {
				response["sessionId"] = command.SessionID
			}
			encoded, _ := json.Marshal(response)
			if err := wsutil.WriteServerMessage(conn, ws.OpText, encoded); err != nil {
				return
			}
			if command.Method == "Page.navigate" {
				for _, event := range []map[string]any{
					{"sessionId": "mock-session", "method": "Page.lifecycleEvent", "params": map[string]any{"frameId": "mock-target", "loaderId": "mock-loader", "name": "init", "timestamp": 1.0}},
					{"sessionId": "mock-session", "method": "Page.loadEventFired", "params": map[string]any{"timestamp": 1.1}},
				} {
					data, _ := json.Marshal(event)
					if err := wsutil.WriteServerMessage(conn, ws.OpText, data); err != nil {
						return
					}
				}
			}
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestOpenURLStartsTheHostBrowser(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows requires a native rundll32 executable")
	}
	command := "xdg-open"
	if runtime.GOOS == "darwin" {
		command = "open"
	}
	installCommand(t, command, "exit 0\n")
	if err := OpenURL("https://example.test"); err != nil {
		t.Fatalf("OpenURL = %v", err)
	}
	t.Setenv("PATH", t.TempDir())
	if err := OpenURL("https://example.test"); err == nil {
		t.Fatal("OpenURL succeeded without a host browser command")
	}
}

func TestOpenCommandSelectsTheNativeBrowserCommand(t *testing.T) {
	for _, test := range []struct {
		goos string
		want []string
	}{
		{"darwin", []string{"open", "https://example.test"}},
		{"windows", []string{"rundll32", "url.dll,FileProtocolHandler", "https://example.test"}},
		{"linux", []string{"xdg-open", "https://example.test"}},
	} {
		cmd, err := openCommand(test.goos, "https://example.test")
		if err != nil || !reflect.DeepEqual(cmd.Args, test.want) {
			t.Errorf("openCommand(%q) = %v, %v; want %v", test.goos, cmd, err, test.want)
		}
	}
	if _, err := openCommand("plan9", "https://example.test"); err == nil || !strings.Contains(err.Error(), "plan9") {
		t.Fatalf("openCommand unsupported OS error = %v", err)
	}
}

func TestOpenURLRejectsUnsupportedHost(t *testing.T) {
	if err := openURL("plan9", "https://example.test"); err == nil || !strings.Contains(err.Error(), "plan9") {
		t.Fatalf("openURL unsupported OS error = %v", err)
	}
}

func TestValidateDevToolsEndpoint(t *testing.T) {
	for _, tc := range []struct {
		endpoint string
		ok       bool
	}{
		{"http://127.0.0.1:9222", true},
		{"http://localhost:9222", true},
		{"http://[::1]:9222", true},
		{"https://127.0.0.1:9222", false},
		{"http://192.168.1.10:9222", false},
		{"http://example.com:9222", false},
		{"http://user:pass@127.0.0.1:9222", false},
		{"http://", false},
		{"127.0.0.1:9222", false},
		{"", false},
	} {
		err := validateDevToolsEndpoint(tc.endpoint)
		if (err == nil) != tc.ok {
			t.Errorf("validateDevToolsEndpoint(%q) error = %v, want ok=%v", tc.endpoint, err, tc.ok)
		}
	}
}

func TestDevToolsEndpointUsesLocalDefault(t *testing.T) {
	if got := devToolsEndpoint(""); got != defaultDevToolsEndpoint {
		t.Fatalf("devToolsEndpoint empty = %q, want %q", got, defaultDevToolsEndpoint)
	}
	if got := devToolsEndpoint("http://localhost:9333"); got != "http://localhost:9333" {
		t.Fatalf("devToolsEndpoint configured = %q", got)
	}
}

func TestCheckDevToolsEndpointRejectsMalformedURL(t *testing.T) {
	if _, err := checkDevToolsEndpoint(context.Background(), "http://%"); err == nil {
		t.Fatal("malformed DevTools endpoint was accepted")
	}
}

func TestCheckDevToolsEndpointReportsUnavailableAndMalformedResponses(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	endpoint := closed.URL
	closed.Close()
	if _, err := checkDevToolsEndpoint(context.Background(), endpoint); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("checkDevToolsEndpoint error = %v, want unavailable endpoint", err)
	}
	for _, tc := range []struct{ body, want string }{{"not json", "decode Chrome DevTools"}, {`{}`, "invalid WebSocket"}} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, tc.body) }))
		_, err := checkDevToolsEndpoint(context.Background(), server.URL)
		server.Close()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("checkDevToolsEndpoint response %q error = %v", tc.body, err)
		}
	}
}

func TestCheckDevToolsEndpointReturnsTheValidatedWebSocketURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/version" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, `{"webSocketDebuggerUrl":"ws://`+r.Host+`/devtools/browser/test"}`)
	}))
	defer server.Close()

	got, err := checkDevToolsEndpoint(context.Background(), server.URL)
	want := "ws" + strings.TrimPrefix(server.URL, "http") + "/devtools/browser/test"
	if err != nil || got != want {
		t.Fatalf("checkDevToolsEndpoint = %q, %v; want %q", got, err, want)
	}
}

func TestCheckDevToolsEndpointRejectsRemoteWebSocketURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"webSocketDebuggerUrl":"ws://example.com:9222/devtools/browser/test"}`)
	}))
	defer server.Close()
	if _, err := checkDevToolsEndpoint(context.Background(), server.URL); err == nil || !strings.Contains(err.Error(), "non-local WebSocket") {
		t.Fatalf("checkDevToolsEndpoint error = %v; want remote WebSocket rejection", err)
	}
}

func TestCheckDevToolsEndpointDoesNotFollowRedirects(t *testing.T) {
	followed := false
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		followed = true
		w.WriteHeader(http.StatusOK)
	}))
	defer remote.Close()
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, remote.URL, http.StatusFound)
	}))
	defer local.Close()

	_, err := checkDevToolsEndpoint(context.Background(), local.URL)
	if err == nil || !strings.Contains(err.Error(), "HTTP 302") || followed {
		t.Fatalf("checkDevToolsEndpoint error = %v, remote redirect followed = %v; want local 302 and no follow", err, followed)
	}
}

type closeErrorBody struct {
	io.Reader
	err error
}

func (body closeErrorBody) Close() error { return body.err }

func TestReadDevToolsResponseReportsCloseErrors(t *testing.T) {
	closeErr := errors.New("response close failed")
	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable} {
		response := &http.Response{StatusCode: status, Body: closeErrorBody{Reader: strings.NewReader(""), err: closeErr}}
		_, err := readDevToolsResponse(response)
		if !errors.Is(err, closeErr) {
			t.Fatalf("checkDevToolsResponse error = %v; want response close error", err)
		}
		if status != http.StatusOK && !strings.Contains(err.Error(), "HTTP 503") {
			t.Fatalf("checkDevToolsResponse error = %v; want status and close errors", err)
		}
	}
}

func TestWaitForPageResultIncludesEvaluationErrorOnTimeout(t *testing.T) {
	evaluationErr := errors.New("page script failed")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := waitForPageResult(ctx, make(chan struct{}), time.Millisecond, func() (string, error) {
		return "", evaluationErr
	})
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, evaluationErr) {
		t.Fatalf("waitForPageResult error = %v, want timeout and page script errors", err)
	}
}

func TestWaitForPageResultClearsTransientEvaluationError(t *testing.T) {
	evaluationErr := errors.New("transient page script error")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	calls := 0
	_, err := waitForPageResult(ctx, make(chan struct{}), time.Millisecond, func() (string, error) {
		calls++
		if calls == 1 {
			return "", evaluationErr
		}
		return "[]", nil
	})
	if !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, evaluationErr) {
		t.Fatalf("waitForPageResult error = %v, want only timeout after successful polls", err)
	}
}

func TestWaitForPageResultReturnsResultOrTabClosure(t *testing.T) {
	got, err := waitForPageResult(context.Background(), make(chan struct{}), time.Hour, func() (string, error) {
		return `{"ok":true}`, nil
	})
	if err != nil || string(got) != `{"ok":true}` {
		t.Fatalf("waitForPageResult = %q, %v", got, err)
	}
	done := make(chan struct{})
	close(done)
	_, err = waitForPageResult(context.Background(), done, time.Hour, func() (string, error) { return "[]", nil })
	if err == nil || !strings.Contains(err.Error(), "tab closed") {
		t.Fatalf("waitForPageResult tab closure error = %v", err)
	}
	evaluationErr := errors.New("evaluate failed")
	_, err = waitForPageResult(context.Background(), done, time.Hour, func() (string, error) { return "", evaluationErr })
	if !errors.Is(err, evaluationErr) || !strings.Contains(err.Error(), "evaluate Vueling page script") {
		t.Fatalf("waitForPageResult evaluation error = %v", err)
	}
}

func TestWaitForPageResultDoesNotDuplicateContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := waitForPageResult(ctx, make(chan struct{}), time.Hour, func() (string, error) {
		return "", context.Canceled
	})
	if !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "last page evaluation") {
		t.Fatalf("waitForPageResult cancellation = %v", err)
	}
}

func TestPageClosedErrorPreservesContextAndEvaluationErrors(t *testing.T) {
	evaluationErr := errors.New("evaluate failed")
	err := pageClosedError(context.Canceled, evaluationErr)
	if !errors.Is(err, context.Canceled) || !errors.Is(err, evaluationErr) {
		t.Fatalf("pageClosedError = %v; want cancellation and evaluation causes", err)
	}
}

type readError struct{ err error }

func (r readError) Read([]byte) (int, error) { return 0, r.err }

func TestReadDevToolsResponseReportsReadError(t *testing.T) {
	readErr := errors.New("read failed")
	_, err := readDevToolsResponse(&http.Response{StatusCode: http.StatusOK, Body: closeErrorBody{Reader: readError{readErr}}})
	if !errors.Is(err, readErr) {
		t.Fatalf("readDevToolsResponse error = %v", err)
	}
}

func TestReadDevToolsResponseRejectsOversizedBodies(t *testing.T) {
	response := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", maxDevToolsBody+1)))}
	if _, err := readDevToolsResponse(response); err == nil || !strings.Contains(err.Error(), "response exceeds") {
		t.Fatalf("readDevToolsResponse oversized error = %v", err)
	}
}

func TestValidateDevToolsWebSocketURL(t *testing.T) {
	for _, tc := range []struct {
		name, rawURL, want string
	}{
		{"valid", "ws://127.0.0.1:9222/devtools/browser/id", ""},
		{"malformed", "://bad", "invalid WebSocket"},
		{"wrong scheme", "http://127.0.0.1:9222/devtools/browser/id", "invalid WebSocket"},
		{"credentials", "ws://user@127.0.0.1:9222/devtools/browser/id", "invalid WebSocket"},
		{"fragment", "ws://127.0.0.1:9222/devtools/browser/id#fragment", "invalid WebSocket"},
		{"wrong path", "ws://127.0.0.1:9222/devtools/page/id", "invalid WebSocket"},
		{"remote host", "ws://example.com:9222/devtools/browser/id", "non-local"},
		{"wrong port", "ws://127.0.0.1:9223/devtools/browser/id", "different port"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateDevToolsWebSocketURL("http://127.0.0.1:9222", tc.rawURL)
			if tc.want == "" {
				if err != nil || got != tc.rawURL {
					t.Fatalf("validateDevToolsWebSocketURL = %q, %v", got, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("validateDevToolsWebSocketURL error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestAppleEventsFailurePreservesCauses(t *testing.T) {
	cause := errors.New("osascript exited")
	for _, tc := range []struct {
		name   string
		output string
		want   string
	}{
		{"permission", "VUELING_JS_DISABLED", "blocks JavaScript"},
		{"no window", "VUELING_NO_CHROME_WINDOW", "no open window"},
		{"no offers", "VUELING_NO_OFFERS", "did not render results"},
		{"script", "VUELING_SCRIPT_ERROR: TypeError: invalid selector", "page script failed"},
		{"unknown", "osascript: unexpected failure", "could not read the Vueling tab"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := appleEventsFailure([]byte(tc.output), cause)
			if !errors.Is(err, cause) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("appleEventsFailure error = %v, want %q and osascript cause", err, tc.want)
			}
		})
	}
}

func TestAppleEventsFailureDoesNotExposePageScriptOutput(t *testing.T) {
	err := appleEventsFailure([]byte("VUELING_SCRIPT_ERROR: access_token=page-secret"), errors.New("osascript exited"))
	if strings.Contains(err.Error(), "page-secret") || strings.Contains(err.Error(), "access_token") {
		t.Fatalf("Apple Events error exposed page output: %v", err)
	}
}

func TestDevToolsUnavailableErrorDoesNotExposeEndpointQuery(t *testing.T) {
	endpoint := "http://127.0.0.1:1/?token=endpoint-secret"
	_, err := checkDevToolsEndpoint(context.Background(), endpoint)
	if err == nil || strings.Contains(err.Error(), "endpoint-secret") || strings.Contains(err.Error(), "127.0.0.1:1") {
		t.Fatalf("DevTools error exposed endpoint details: %v", err)
	}
}
