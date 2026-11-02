package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type authCalls struct {
	opened, extended, deleted int
	deleteStatus              int
}

func sessionServer(t *testing.T, extendStatus int) (*Client, *authCalls) {
	t.Helper()
	calls := &authCalls{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /asm/v1/Auth", func(w http.ResponseWriter, _ *http.Request) {
		calls.opened++
		_, _ = fmt.Fprintf(w, `{"accessToken":%q,"expiration":1200}`, fmt.Sprintf("new-%d", calls.opened))
	})
	mux.HandleFunc("PUT /asm/v1/Auth", func(w http.ResponseWriter, r *http.Request) {
		calls.extended++
		if extendStatus != http.StatusOK || r.Header.Get("Authorization") != "Bearer old" {
			http.Error(w, "expired", extendStatus)
			return
		}
		_, _ = w.Write([]byte(`{"expires":1200}`))
	})
	mux.HandleFunc("DELETE /asm/v1/Auth", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer old" {
			calls.deleted++
		}
		if calls.deleteStatus != 0 {
			w.WriteHeader(calls.deleteStatus)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	file := filepath.Join(t.TempDir(), "vuelingcli", "session.json")
	return &Client{HTTP: server.Client(), AMS: server.URL, SessionFile: file}, calls
}

func storeTestSession(t *testing.T, c *Client, left time.Duration) {
	t.Helper()
	if err := c.storeSession(context.Background(), Session{Token: "old", Expires: time.Now().Add(left)}); err != nil {
		t.Fatal(err)
	}
}

func TestAnonymousTokenReusesExtendsOrReplacesTheStoredSession(t *testing.T) {
	for _, tc := range []struct {
		name         string
		left         time.Duration
		extendStatus int
		want         string
		calls        authCalls
	}{
		{"fresh", 10 * time.Minute, http.StatusOK, "old", authCalls{}},
		{"near expiry", time.Minute, http.StatusOK, "old", authCalls{extended: 1}},
		{"extend refused", time.Minute, http.StatusUnauthorized, "new-1", authCalls{opened: 1, extended: 1}},
		{"expired", -time.Minute, http.StatusOK, "new-1", authCalls{opened: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, calls := sessionServer(t, tc.extendStatus)
			storeTestSession(t, c, tc.left)
			token, err := c.anonymousToken(context.Background())
			if err != nil || token != tc.want || *calls != tc.calls {
				t.Fatalf("anonymousToken = %q, %v, calls %+v; want %q, %+v", token, err, *calls, tc.want, tc.calls)
			}
			stored, err := c.StoredSession()
			if err != nil || stored.Token != tc.want || !stored.Active(time.Now().Add(5*time.Minute)) {
				t.Fatalf("stored session = %+v, %v", stored, err)
			}
		})
	}
}

func TestSessionFileIsPrivate(t *testing.T) {
	c, _ := sessionServer(t, http.StatusOK)
	if _, err := c.anonymousToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" { // Windows uses inherited ACLs, not POSIX permission bits.
		for path, want := range map[string]os.FileMode{c.SessionFile: 0o600, filepath.Dir(c.SessionFile): 0o700} {
			if info, err := os.Stat(path); err != nil || info.Mode().Perm() != want {
				t.Errorf("%s mode = %v, %v; want %v", path, info.Mode().Perm(), err, want)
			}
		}
	}
}

func TestStoreSessionRemovesTemporaryFileWhenRenameFails(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "session")
	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	c := &Client{SessionFile: destination}
	if err := c.storeSession(context.Background(), Session{Token: "new"}); err == nil {
		t.Fatal("storeSession replaced a directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "session" {
		t.Fatalf("temporary session files left behind: %+v", entries)
	}
}

func TestSessionCacheReportsUnavailableDirectories(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "cache-blocker")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	client := &Client{SessionFile: filepath.Join(blocker, "session.json")}
	if err := client.storeSession(context.Background(), Session{Token: "new"}); err == nil {
		t.Fatal("storeSession wrote into a cache path blocked by a file")
	}

	t.Setenv("HOME", "")
	t.Setenv("LocalAppData", "")
	if got := DefaultSessionFile(); got != "" {
		t.Fatalf("DefaultSessionFile without a cache directory = %q, want empty", got)
	}
}

func TestStoreSessionReportsJSONAndTemporaryWriteFailures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := storeSessionFile(path, Session{Expires: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}, os.CreateTemp); err == nil {
		t.Fatal("storeSessionFile encoded a time outside JSON's supported range")
	}

	dir := t.TempDir()
	createUnremovable := func(dir, pattern string) (*os.File, error) {
		created, err := os.CreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		name := created.Name()
		if err := created.Close(); err != nil {
			return nil, err
		}
		if err := os.Remove(name); err != nil {
			return nil, err
		}
		if err := os.Mkdir(name, 0o700); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(name, "keep"), nil, 0o600); err != nil {
			return nil, err
		}
		return created, nil
	}
	err := storeSessionFile(filepath.Join(dir, "session.json"), Session{Token: "new"}, createUnremovable)
	if err == nil || !strings.Contains(err.Error(), "remove the temporary Vueling session file") {
		t.Fatalf("storeSessionFile write/cleanup error = %v", err)
	}
}

func TestAnonymousTokenReturnsStoredSessionReadError(t *testing.T) {
	c, calls := sessionServer(t, http.StatusOK)
	c.SessionFile = t.TempDir()
	if token, err := c.anonymousToken(context.Background()); err == nil || token != "" || calls.opened != 0 {
		t.Fatalf("anonymousToken = %q, %v, calls %+v", token, err, *calls)
	}
	if _, err := c.RefreshSession(context.Background()); err == nil {
		t.Fatal("RefreshSession stored nowhere without an error")
	}
	c.SessionFile = ""
	if _, err := c.RefreshSession(context.Background()); err == nil {
		t.Fatal("RefreshSession without a cache file succeeded")
	}
	if err := c.ClearSession(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestWithSessionReturnsAnUnwritableCacheErrorAfterReplacingRejectedToken(t *testing.T) {
	c, calls := sessionServer(t, http.StatusOK)
	storeTestSession(t, c, 10*time.Minute)
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var callCount int
	_, err := withSession(context.Background(), c, func(token string) (string, error) {
		callCount++
		if token == "old" {
			c.SessionFile = filepath.Join(blocker, "session.json")
			return "", &StatusError{Status: http.StatusUnauthorized}
		}
		return "unexpected retry", nil
	})
	if err == nil || callCount != 1 || calls.opened != 1 {
		t.Fatalf("withSession error = %v, calls = %d, sessions = %+v; want storage error before retry", err, callCount, *calls)
	}
}

func TestStoredSessionReportsACorruptFile(t *testing.T) {
	c, calls := sessionServer(t, http.StatusOK)
	if err := os.MkdirAll(filepath.Dir(c.SessionFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.SessionFile, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if session, err := c.StoredSession(); err == nil || session != (Session{}) {
		t.Fatalf("StoredSession = %+v, %v; want a decode error", session, err)
	}
	if token, err := c.anonymousToken(context.Background()); err == nil || token != "" || calls.opened != 0 {
		t.Fatalf("anonymousToken = %q, %v, sessions %+v; want the stored-session error", token, err, *calls)
	}
	if err := c.ClearSession(context.Background()); err == nil {
		t.Fatal("ClearSession hid the corrupt stored-session error")
	}
	if _, err := os.Stat(c.SessionFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("corrupt session file still exists: %v", err)
	}
}

func TestRefreshSessionExtendsAFreshSession(t *testing.T) {
	c, calls := sessionServer(t, http.StatusOK)
	storeTestSession(t, c, 10*time.Minute)
	session, err := c.RefreshSession(context.Background())
	if err != nil || session.Token != "old" || calls.extended != 1 || time.Until(session.Expires) < 19*time.Minute {
		t.Fatalf("RefreshSession = %+v, %v, calls %+v", session, err, *calls)
	}
}

func TestSessionValidityRejectsInvalidValues(t *testing.T) {
	for _, seconds := range []int64{-1, maxSessionValiditySeconds + 1} {
		if _, err := sessionValidity(seconds); err == nil {
			t.Errorf("sessionValidity(%d) accepted an invalid duration", seconds)
		}
	}
	if got, err := sessionValidity(0); err != nil || got != 0 {
		t.Fatalf("sessionValidity(0) = %s, %v; want zero", got, err)
	}
	if got, err := sessionValidity(1200); err != nil || got != 20*time.Minute {
		t.Fatalf("sessionValidity(1200) = %s, %v; want 20m", got, err)
	}
}

func TestSessionEndpointsRejectInvalidDurations(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /asm/v1/Auth", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"accessToken":"new","expiration":9223372037}`))
	})
	mux.HandleFunc("PUT /asm/v1/Auth", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"expires":-1}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client := &Client{HTTP: server.Client(), AMS: server.URL}
	if _, err := client.openSession(context.Background()); err == nil || !strings.Contains(err.Error(), "invalid session duration") {
		t.Fatalf("openSession invalid duration error = %v", err)
	}
	if _, err := client.extendSession(context.Background(), Session{Token: "old"}); err == nil || !strings.Contains(err.Error(), "invalid session duration") {
		t.Fatalf("extendSession invalid duration error = %v", err)
	}
}

func TestRefreshSessionReportsExtensionAndReplacementFailures(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /asm/v1/Auth", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("POST /asm/v1/Auth", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	c := &Client{HTTP: server.Client(), AMS: server.URL, SessionFile: filepath.Join(t.TempDir(), "session.json")}
	storeTestSession(t, c, time.Minute)

	if _, err := c.RefreshSession(context.Background()); err == nil || !strings.Contains(err.Error(), "HTTP 500") || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("RefreshSession error = %v; want both extension and replacement errors", err)
	}
}

func TestClearSessionEndsAndForgetsIt(t *testing.T) {
	c, calls := sessionServer(t, http.StatusOK)
	storeTestSession(t, c, 10*time.Minute)
	if err := c.ClearSession(context.Background()); err != nil || calls.deleted != 1 {
		t.Fatalf("ClearSession = %v, calls %+v", err, *calls)
	}
	if _, err := os.Stat(c.SessionFile); !os.IsNotExist(err) {
		t.Fatalf("session file still exists: %v", err)
	}
	if err := c.ClearSession(context.Background()); err != nil {
		t.Fatalf("second ClearSession = %v", err)
	}
}

func TestClearSessionReportsRemoteFailureAndStillForgetsIt(t *testing.T) {
	c, calls := sessionServer(t, http.StatusOK)
	calls.deleteStatus = http.StatusInternalServerError
	storeTestSession(t, c, 10*time.Minute)

	err := c.ClearSession(context.Background())
	var status *StatusError
	if !errors.As(err, &status) || status.Status != http.StatusInternalServerError || calls.deleted != 1 {
		t.Fatalf("ClearSession = %v, calls %+v; want the remote status error", err, *calls)
	}
	if _, err := os.Stat(c.SessionFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("session file still exists after remote failure: %v", err)
	}
}

func TestClearSessionReportsFailureToRemoveStoredDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keep"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := (&Client{SessionFile: dir}).ClearSession(context.Background())
	if err == nil || !strings.Contains(err.Error(), "forget the Vueling session") {
		t.Fatalf("ClearSession removal error = %v", err)
	}
}

func TestWithSessionReturnsNonAuthenticationErrorsAndReplacementFailures(t *testing.T) {
	client, _ := sessionServer(t, http.StatusOK)
	storeTestSession(t, client, 10*time.Minute)
	callErr := errors.New("availability failed")
	if _, err := withSession(context.Background(), client, func(string) (string, error) { return "", callErr }); !errors.Is(err, callErr) {
		t.Fatalf("withSession non-authentication error = %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /asm/v1/Auth", func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "blocked", http.StatusBadGateway) })
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client = &Client{HTTP: server.Client(), AMS: server.URL, SessionFile: filepath.Join(t.TempDir(), "session.json")}
	storeTestSession(t, client, 10*time.Minute)
	_, err := withSession(context.Background(), client, func(string) (string, error) {
		return "", &StatusError{Status: http.StatusUnauthorized}
	})
	if err == nil || !strings.Contains(err.Error(), "HTTP 502") {
		t.Fatalf("withSession replacement error = %v", err)
	}
}

func TestAnonymousTokenReportsSessionSaveFailure(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "cache-blocker")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	client, _ := sessionServer(t, http.StatusOK)
	client.SessionFile = filepath.Join(blocker, "session.json")
	if _, err := client.anonymousToken(context.Background()); err == nil {
		t.Fatal("anonymousToken succeeded without being able to save its session")
	}
}

func TestSessionEndIsCappedByTheTokenExpiry(t *testing.T) {
	exp := time.Now().Add(5 * time.Minute).Truncate(time.Second)
	payload, _ := json.Marshal(map[string]int64{"exp": exp.Unix()})
	token := "h." + base64.RawURLEncoding.EncodeToString(payload) + ".s"
	session := Session{Token: token, TokenExpires: tokenExpiry(token)}
	if got := session.ExpiresAt(time.Now(), 20*time.Minute); !got.Equal(exp) {
		t.Fatalf("end = %v; want %v", got, exp)
	}
	for _, bad := range []string{"jwt", "a.!!.c", "a." + base64.RawURLEncoding.EncodeToString([]byte("{}")) + ".c"} {
		if got := tokenExpiry(bad); !got.IsZero() {
			t.Errorf("tokenExpiry(%q) = %v", bad, got)
		}
	}
}

func TestWithSessionRetriesOnceWithANewSessionWhenTheTokenIsRejected(t *testing.T) {
	c, calls := sessionServer(t, http.StatusOK)
	storeTestSession(t, c, 10*time.Minute)
	var tokens []string
	got, err := withSession(context.Background(), c, func(token string) (string, error) {
		tokens = append(tokens, token)
		if token == "old" {
			return "", &StatusError{Status: http.StatusUnauthorized}
		}
		return "ok", nil
	})
	stored, storedErr := c.StoredSession()
	if err != nil || storedErr != nil || got != "ok" || fmt.Sprint(tokens) != "[old new-1]" || calls.opened != 1 || stored.Token != "new-1" {
		t.Fatalf("withSession = %q, %v, stored = %+v (%v), tokens %v, calls %+v", got, err, stored, storedErr, tokens, *calls)
	}
}
