// Package api calls Vueling's public JSON APIs directly over HTTPS, without a browser.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	utls "github.com/refraction-networking/utls"
	"github.com/seifreed/vuelingcli/internal/observability"
)

const (
	// DefaultAPIW serves availability calendars and flight status.
	DefaultAPIW = "https://apiw.vueling.com"
	// DefaultAMS serves the anonymous session token and flight availability.
	DefaultAMS = "https://ams.vueling.com"
	// DefaultStatic serves the website's station and route network.
	DefaultStatic = "https://static.vueling.com"
	// DefaultWWW serves the website's own APIs, such as its alert banner.
	DefaultWWW = "https://www.vueling.com"

	wwwOrigin     = "https://www.vueling.com"
	ticketsOrigin = "https://tickets.vueling.com"
	maxBody       = 8 << 20
)

// Client calls Vueling's APIs with Chrome's TLS fingerprint and request headers.
type Client struct {
	HTTP   *http.Client
	Logger *slog.Logger
	APIW   string
	AMS    string
	Static string
	WWW    string
	// SessionFile keeps the anonymous session between runs; empty opens a new session every run.
	SessionFile string
}

// New returns a client that presents Chrome's TLS fingerprint to Vueling's CDN.
func New() (*Client, error) {
	return newClient(newChromeTransport)
}

func newClient(transportFactory func() (*http.Transport, error)) (*Client, error) {
	transport, err := transportFactory()
	if err != nil {
		return nil, err
	}
	return &Client{HTTP: &http.Client{
		Transport: transport, Timeout: 30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}, APIW: DefaultAPIW, AMS: DefaultAMS, Static: DefaultStatic, WWW: DefaultWWW, SessionFile: DefaultSessionFile()}, nil
}

// StatusError reports a non-2xx API response.
type StatusError struct {
	Status     int
	CDNBlocked bool
}

type operationError struct {
	operation string
	cause     error
}

func (e operationError) Error() string { return e.operation + " failed" }
func (e operationError) Unwrap() error { return e.cause }

// HTTPStatusCode exposes the response status without requiring callers to inspect error text.
func (e *StatusError) HTTPStatusCode() int { return e.Status }

func (e *StatusError) Error() string {
	if e.Status == http.StatusForbidden && e.CDNBlocked {
		return "Vueling's CDN blocked the request (HTTP 403); retry later or use --browser"
	}
	return fmt.Sprintf("Vueling API returned HTTP %d", e.Status)
}

func (c *Client) getJSON(ctx context.Context, rawURL, origin string, out any) error {
	return c.do(ctx, http.MethodGet, rawURL, origin, "", nil, out)
}

func (c *Client) postJSON(ctx context.Context, rawURL, origin, token string, body, out any) error {
	return c.do(ctx, http.MethodPost, rawURL, origin, token, body, out)
}

func (c *Client) do(ctx context.Context, method, rawURL, origin, token string, body, out any) (err error) {
	started := time.Now()
	statusCode, remoteHost := 0, ""
	observe := c.Logger != nil && c.Logger.Enabled(ctx, slog.LevelDebug)
	if observe {
		defer func() {
			attrs := []slog.Attr{slog.String("method", method)}
			if remoteHost != "" {
				attrs = append(attrs, slog.String("remote_host", remoteHost))
			}
			if statusCode != 0 {
				attrs = append(attrs, slog.Int("status_code", statusCode))
			}
			c.logOperation(ctx, slog.LevelDebug, "http.request.finished", "HTTP request completed", "http_request", started, err, attrs...)
		}()
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
	if err != nil {
		return operationError{operation: "create Vueling API request", cause: err}
	}
	remoteHost = req.URL.Hostname()
	setChromeHeaders(req.Header, origin)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if c.Logger != nil && c.Logger.Enabled(ctx, observability.TraceLevel) {
		c.Logger.LogAttrs(ctx, observability.TraceLevel, "HTTP request started",
			slog.String("event_name", "http.request.started"), slog.String("operation", "http_request"),
			slog.String("method", method), slog.String("remote_host", remoteHost))
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return operationError{operation: "call Vueling API", cause: err}
	}
	statusCode = resp.StatusCode
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			err = errors.Join(err, operationError{operation: "close Vueling API response", cause: closeErr})
		}
	}()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return operationError{operation: "read Vueling API response", cause: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		blocked := bytes.Contains(data[:min(len(data), 300)], []byte("Access Denied"))
		return &StatusError{Status: resp.StatusCode, CDNBlocked: blocked}
	}
	if len(data) > maxBody {
		return fmt.Errorf("Vueling API response exceeds %d bytes", maxBody)
	}
	if raw, ok := out.(*[]byte); ok {
		*raw = data
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode Vueling API response: %w", err)
	}
	return nil
}

func (c *Client) logOperation(ctx context.Context, level slog.Level, eventName, message, operation string, started time.Time, err error, attrs ...slog.Attr) {
	if c.Logger == nil {
		return
	}
	outcome := "success"
	switch {
	case errors.Is(err, context.Canceled):
		outcome = "cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		outcome = "timeout"
	case err != nil:
		outcome = "error"
	}
	fields := []slog.Attr{
		slog.String("event_name", eventName), slog.String("operation", operation),
		slog.Int64("duration_ms", time.Since(started).Milliseconds()), slog.String("outcome", outcome),
	}
	if err != nil {
		fields = append(fields, slog.String("error_type", apiErrorType(err)))
		switch {
		case errors.Is(err, context.Canceled):
			fields = append(fields, slog.String("error_code", "context_canceled"))
		case errors.Is(err, context.DeadlineExceeded):
			fields = append(fields, slog.String("error_code", "deadline_exceeded"))
		}
		var status interface{ HTTPStatusCode() int }
		if errors.As(err, &status) {
			fields = append(fields, slog.String("error_code", fmt.Sprintf("http_%d", status.HTTPStatusCode())))
		}
	}
	c.Logger.LogAttrs(ctx, level, message, append(fields, attrs...)...)
}

func apiErrorType(err error) string {
	var status interface{ HTTPStatusCode() int }
	if errors.As(err, &status) {
		return "http_status"
	}
	if errors.Is(err, context.Canceled) {
		return "context_canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	for {
		cause := errors.Unwrap(err)
		if cause == nil {
			return fmt.Sprintf("%T", err)
		}
		err = cause
	}
}

// setChromeHeaders sends what Chrome sends on a cross-site fetch; Akamai rejects requests without the client hints.
func setChromeHeaders(h http.Header, origin string) {
	version := utls.HelloChrome_Auto.Version
	h.Set("User-Agent", fmt.Sprintf("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s.0.0.0 Safari/537.36", version))
	h.Set("Accept", "application/json, text/plain, */*")
	h.Set("Accept-Language", "es-ES,es;q=0.9")
	h.Set("Origin", origin)
	h.Set("Referer", origin+"/")
	h.Set("Sec-Ch-Ua", fmt.Sprintf(`"Google Chrome";v="%s", "Not?A_Brand";v="8", "Chromium";v="%s"`, version, version))
	h.Set("Sec-Ch-Ua-Mobile", "?0")
	h.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	h.Set("Sec-Fetch-Site", "same-site")
	h.Set("Sec-Fetch-Mode", "cors")
	h.Set("Sec-Fetch-Dest", "empty")
}
