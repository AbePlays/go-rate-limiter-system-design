package ratelimit

import (
	"testing"
	"time"
)

func TestAllowsUpToLimit(t *testing.T) {
	fw := New(3, time.Minute)

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
	fw := New(10, time.Minute)
	now := int64(1_000_000)
	fw.now = func() int64 { return now }

	admitted := 0
	for range 10 {
		if fw.Allow("ip").Allowed {
			admitted++
		}
	}

	now += 60 // cross into the next window without waiting
	for range 10 {
		if fw.Allow("ip").Allowed {
			admitted++
		}
	}

	if admitted != 20 {
		t.Fatalf("expected boundary burst of 20 admits, got %d", admitted)
	}
}
