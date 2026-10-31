// Package browser reads rendered pages from the user's local browser session.
package browser

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// ChromeReader reads rendered pages from the user's local Chrome session.
type ChromeReader struct {
	endpoint string
}

// NewChromeReader configures Chrome access with an optional local DevTools endpoint.
func NewChromeReader(endpoint string) ChromeReader {
	return ChromeReader{endpoint: endpoint}
}

// ReadPage navigates to a page and waits for the script to return a result.
func (reader ChromeReader) ReadPage(ctx context.Context, pageURL, script string) ([]byte, error) {
	if reader.endpoint == "" && runtime.GOOS == "darwin" {
		return readPageWithAppleEvents(ctx, pageURL, script)
	}
	return readPageWithDevTools(ctx, reader.endpoint, pageURL, script)
}

func readPageWithAppleEvents(ctx context.Context, pageURL, pageScript string) ([]byte, error) {
	const script = `on run argv
	set searchURL to item 1 of argv
	set pageScript to item 2 of argv
	set lastError to ""
	tell application "Google Chrome"
		if (count of windows) is 0 then error "VUELING_NO_CHROME_WINDOW"
		set resultTab to make new tab at end of tabs of front window with properties {URL:searchURL}
		repeat 90 times
			try
				set resultJSON to execute resultTab javascript ("JSON.stringify(" & pageScript & ")")
				if resultJSON is not missing value and resultJSON is not "[]" then
					close resultTab
					return resultJSON
				end if
				set lastError to ""
			on error errorText
				if errorText contains "Executing JavaScript through AppleScript is turned off" then error "VUELING_JS_DISABLED"
				set lastError to errorText
			end try
			delay 0.5
		end repeat
		if lastError is not "" then error "VUELING_SCRIPT_ERROR:" & lastError
		error "VUELING_NO_OFFERS"
	end tell
end run`
	cmd := exec.CommandContext(ctx, "osascript", "-e", script, pageURL, pageScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, appleEventsFailure(output, err)
	}
	return []byte(strings.TrimSpace(string(output))), nil
}

func appleEventsFailure(output []byte, cause error) error {
	message := strings.ToLower(string(output))
	switch {
	case strings.Contains(message, "vueling_js_disabled"):
		return fmt.Errorf("Chrome blocks JavaScript from Apple Events; enable View > Developer > Allow JavaScript from Apple Events, then retry: %w", cause)
	case strings.Contains(message, "vueling_no_chrome_window"):
		return fmt.Errorf("Google Chrome has no open window: %w", cause)
	case strings.Contains(message, "vueling_no_offers"):
		return fmt.Errorf("Vueling did not render results; the tab was left open in Chrome so you can check it, and a manual search there shows whether Vueling is unavailable or the session needs attention. Retry later rather than in a loop: %w", cause)
	case strings.Contains(string(output), "VUELING_SCRIPT_ERROR:"):
		index := strings.Index(string(output), "VUELING_SCRIPT_ERROR:") + len("VUELING_SCRIPT_ERROR:")
		return fmt.Errorf("Vueling page script failed: %s: %w", strings.TrimSpace(string(output[index:])), cause)
	default:
		detail := strings.TrimSpace(string(output))
		if detail != "" {
			return fmt.Errorf("could not read the Vueling tab through Chrome Apple Events: %w: %s", cause, detail)
		}
		return fmt.Errorf("could not read the Vueling tab through Chrome Apple Events: %w", cause)
	}
}

func readPageWithDevTools(ctx context.Context, endpoint, pageURL, pageScript string) ([]byte, error) {
	if endpoint == "" {
		endpoint = "http://127.0.0.1:9222"
	}
	if err := validateDevToolsEndpoint(endpoint); err != nil {
		return nil, err
	}
	if err := checkDevToolsEndpoint(ctx, endpoint); err != nil {
		return nil, err
	}
	allocator, stopAllocator := chromedp.NewRemoteAllocator(ctx, endpoint)
	defer stopAllocator()
	tab, stopTab := chromedp.NewContext(allocator)
	defer stopTab()
	if err := chromedp.Run(tab, chromedp.Navigate(pageURL)); err != nil {
		return nil, fmt.Errorf("open Vueling in Chrome: %w", err)
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
	pageError := func(ctxErr error) error {
		if lastEvaluationErr == nil || errors.Is(lastEvaluationErr, ctxErr) {
			return ctxErr
		}
		return errors.Join(ctxErr, fmt.Errorf("last page evaluation failed: %w", lastEvaluationErr))
	}
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
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, pageError(ctxErr)
		}
		select {
		case <-ctx.Done():
			return nil, pageError(ctx.Err())
		case <-done:
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, pageError(ctxErr)
			}
			if lastEvaluationErr != nil {
				return nil, fmt.Errorf("evaluate Vueling page script: %w", lastEvaluationErr)
			}
			return nil, fmt.Errorf("Vueling browser tab closed before rendering results")
		case <-ticker.C:
		}
	}
}

func checkDevToolsEndpoint(ctx context.Context, endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("Chrome DevTools endpoint is invalid: %w", err)
	}
	u.Path, u.RawQuery = "/json/version", ""
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("Chrome DevTools endpoint is invalid: %w", err)
	}
	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("Chrome DevTools is unavailable at %s; enable local remote debugging for the Chrome profile holding Vueling, then retry (%w)", endpoint, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Chrome DevTools returned HTTP %d at %s", response.StatusCode, endpoint)
	}
	return nil
}

func validateDevToolsEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "http" || u.Hostname() == "" || u.User != nil {
		return fmt.Errorf("Chrome DevTools endpoint must be a local http URL, such as http://127.0.0.1:9222")
	}
	if u.Hostname() != "localhost" && !net.ParseIP(u.Hostname()).IsLoopback() {
		return fmt.Errorf("Chrome DevTools endpoint must use localhost or a loopback IP")
	}
	return nil
}
