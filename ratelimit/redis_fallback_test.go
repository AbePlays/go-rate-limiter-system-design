package ratelimit

import (
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func deadClient() *redis.Client {
	return redis.NewClient(&redis.Options{Addr: "localhost:6390"})
}

func TestRedisFailOpenByDefault(t *testing.T) {
	r := NewRedisFixedWindow(deadClient(), 2, time.Minute)

	d := r.Allow("ip")
	if !d.Allowed || !d.Fallback {
		t.Fatalf("expected fallback allow, got %+v", d)
	}
	if h := Headers(r.Limit(), d); h.Get("X-RateLimit-Fallback") != "true" {
		t.Fatalf("expected fallback header, got %v", h)
	}
}

func TestRedisFailClosedOption(t *testing.T) {
	r := NewRedisFixedWindow(deadClient(), 2, time.Minute)
	r.SetFailClosed(true)

	d := r.Allow("ip")
	if d.Allowed || d.Fallback {
		t.Fatalf("expected hard deny, got %+v", d)
	}
	if h := Headers(r.Limit(), d); h.Get("Retry-After") == "" {
		t.Fatal("fail-closed deny must carry Retry-After")
	}
}
