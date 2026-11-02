package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

const maxDevToolsBody = 1 << 20
const defaultDevToolsEndpoint = "http://127.0.0.1:9222"

type browserOperationError struct {
	message string
	cause   error
}

func (e browserOperationError) Error() string { return e.message }
func (e browserOperationError) Unwrap() error { return e.cause }

func devToolsEndpoint(endpoint string) string {
	if endpoint == "" {
		return defaultDevToolsEndpoint
	}
	return endpoint
}

func readPageWithDevTools(ctx context.Context, endpoint, pageURL, pageScript string) ([]byte, error) {
	websocketURL, err := checkDevToolsEndpoint(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	allocator, stopAllocator := chromedp.NewRemoteAllocator(ctx, websocketURL, chromedp.NoModifyURL)
	defer stopAllocator()
	tab, stopTab := chromedp.NewContext(allocator)
	defer stopTab()
	if err := chromedp.Run(tab, chromedp.Navigate(pageURL)); err != nil {
		return nil, browserOperationError{message: "open Vueling in Chrome failed", cause: err}
	}
	return waitForPageResult(ctx, tab.Done(), 500*time.Millisecond, func() (string, error) {
		var result string
		err := chromedp.Run(tab, chromedp.Evaluate("JSON.stringify("+pageScript+")", &result))
		return result, err
	})
}

func waitForPageResult(ctx context.Context, done <-chan struct{}, interval time.Duration, evaluate func() (string, error)) ([]byte, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var lastEvaluationErr error
	for {
		result, err := evaluate()
		if err != nil {
			lastEvaluationErr = err
		} else {
			lastEvaluationErr = nil
		}
		if err == nil && result != "" && result != "[]" {
			return []byte(result), nil
		}
		select {
		case <-ctx.Done():
			return nil, pageContextError(ctx.Err(), lastEvaluationErr)
		case <-done:
			return nil, pageClosedError(ctx.Err(), lastEvaluationErr)
		case <-ticker.C:
		}
	}
}

func pageContextError(ctxErr, evaluationErr error) error {
	if evaluationErr == nil || errors.Is(evaluationErr, ctxErr) {
		return ctxErr
	}
	return errors.Join(ctxErr, browserOperationError{message: "last page evaluation failed", cause: evaluationErr})
}

func pageClosedError(ctxErr, evaluationErr error) error {
	if ctxErr != nil {
		return pageContextError(ctxErr, evaluationErr)
	}
	if evaluationErr != nil {
		return browserOperationError{message: "evaluate Vueling page script failed", cause: evaluationErr}
	}
	return errors.New("Vueling browser tab closed before rendering results")
}

func checkDevToolsEndpoint(ctx context.Context, endpoint string) (string, error) {
	u, err := parseDevToolsEndpoint(endpoint)
	if err != nil {
		return "", err
	}
	u.Path, u.RawPath, u.RawQuery, u.Fragment = "/json/version", "", "", ""
	request := (&http.Request{Method: http.MethodGet, URL: u, Header: make(http.Header)}).WithContext(ctx)
	client := http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(request)
	if err != nil {
		return "", browserOperationError{message: "Chrome DevTools is unavailable; enable local remote debugging for the Chrome profile holding Vueling, then retry", cause: err}
	}
	data, err := readDevToolsResponse(response)
	if err != nil {
		return "", err
	}
	var version struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(data, &version); err != nil {
		return "", fmt.Errorf("decode Chrome DevTools version response: %w", err)
	}
	websocketURL, err := validateDevToolsWebSocketURL(endpoint, version.WebSocketDebuggerURL)
	if err != nil {
		return "", err
	}
	return websocketURL, nil
}

func readDevToolsResponse(response *http.Response) ([]byte, error) {
	var err error
	if response.StatusCode != http.StatusOK {
		err = fmt.Errorf("Chrome DevTools returned HTTP %d", response.StatusCode)
	}
	var data []byte
	if err == nil {
		data, err = io.ReadAll(io.LimitReader(response.Body, maxDevToolsBody+1))
		if err != nil {
			err = browserOperationError{message: "read Chrome DevTools response failed", cause: err}
		}
		if err == nil && len(data) > maxDevToolsBody {
			err = fmt.Errorf("Chrome DevTools response exceeds %d bytes", maxDevToolsBody)
		}
	}
	if closeErr := response.Body.Close(); closeErr != nil {
		err = errors.Join(err, browserOperationError{message: "close Chrome DevTools response failed", cause: closeErr})
	}
	return data, err
}

func validateDevToolsWebSocketURL(endpoint, rawURL string) (string, error) {
	endpointURL, _ := url.Parse(endpoint)
	websocketURL, err := url.Parse(rawURL)
	if err != nil || websocketURL.Scheme != "ws" || websocketURL.User != nil || websocketURL.Fragment != "" || !strings.HasPrefix(websocketURL.Path, "/devtools/browser/") {
		return "", errors.New("Chrome DevTools returned an invalid WebSocket endpoint")
	}
	host := websocketURL.Hostname()
	if !strings.EqualFold(host, "localhost") && !net.ParseIP(host).IsLoopback() {
		return "", errors.New("Chrome DevTools returned a non-local WebSocket endpoint")
	}
	if websocketURL.Port() != endpointURL.Port() {
		return "", errors.New("Chrome DevTools returned a WebSocket endpoint on a different port")
	}
	return websocketURL.String(), nil
}

func validateDevToolsEndpoint(endpoint string) error {
	_, err := parseDevToolsEndpoint(endpoint)
	return err
}

func parseDevToolsEndpoint(endpoint string) (*url.URL, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "http" || u.Hostname() == "" || u.User != nil {
		return nil, fmt.Errorf("Chrome DevTools endpoint must be a local http URL, such as http://127.0.0.1:9222")
	}
	if u.Hostname() != "localhost" && !net.ParseIP(u.Hostname()).IsLoopback() {
		return nil, fmt.Errorf("Chrome DevTools endpoint must use localhost or a loopback IP")
	}
	return u, nil
}
