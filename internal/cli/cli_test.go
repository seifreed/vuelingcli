package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/seifreed/vuelingcli/internal/airports"
	"github.com/seifreed/vuelingcli/internal/vueling"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
	toon "github.com/toon-format/toon-go"
)

func TestConnectionSummary(t *testing.T) {
	tests := []struct {
		flight vueling.Flight
		want   string
	}{
		{vueling.Flight{Flights: []string{"VY1000"}}, ""},
		{vueling.Flight{Stops: 1, Flights: []string{"VY1383", "VY1050"}, Via: []string{"BCN"}}, "1 escala vía BCN · VY1383+VY1050 | "},
		{vueling.Flight{Stops: 1, Flights: []string{"VY1001"}}, "1 escala | "},
		{vueling.Flight{Stops: 2, Flights: []string{"VY1", "VY2", "VY3"}, Via: []string{"BCN", "FCO"}}, "2 escalas vía BCN, FCO · VY1+VY2+VY3 | "},
	}
	for _, test := range tests {
		if got := connectionSummary(test.flight); got != test.want {
			t.Errorf("connectionSummary(%+v) = %q, want %q", test.flight, got, test.want)
		}
	}
}

func TestRequireAPIReportsInitializationError(t *testing.T) {
	cause := errors.New("transport setup failed")
	err := (&CLI{}).WithAPIError(cause).requireAPI()
	if !errors.Is(err, cause) {
		t.Fatalf("requireAPI error = %v, want cause %v", err, cause)
	}
}

func TestOptionalCLIAdapters(t *testing.T) {
	cli := &CLI{}
	if err := cli.openLink("https://example.test"); err == nil {
		t.Fatal("openLink without an opener succeeded")
	}
	if _, err := cli.webPageSource(); err == nil {
		t.Fatal("webPageSource without a source succeeded")
	}
	if _, err := cli.pageReader(""); err == nil {
		t.Fatal("pageReader without a factory succeeded")
	}
	cli.newReader = func(string) web.PageReader { return nil }
	if _, err := cli.pageReader(""); err == nil {
		t.Fatal("pageReader accepted a nil reader")
	}
	reader := testPageReader{}
	cli.newReader = func(string) web.PageReader { return reader }
	if got, err := cli.pageReader(""); err != nil || got == nil {
		t.Fatalf("pageReader = %v, %v", got, err)
	}
	if got, err := cli.WithWebPageSource(web.Source{}).webPageSource(); err != nil || got == nil {
		t.Fatalf("webPageSource = %v, %v", got, err)
	}
	openErr := errors.New("could not open URL")
	if err := cli.WithLinkOpener(func(string) error { return openErr }).openLink("https://example.test"); !errors.Is(err, openErr) {
		t.Fatalf("openLink error = %v, want %v", err, openErr)
	}
}

func TestIsHelp(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"--help"}, true},
		{[]string{"-h"}, true},
		{[]string{"help"}, false},
		{[]string{"--help", "extra"}, false},
	} {
		if got := isHelp(tc.args); got != tc.want {
			t.Errorf("isHelp(%v) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestEncodeTOONUsesJSONFields(t *testing.T) {
	encoded, err := encodeTOON([]airports.Airport{{Code: "BCN", City: "Barcelona"}})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := toon.DecodeString(encoded)
	if err != nil {
		t.Fatalf("invalid TOON: %v", err)
	}
	rows, ok := decoded.([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("decoded TOON = %#v, want one airport row", decoded)
	}
	row, ok := rows[0].(map[string]any)
	if !ok || row["code"] != "BCN" || row["city"] != "Barcelona" {
		t.Fatalf("decoded TOON row = %#v, want JSON-tagged airport fields", rows[0])
	}
}

func TestStructuredOutputReportsEncodingAndWriterErrors(t *testing.T) {
	if _, err := encodeTOON(make(chan int)); err == nil {
		t.Fatal("encodeTOON accepted a channel")
	}
	for _, format := range []string{"json", "toon"} {
		cli := New("test", errorWriter{err: errors.New("write failed")}, nil, nil)
		if err := cli.writeStructured([]string{"x"}, format); err == nil {
			t.Errorf("writeStructured(%q) ignored a writer error", format)
		}
	}
	if err := New("test", new(bytes.Buffer), nil, nil).writeStructured(make(chan int), "toon"); err == nil {
		t.Fatal("writeStructured ignored a TOON encoding error")
	}
	cli := New("test", new(bytes.Buffer), nil, nil)
	if err := cli.writeStructured(nil, "yaml"); err == nil {
		t.Fatal("writeStructured accepted an unsupported format")
	}
}

func TestParseSearchTermsRejectsConflictingFormats(t *testing.T) {
	if _, _, err := parseSearchTerms([]string{"--json", "--toon"}); err == nil || !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("parseSearchTerms error = %v", err)
	}
}
