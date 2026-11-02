package web

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReadPageWrapsDeadline(t *testing.T) {
	reader := &testPageReader{err: context.DeadlineExceeded}
	_, err := readPage(context.Background(), reader, "https://example.test", "script", time.Second, "page timed out")
	if !errors.Is(err, context.DeadlineExceeded) || err.Error() != "page timed out: context deadline exceeded" {
		t.Fatalf("readPage error = %v, want wrapped deadline error", err)
	}
}
