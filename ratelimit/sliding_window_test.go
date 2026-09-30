package ratelimit

import (
	"testing"
	"time"
)

func TestSlidingAllowsUpToLimit(t *testing.T) {
	sw := NewSlidingWindow(3, time.Minute)

	for i := 0; i < 3; i++ {
		if d := sw.Allow("ip"); !d.Allowed {
			t.Fatalf("request %d: expected allowed, got denied", i+1)
		}
	}
	if d := sw.Allow("ip"); d.Allowed {
		t.Fatal("request 4: expected denied, got allowed")
	} else if d.RetryAfter <= 0 {
		t.Fatalf("denied request: expected positive RetryAfter, got %d", d.RetryAfter)
	}
}

func TestSlidingBoundsBoundaryBurst(t *testing.T) {
	sw := NewSlidingWindow(10, time.Minute)
	// 59s into the tick: 10 requests land, then 1s later the window rolls
	// and 10 more arrive — 20 requests within ~2 seconds.
	now := int64(1_000_019)
	sw.now = func() int64 { return now }

	admitted := 0
	for range 10 {
		if sw.Allow("ip").Allowed {
			admitted++
		}
	}

	now += 1 // cross the tick with the previous window fully overlapping
	for range 10 {
		if sw.Allow("ip").Allowed {
			admitted++
		}
	}

	if admitted > 11 {
		t.Fatalf("expected boundary burst capped near limit, got %d admits", admitted)
	}
}

func TestSlidingForgetsIdleKeys(t *testing.T) {
	sw := NewSlidingWindow(2, time.Minute)
	now := int64(1_000_000)
	sw.now = func() int64 { return now }

	sw.Allow("ip")
	sw.Allow("ip")
	if d := sw.Allow("ip"); d.Allowed {
		t.Fatal("at limit: expected denied, got allowed")
	}

	now += 180 // idle past a whole window: previous must read as zero
	for i := 0; i < 2; i++ {
		if d := sw.Allow("ip"); !d.Allowed {
			t.Fatalf("after idle: request %d expected allowed, got denied", i+1)
		}
	}
}
