package ratelimit

import (
	"fmt"
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
		hops       int
		want       string
	}{
		{"untrusted ignores forwarded header", "9.9.9.9:1234", "1.2.3.4", 0, "9.9.9.9"},
		{"one hop reads rightmost", "10.0.0.1:1234", "1.2.3.4", 1, "1.2.3.4"},
		{"one hop ignores client-forged prefix", "10.0.0.1:1234", "6.6.6.6, 1.2.3.4", 1, "1.2.3.4"},
		{"two hops skips the inner proxy", "10.0.0.1:1234", "1.2.3.4, 10.0.0.2", 2, "1.2.3.4"},
		{"chain shorter than hops falls back to peer", "10.0.0.1:1234", "1.2.3.4", 2, "10.0.0.1"},
		{"garbage entry falls back to peer", "10.0.0.1:1234", "not-an-ip", 1, "10.0.0.1"},
		{"ipv6 is normalized", "10.0.0.1:1234", "2001:DB8:0:0:0:0:0:1", 1, "2001:db8::1"},
		{"no header strips port", "1.2.3.4:56789", "", 0, "1.2.3.4"},
		{"trusted but no header uses peer", "1.2.3.4:56789", "", 1, "1.2.3.4"},
		{"malformed remote addr passes through", "not-an-addr", "", 0, "not-an-addr"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetTrustedProxyHops(tt.hops)
			t.Cleanup(func() { SetTrustedProxyHops(0) })

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

func TestClientIPJoinsRepeatedForwardedHeaders(t *testing.T) {
	SetTrustedProxyHops(1)
	t.Cleanup(func() { SetTrustedProxyHops(0) })

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:1234"
	r.Header.Add("X-Forwarded-For", "6.6.6.6")
	r.Header.Add("X-Forwarded-For", "1.2.3.4")
	if got := ClientIP(r); got != "1.2.3.4" {
		t.Fatalf("ClientIP() = %q, want 1.2.3.4", got)
	}
}

func TestForgedForwardedForCannotEvadeLimit(t *testing.T) {
	SetTrustedProxyHops(0)
	h := Middleware(NewFixedWindow(1, time.Minute), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	var last int
	for i := range 3 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("1.1.1.%d", i))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		last = rec.Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("rotating X-Forwarded-For evaded the limit: last status %d", last)
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

func TestPolicyMiddleware(t *testing.T) {
	set := NewPolicySet(NewFixedWindow(1, time.Minute)) // floor: 1/min per IP
	set.Add("login", NewFixedWindow(100, time.Minute))  // roomy key policy

	reached := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true })
	h := PolicyMiddleware(set, "login", inner)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK || !reached {
		t.Fatalf("expected passthrough, got %d reached=%v", rec.Code, reached)
	}

	reached = false
	for range 3 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("api_key", "fake")
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, req)
	}
	if rec.Code != http.StatusTooManyRequests || reached {
		t.Fatalf("expected floor 429 without passthrough, got %d reached=%v", rec.Code, reached)
	}

	rec = httptest.NewRecorder()
	PolicyMiddleware(set, "nope", inner).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("unknown policy: expected 500, got %d", rec.Code)
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
