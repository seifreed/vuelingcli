package api

import (
	"context"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	utls "github.com/refraction-networking/utls"
)

func TestClientConstructionPropagatesTransportFactoryErrors(t *testing.T) {
	wantErr := errors.New("transport unavailable")
	if client, err := newClient(func() (*http.Transport, error) { return nil, wantErr }); client != nil || !errors.Is(err, wantErr) {
		t.Fatalf("newClient = %v, %v; want transport error", client, err)
	}
	if transport, err := newChromeTransportWithSpec(func() (utls.ClientHelloSpec, error) { return utls.ClientHelloSpec{}, wantErr }); transport != nil || !errors.Is(err, wantErr) {
		t.Fatalf("newChromeTransportWithSpec = %v, %v; want spec error", transport, err)
	}
	if _, err := chromeSpecFor(utls.ClientHelloID{}); err == nil {
		t.Fatal("chromeSpecFor accepted an unknown ClientHello ID")
	}
}

func TestDialChromeCoversConnectAndHandshakeOutcomes(t *testing.T) {
	if _, err := dialChrome(context.Background(), "tcp", "not-a-host-port"); err == nil {
		t.Fatal("dialChrome accepted an invalid host:port")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedAddress := listener.Addr().String()
	_ = listener.Close()
	if _, err := dialChrome(context.Background(), "tcp", closedAddress); err == nil {
		t.Fatal("dialChrome connected to a closed listener")
	}

	listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			_ = conn.Close()
		}
	}()
	if _, err := dialChrome(context.Background(), "tcp", listener.Addr().String()); err == nil || !strings.Contains(err.Error(), "utls handshake") {
		t.Fatalf("dialChrome handshake error = %v", err)
	}

	server := httptest.NewTLSServer(nil)
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	conn, err := dialChromeWithSpec(context.Background(), "tcp", server.Listener.Addr().String(), chromeSpec, roots)
	if err != nil {
		t.Fatalf("dialChromeWithSpec TLS success = %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDialChromeClosesConnectionWhenSpecOrPresetFails(t *testing.T) {
	badSpecErr := errors.New("spec unavailable")
	for _, factory := range []func() (utls.ClientHelloSpec, error){
		func() (utls.ClientHelloSpec, error) { return utls.ClientHelloSpec{}, badSpecErr },
		func() (utls.ClientHelloSpec, error) {
			return utls.ClientHelloSpec{Extensions: []utls.TLSExtension{&utls.SupportedVersionsExtension{Versions: nil}}}, nil
		},
	} {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = listener.Close() }()
		if _, err := dialChromeWithSpec(context.Background(), "tcp", listener.Addr().String(), factory, nil); err == nil {
			t.Fatal("dialChromeWithSpec accepted an invalid TLS specification")
		}
	}
}
