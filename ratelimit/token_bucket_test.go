package ratelimit

import "testing"

func TestBurstThenDeny(t *testing.T) {
	tb := NewTokenBucket(5, 1)

	for i := range 5 {
		if d := tb.Allow("ip"); !d.Allowed {
			t.Fatalf("request %d: expected allowed, got denied", i+1)
		}
	}
	if d := tb.Allow("ip"); d.Allowed {
		t.Fatal("request 6: expected denied, got allowed")
	} else if d.RetryAfter <= 0 {
		t.Fatalf("denied request: expected positive RetryAfter, got %d", d.RetryAfter)
	}
}

func TestIdleRefillsBucket(t *testing.T) {
	tb := NewTokenBucket(5, 2)
	now := int64(1_000_000)
	tb.now = func() int64 { return now }

	for range 5 {
		tb.Allow("ip")
	}
	if d := tb.Allow("ip"); d.Allowed {
		t.Fatal("drained bucket: expected denied, got allowed")
	}

	now += 3 // 3s idle at 2/sec = 6 tokens, capped at 5
	if d := tb.Allow("ip"); !d.Allowed {
		t.Fatal("after idle: expected allowed, got denied")
	} else if d.Remaining != 4 {
		t.Fatalf("after idle: expected remaining 4, got %d", d.Remaining)
	}
}
