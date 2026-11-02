package api

import (
	"net/http"
	"testing"
)

func TestNewUsesDefaultAPIEndpointsAndChromeTransport(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if client.HTTP == nil || client.HTTP.Timeout == 0 || client.APIW != DefaultAPIW || client.AMS != DefaultAMS || client.Static != DefaultStatic || client.WWW != DefaultWWW {
		t.Fatalf("New() returned incomplete client: %+v", client)
	}
	if _, ok := client.HTTP.Transport.(*http.Transport); !ok {
		t.Fatalf("New() transport = %T, want *http.Transport", client.HTTP.Transport)
	}
	if client.SessionFile != DefaultSessionFile() {
		t.Fatalf("New() SessionFile = %q", client.SessionFile)
	}
}
