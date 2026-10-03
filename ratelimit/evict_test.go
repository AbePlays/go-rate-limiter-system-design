package ratelimit

import (
	"testing"
	"time"
)

func TestMemStoreCapsKeysLRU(t *testing.T) {
	s := NewMemStore(3, nil)

	s.Set("a", []byte("1"), 0)
	s.Set("b", []byte("2"), 0)
	s.Set("c", []byte("3"), 0)
	s.Get("a")
	s.Set("d", []byte("4"), 0)

	if _, ok := s.Get("b"); ok {
		t.Fatal("expected least-recently-used b evicted")
	}
	if _, ok := s.Get("a"); !ok {
		t.Fatal("expected recently-used a kept")
	}
	if n := s.Len(); n != 3 {
		t.Fatalf("expected 3 keys, got %d", n)
	}
}

func TestMemStoreSubSecondTTLKept(t *testing.T) {
	s := NewMemStore(0, nil)

	s.Set("k", []byte("v"), 500*time.Millisecond)
	if _, ok := s.Get("k"); !ok {
		t.Fatal("sub-second TTL must round up, not expire instantly")
	}
}

func TestMemStoreExpiry(t *testing.T) {
	now := int64(1_000_000)
	s := NewMemStore(0, func() int64 { return now })

	s.Set("k", []byte("v"), time.Minute)
	now += 61
	if _, ok := s.Get("k"); ok {
		t.Fatal("expected expired key gone")
	}
	if n := s.Len(); n != 0 {
		t.Fatalf("expected len 0, got %d", n)
	}
}

func TestLimiterMapStaysBounded(t *testing.T) {
	fw := NewFixedWindow(1000, time.Minute, WithFixedStore(NewMemStore(10, nil)))

	for i := range 25 {
		fw.Allow(string(rune('a' + i)))
	}
	if n := len(fw.store.data); n > 10 {
		t.Fatalf("expected at most 10 keys, got %d", n)
	}
}
