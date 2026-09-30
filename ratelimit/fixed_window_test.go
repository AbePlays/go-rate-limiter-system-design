package ratelimit

import (
	"testing"
	"time"
)

func TestAllowsUpToLimit(t *testing.T) {
	fw := NewFixedWindow(3, time.Minute)

	for i := range 3 {
		if d := fw.Allow("ip"); !d.Allowed {
			t.Fatalf("request %d: expected allowed, got denied", i+1)
		}
	}
	if d := fw.Allow("ip"); d.Allowed {
		t.Fatal("request 4: expected denied, got allowed")
	} else if d.RetryAfter <= 0 {
		t.Fatalf("denied request: expected positive RetryAfter, got %d", d.RetryAfter)
	}
}

func TestBoundaryBurst(t *testing.T) {
	fw := NewFixedWindow(10, time.Minute)
	// 59s into the tick: 10 requests land, then 1s later the window rolls
	// and 10 more arrive — 20 requests within ~2 seconds, all admitted.
	// This is the boundary flaw stage 2 fixes; kept as documentation.
	now := int64(1_000_019)
	fw.now = func() int64 { return now }

	admitted := 0
	for range 10 {
		if fw.Allow("ip").Allowed {
			admitted++
		}
	}

	now += 1 // cross the tick: counter resets, second burst fully admitted
	for range 10 {
		if fw.Allow("ip").Allowed {
			admitted++
		}
	}

	if admitted != 20 {
		t.Fatalf("expected boundary burst of 20 admits, got %d", admitted)
	}
}
