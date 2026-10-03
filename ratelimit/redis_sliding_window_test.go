package ratelimit

import (
	"testing"
	"time"
)

func TestRedisSlidingBoundsBoundaryBurst(t *testing.T) {
	r := NewRedisSlidingWindow(testRedisClient(t), 10, time.Minute)
	now := int64(1_000_019)
	r.now = func() int64 { return now }

	admitted := 0
	for range 10 {
		if r.Allow("ip").Allowed {
			admitted++
		}
	}

	now += 1
	for range 10 {
		if r.Allow("ip").Allowed {
			admitted++
		}
	}

	if admitted > 11 {
		t.Fatalf("expected boundary burst capped near limit, got %d admits", admitted)
	}
}

func TestRedisSlidingRemainingCountsPreviousWindow(t *testing.T) {
	r := NewRedisSlidingWindow(testRedisClient(t), 10, time.Minute)
	now := int64(1_000_079) // last second of a window
	r.now = func() int64 { return now }

	for range 10 {
		r.Allow("ip")
	}

	now = 1_000_110 // halfway through the next window: previous counts 50%
	d := r.Allow("ip")
	if !d.Allowed || d.Remaining != 4 {
		t.Fatalf("expected allow with remaining 4, got %+v", d)
	}
	for i := range d.Remaining {
		if !r.Allow("ip").Allowed {
			t.Fatalf("request %d of the advertised remaining was denied", i+1)
		}
	}
	if r.Allow("ip").Allowed {
		t.Fatal("allowed past the advertised remaining")
	}
}
