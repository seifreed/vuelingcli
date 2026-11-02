package api

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	utls "github.com/refraction-networking/utls"
)

// newChromeTransport presents Chrome's ClientHello; Vueling's Akamai edge rejects Go's default TLS fingerprint.
// ALPN is pinned to http/1.1 so the standard transport can carry the connection.
func newChromeTransport() (*http.Transport, error) {
	return newChromeTransportWithSpec(chromeSpec)
}

func newChromeTransportWithSpec(specFactory func() (utls.ClientHelloSpec, error)) (*http.Transport, error) {
	if _, err := specFactory(); err != nil {
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
	return dialChromeWithSpec(ctx, network, addr, chromeSpec, nil)
}

func dialChromeWithSpec(ctx context.Context, network, addr string, specFactory func() (utls.ClientHelloSpec, error), roots *x509.CertPool) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	raw, err := (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	spec, err := specFactory()
	if err != nil {
		return nil, closeAfterError(raw, err)
	}
	conn := utls.UClient(raw, &utls.Config{ServerName: host, RootCAs: roots}, utls.HelloCustom)
	if err := conn.ApplyPreset(&spec); err != nil {
		return nil, closeAfterError(raw, fmt.Errorf("utls preset: %w", err))
	}
	if err := conn.HandshakeContext(ctx); err != nil {
		return nil, closeAfterError(raw, fmt.Errorf("utls handshake: %w", err))
	}
	return conn, nil
}

func closeAfterError(conn net.Conn, err error) error {
	return errors.Join(err, conn.Close())
}

func chromeSpec() (utls.ClientHelloSpec, error) {
	return chromeSpecFor(utls.HelloChrome_Auto)
}

func chromeSpecFor(id utls.ClientHelloID) (utls.ClientHelloSpec, error) {
	spec, err := utls.UTLSIdToSpec(id)
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
