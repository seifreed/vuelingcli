package browser

import (
	"context"
	"errors"
	"testing"
	"time"
)

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

func TestCheckDevToolsEndpointRejectsMalformedURL(t *testing.T) {
	if err := checkDevToolsEndpoint(context.Background(), "http://%"); err == nil {
		t.Fatal("malformed DevTools endpoint was accepted")
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
