// Package browser reads rendered pages from the user's local browser session.
package browser

import (
	"context"
	"runtime"
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
	return readPageWithDevTools(ctx, devToolsEndpoint(reader.endpoint), pageURL, script)
}
