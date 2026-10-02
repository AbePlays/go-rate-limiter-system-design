package ratelimit

import (
	"testing"
	"time"
)

func TestFixedWindowCapsKeys(t *testing.T) {
	fw := NewFixedWindow(100, time.Minute)
	now := int64(1_000_000)
	fw.now = func() int64 { return now }
	fw.SetMaxKeys(10)

	for i := range 25 {
		fw.Allow(string(rune('a' + i)))
		now += 1
	}
	if n := fw.store.Len(); n > 10 {
		t.Fatalf("expected at most 10 keys, got %d", n)
	}
}

func TestEvictionPrefersStale(t *testing.T) {
	fw := NewFixedWindow(100, time.Minute)
	now := int64(1_000_000)
	fw.now = func() int64 { return now }
	fw.SetMaxKeys(3)

	fw.Allow("old1")
	fw.Allow("old2")
	now += 180 // both entries stale (3 windows behind)
	fw.Allow("new1")
	fw.Allow("new2")

	if _, ok := fw.store.Get("old1"); ok {
		t.Fatal("expected stale old1 evicted")
	}
	if _, ok := fw.store.Get("old2"); ok {
		t.Fatal("expected stale old2 evicted")
	}
}
