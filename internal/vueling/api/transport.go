package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	utls "github.com/refraction-networking/utls"
)

// newChromeTransport presents Chrome's ClientHello; Vueling's Akamai edge rejects Go's default TLS fingerprint.
// ALPN is pinned to http/1.1 so the standard transport can carry the connection.
func newChromeTransport() (*http.Transport, error) {
	if _, err := chromeSpec(); err != nil {
		return nil, err
	}
	return &http.Transport{
		DialTLSContext:      dialChrome,
		MaxIdleConns:        10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 15 * time.Second,
	}, nil
}

func dialChrome(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	raw, err := (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	spec, err := chromeSpec()
	if err != nil {
		_ = raw.Close()
		return nil, err
	}
	conn := utls.UClient(raw, &utls.Config{ServerName: host}, utls.HelloCustom)
	if err := conn.ApplyPreset(&spec); err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("utls preset: %w", err)
	}
	if err := conn.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("utls handshake: %w", err)
	}
	return conn, nil
}

func chromeSpec() (utls.ClientHelloSpec, error) {
	spec, err := utls.UTLSIdToSpec(utls.HelloChrome_Auto)
	if err != nil {
		return spec, err
	}
	for _, ext := range spec.Extensions {
		if alpn, ok := ext.(*utls.ALPNExtension); ok {
			alpn.AlpnProtocols = []string{"http/1.1"}
		}
	}
	return spec, nil
}
