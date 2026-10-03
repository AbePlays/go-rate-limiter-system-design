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
