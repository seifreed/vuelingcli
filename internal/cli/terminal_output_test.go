package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewSanitizesTerminalControlsFromOutput(t *testing.T) {
	var output bytes.Buffer
	program := New("test", &output, &bytes.Buffer{}, nil)
	input := "flight \x1b[31mdelayed\x1b[0m \x1b]52;c;secret\a\nnext\tline\u009b31m"

	n, err := program.stdout.Write([]byte(input))
	if err != nil || n != len(input) {
		t.Fatalf("Write = %d, %v; want %d bytes and no error", n, err, len(input))
	}
	got := output.String()
	if strings.ContainsAny(got, "\x1b\a\u009b") || !strings.Contains(got, "\nnext\tline") {
		t.Fatalf("terminal output = %q; want controls removed and formatting preserved", got)
	}
}
