// Package api calls Vueling's public JSON APIs directly over HTTPS, without a browser.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultAPIW serves availability calendars and flight status.
	DefaultAPIW = "https://apiw.vueling.com"
	// DefaultAMS serves the anonymous session token and flight availability.
	DefaultAMS = "https://ams.vueling.com"

	wwwOrigin     = "https://www.vueling.com"
	ticketsOrigin = "https://tickets.vueling.com"
	userAgent     = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36"
	maxBody       = 8 << 20
)

// Client calls Vueling's APIs with Chrome's TLS fingerprint and request headers.
type Client struct {
	HTTP *http.Client
	APIW string
	AMS  string
}

// New returns a client that presents Chrome's TLS fingerprint to Vueling's CDN.
func New() (*Client, error) {
	transport, err := newChromeTransport()
	if err != nil {
		return nil, err
	}
	return &Client{HTTP: &http.Client{Transport: transport, Timeout: 30 * time.Second}, APIW: DefaultAPIW, AMS: DefaultAMS}, nil
}

// StatusError reports a non-2xx API response.
type StatusError struct {
	URL    string
	Status int
	Body   string
}

func (e *StatusError) Error() string {
	if e.Status == http.StatusForbidden && strings.Contains(e.Body, "Access Denied") {
		return fmt.Sprintf("Vueling's CDN blocked the request to %s (HTTP 403); retry later or use --browser", e.URL)
	}
	return fmt.Sprintf("Vueling API %s returned HTTP %d: %s", e.URL, e.Status, strings.TrimSpace(e.Body))
}

func (c *Client) getJSON(ctx context.Context, rawURL, origin string, out any) error {
	return c.do(ctx, http.MethodGet, rawURL, origin, "", nil, out)
}

func (c *Client) postJSON(ctx context.Context, rawURL, origin, token string, body, out any) error {
	return c.do(ctx, http.MethodPost, rawURL, origin, token, body, out)
}

func (c *Client) do(ctx context.Context, method, rawURL, origin, token string, body, out any) error {
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
		return err
	}
	setChromeHeaders(req.Header, origin)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("call Vueling API: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return fmt.Errorf("read Vueling API response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &StatusError{URL: strings.SplitN(rawURL, "?", 2)[0], Status: resp.StatusCode, Body: string(data[:min(len(data), 300)])}
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode Vueling API response: %w", err)
	}
	return nil
}

// setChromeHeaders sends what Chrome sends on a cross-site fetch; Akamai rejects requests without the client hints.
func setChromeHeaders(h http.Header, origin string) {
	h.Set("User-Agent", userAgent)
	h.Set("Accept", "application/json, text/plain, */*")
	h.Set("Accept-Language", "es-ES,es;q=0.9")
	h.Set("Origin", origin)
	h.Set("Referer", origin+"/")
	h.Set("Sec-Ch-Ua", `"Google Chrome";v="141", "Not?A_Brand";v="8", "Chromium";v="141"`)
	h.Set("Sec-Ch-Ua-Mobile", "?0")
	h.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	h.Set("Sec-Fetch-Site", "same-site")
	h.Set("Sec-Fetch-Mode", "cors")
	h.Set("Sec-Fetch-Dest", "empty")
}
