package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		forwarded  string
		want       string
	}{
		{"forwarded single", "10.0.0.1:1234", "1.2.3.4", "1.2.3.4"},
		{"forwarded chain takes leftmost", "10.0.0.1:1234", "1.2.3.4, 5.6.7.8", "1.2.3.4"},
		{"no header strips port", "1.2.3.4:56789", "", "1.2.3.4"},
		{"malformed remote addr passes through", "not-an-addr", "", "not-an-addr"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remoteAddr
			if tt.forwarded != "" {
				r.Header.Set("X-Forwarded-For", tt.forwarded)
			}
			if got := ClientIP(r); got != tt.want {
				t.Fatalf("ClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHeaders(t *testing.T) {
	allow := Headers(10, Decision{Allowed: true, Remaining: 7, ResetAt: 999})
	if allow.Get("X-RateLimit-Limit") != "10" || allow.Get("X-RateLimit-Remaining") != "7" || allow.Get("X-RateLimit-Reset") != "999" {
		t.Fatalf("allow headers wrong: %v", allow)
	}
	if allow.Get("Retry-After") != "" {
		t.Fatalf("allow must not set Retry-After, got %q", allow.Get("Retry-After"))
	}
	deny := Headers(10, Decision{Allowed: false, Remaining: 0, ResetAt: 999, RetryAfter: 17})
	if deny.Get("Retry-After") != "17" {
		t.Fatalf("deny must set Retry-After=17, got %q", deny.Get("Retry-After"))
	}
}

func TestMiddleware(t *testing.T) {
	reached := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true })
	h := Middleware(NewFixedWindow(2, time.Minute), inner)

	for i := range 2 {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusOK || !reached {
			t.Fatalf("request %d: expected passthrough, got %d reached=%v", i+1, rec.Code, reached)
		}
		if rec.Header().Get("X-RateLimit-Remaining") == "" {
			t.Fatal("allowed response missing limit headers")
		}
		reached = false
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("deny missing Retry-After")
	}
	if reached {
		t.Fatal("denied request reached the inner handler")
	}
}
