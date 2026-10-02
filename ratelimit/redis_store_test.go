package ratelimit

import (
	"context"
	"testing"
	"time"
)

func testRedisStore(t *testing.T) *RedisStore {
	t.Helper()
	s := NewRedisStore("localhost:6379")
	if err := s.client.FlushDB(context.Background()).Err(); err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	t.Cleanup(func() { s.client.FlushDB(context.Background()) })
	return s
}

func TestRedisBackedFixedWindow(t *testing.T) {
	fw := NewFixedWindow(2, time.Minute, WithFixedStore(testRedisStore(t)))

	for range 2 {
		if !fw.Allow("ip").Allowed {
			t.Fatal("expected allowed")
		}
	}
	if d := fw.Allow("ip"); d.Allowed || d.RetryAfter <= 0 {
		t.Fatalf("expected deny with RetryAfter, got %+v", d)
	}
}

func TestRedisBackedTokenBucket(t *testing.T) {
	tb := NewTokenBucket(2, 1, WithBucketStore(testRedisStore(t)))

	for range 2 {
		if !tb.Allow("ip").Allowed {
			t.Fatal("expected allowed")
		}
	}
	if d := tb.Allow("ip"); d.Allowed {
		t.Fatal("expected deny after burst")
	}
}

func TestRedisBackedSlidingWindow(t *testing.T) {
	sw := NewSlidingWindow(2, time.Minute, WithSlidingStore(testRedisStore(t)))

	for range 2 {
		if !sw.Allow("ip").Allowed {
			t.Fatal("expected allowed")
		}
	}
	if d := sw.Allow("ip"); d.Allowed {
		t.Fatal("expected deny at limit")
	}
}

func TestRedisStoreBasics(t *testing.T) {
	s := testRedisStore(t)
	s.Set("k", []byte("v"), 2*time.Second)
	if _, ok := s.Get("k"); !ok {
		t.Fatal("expected key present")
	}
	if n := s.Len(); n != 1 {
		t.Fatalf("expected len 1, got %d", n)
	}
}
