package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRunSanitizesControlCharactersInErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	apiErr := errors.New("server response: \x1b]52;c;Y2xpcGJvYXJk\a")
	cli := New("test", &stdout, &stderr, nil).WithAPIError(apiErr)
	if code := cli.Run([]string{"schedule", "BCN", "BIO"}); code != 2 {
		t.Fatalf("Run exit code = %d, want 2", code)
	}
	if strings.ContainsAny(stderr.String(), "\x1b\a") || !strings.Contains(stderr.String(), "Y2xpcGJvYXJk") {
		t.Fatalf("Run stderr = %q; want visible message without terminal controls", stderr.String())
	}
}
