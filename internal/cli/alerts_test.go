package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/seifreed/vuelingcli/internal/vueling/api"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

func alertsCLI(t *testing.T, body string) (*CLI, *bytes.Buffer) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
	t.Cleanup(server.Close)
	stdout := new(bytes.Buffer)
	cli := New("test", stdout, new(bytes.Buffer), func(string) web.PageReader { return testPageReader{} })
	return cli.WithAPI(&api.Client{HTTP: server.Client(), WWW: server.URL}), stdout
}

func TestAlertsPrintsTheNoticeOrItsAbsence(t *testing.T) {
	cli, stdout := alertsCLI(t, `{"pae":true,"text":"Huelga en Francia","url":"https://www.vueling.com/es/huelga"}`)
	if err := cli.alerts(nil); err != nil || stdout.String() != "Huelga en Francia\nhttps://www.vueling.com/es/huelga\n" {
		t.Fatalf("alerts = %v, %q", err, stdout)
	}
	cli, stdout = alertsCLI(t, `{"pae":false,"text":"","url":""}`)
	if err := cli.alerts([]string{"--culture", "en-GB"}); err != nil || stdout.String() != "No active Vueling disruption notice\n" {
		t.Fatalf("alerts = %v, %q", err, stdout)
	}
	stdout.Reset()
	if err := cli.alerts([]string{"--json"}); err != nil || stdout.String() != "{\"active\":false}\n" {
		t.Fatalf("alerts --json = %v, %q", err, stdout)
	}
	for _, args := range [][]string{{"--culture", "es&x=1"}, {"extra"}, {"--nope", "1"}} {
		if err := cli.alerts(args); err == nil || !strings.Contains(err.Error(), "culture") && !strings.Contains(err.Error(), "usage") {
			t.Errorf("alerts %v error = %v", args, err)
		}
	}
}
