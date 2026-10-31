package browser

import "testing"

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
