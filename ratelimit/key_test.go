package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestKey(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		forwarded  string
		apiKey     string
		wantKey    string
		wantIdent  bool
	}{
		{"header wins", "10.0.0.1:1234", "", "cust-1", "cust-1", true},
		{"padded header trims to same identity", "10.0.0.1:1234", "", "  cust-1  ", "cust-1", true},
		{"spaces-only falls back to ip", "10.0.0.1:1234", "", "   ", "10.0.0.1", false},
		{"absent header falls back to ip", "10.0.0.1:1234", "", "", "10.0.0.1", false},
		{"header beats forwarded-for", "10.0.0.1:1234", "1.2.3.4", "cust-1", "cust-1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remoteAddr
			if tt.forwarded != "" {
				r.Header.Set("X-Forwarded-For", tt.forwarded)
			}
			if tt.apiKey != "" {
				r.Header.Set("api_key", tt.apiKey)
			}
			key, ident := Key(r)
			if key != tt.wantKey || ident != tt.wantIdent {
				t.Fatalf("Key() = (%q, %v), want (%q, %v)", key, ident, tt.wantKey, tt.wantIdent)
			}
		})
	}
}
