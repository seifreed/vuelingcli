package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seifreed/vuelingcli/internal/vueling/api"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

func sessionCLI(t *testing.T) (*CLI, *bytes.Buffer, string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /asm/v1/Auth", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"accessToken":"jwt","expiration":1200}`))
	})
	mux.HandleFunc("PUT /asm/v1/Auth", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"expires":1200}`)) })
	mux.HandleFunc("DELETE /asm/v1/Auth", func(http.ResponseWriter, *http.Request) {})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	file := filepath.Join(t.TempDir(), "session.json")
	stdout := new(bytes.Buffer)
	cli := New("test", stdout, new(bytes.Buffer), func(string) web.PageReader { return testPageReader{} })
	return cli.WithAPI(&api.Client{HTTP: server.Client(), AMS: server.URL, SessionFile: file}), stdout, file
}

func TestSessionShowsRefreshesAndClears(t *testing.T) {
	cli, stdout, file := sessionCLI(t)
	if code := cli.Run([]string{"session"}); code != 0 || stdout.String() != "No active anonymous Vueling session\n" {
		t.Fatalf("session = %d, %q", code, stdout)
	}
	stdout.Reset()
	if err := cli.session([]string{"refresh"}); err != nil || !strings.HasPrefix(stdout.String(), "Anonymous Vueling session active until ") || !strings.Contains(stdout.String(), "(19 min left)") {
		t.Fatalf("session refresh = %v, %q", err, stdout)
	}
	stdout.Reset()
	if err := cli.session([]string{"--json"}); err != nil {
		t.Fatal(err)
	}
	var status sessionStatus
	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil || !status.Active || status.MinutesLeft != 19 || status.ExpiresAt == "" {
		t.Fatalf("session --json = %v, %q", err, stdout)
	}
	stdout.Reset()
	if err := cli.session([]string{"clear"}); err != nil || stdout.String() != "Anonymous Vueling session cleared\n" {
		t.Fatalf("session clear = %v, %q", err, stdout)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("session file after clear: %v", err)
	}
	stdout.Reset()
	if err := cli.session([]string{"--toon"}); err != nil || !strings.Contains(stdout.String(), "active: false") {
		t.Fatalf("session --toon = %v, %q", err, stdout)
	}
	if err := cli.session([]string{"clear", "--json"}); err == nil {
		t.Fatal("session clear --json succeeded")
	}
	stdout.Reset()
	if err := cli.session([]string{"--help"}); err != nil || !strings.Contains(stdout.String(), "vueling session refresh") {
		t.Fatalf("session --help = %v, %q", err, stdout)
	}
}
