package ratelimit

import (
	"testing"
)

func TestRedisBucketBurstThenDeny(t *testing.T) {
	r := NewRedisTokenBucket(testRedisClient(t), 5, 1)

	for range 5 {
		if !r.Allow("ip").Allowed {
			t.Fatal("expected burst admitted")
		}
	}
	if d := r.Allow("ip"); d.Allowed || d.RetryAfter <= 0 {
		t.Fatalf("expected deny with RetryAfter, got %+v", d)
	}
}

func TestRedisBucketIdleRefills(t *testing.T) {
	r := NewRedisTokenBucket(testRedisClient(t), 5, 2)
	now := int64(1_000_000)
	r.now = func() int64 { return now }

	for range 5 {
		r.Allow("ip")
	}
	if d := r.Allow("ip"); d.Allowed {
		t.Fatal("drained bucket: expected denied")
	}

	now += 3
	if d := r.Allow("ip"); !d.Allowed || d.Remaining != 4 {
		t.Fatalf("after idle: expected allow with remaining 4, got %+v", d)
	}
}
