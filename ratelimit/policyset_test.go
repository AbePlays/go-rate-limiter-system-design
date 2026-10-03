package ratelimit

import (
	"fmt"
	"testing"
	"time"
)

func TestPolicyAllowsWhenBothAllow(t *testing.T) {
	p := NewPolicySet(NewFixedWindow(100, time.Minute))
	p.Add("login", NewFixedWindow(5, time.Minute))

	d, limit, ok := p.Allow("login", "cust-1", "1.2.3.4")
	if !ok || !d.Allowed || limit != 5 {
		t.Fatalf("expected allow with limit 5, got %+v limit=%d ok=%v", d, limit, ok)
	}
}

func TestFloorBindsForgedKeys(t *testing.T) {
	p := NewPolicySet(NewFixedWindow(3, time.Minute)) // strict floor
	p.Add("redirects", NewTokenBucket(1000, 100))     // generous room

	denied := false
	for i := range 10 {
		d, _, ok := p.Allow("redirects", fmt.Sprintf("fake-%d", i), "1.2.3.4")
		if !ok {
			t.Fatal("expected known policy")
		}
		if !d.Allowed {
			denied = true
			break
		}
	}
	if !denied {
		t.Fatal("floor never bound: forged keys flooded past the IP limit")
	}
}

func TestFloorDenialDoesNotSpendKeyQuota(t *testing.T) {
	key := NewFixedWindow(5, time.Minute)
	p := NewPolicySet(NewFixedWindow(1, time.Minute))
	p.Add("login", key)

	for range 4 {
		p.Allow("login", "cust-1", "1.2.3.4")
	}

	// 1 admitted + 3 floor denials: only the admitted request may have counted.
	if d := key.Allow("cust-1"); d.Remaining != 3 {
		t.Fatalf("floor denials spent per-key quota: remaining %d, want 3", d.Remaining)
	}
}

func TestFloorDenialCreatesNoKeyState(t *testing.T) {
	key := NewFixedWindow(1000, time.Minute)
	p := NewPolicySet(NewFixedWindow(3, time.Minute))
	p.Add("redirects", key)

	for i := range 50 {
		p.Allow("redirects", fmt.Sprintf("fake-%d", i), "1.2.3.4")
	}

	if n := key.store.Len(); n != 3 {
		t.Fatalf("forged keys created per-key state past the floor: %d entries, want 3", n)
	}
}

func TestReportsTighterConstraint(t *testing.T) {
	p := NewPolicySet(NewFixedWindow(10, time.Minute)) // floor tighter than policy
	p.Add("api", NewFixedWindow(100, time.Minute))

	d, limit, ok := p.Allow("api", "cust-1", "1.2.3.4")
	if !ok || !d.Allowed || limit != 10 || d.Remaining != 9 {
		t.Fatalf("expected the floor (limit 10, remaining 9) to be reported, got %+v limit=%d ok=%v", d, limit, ok)
	}
}

func TestUnknownPolicy(t *testing.T) {
	p := NewPolicySet(NewFixedWindow(100, time.Minute))
	if _, _, ok := p.Allow("nope", "k", "1.2.3.4"); ok {
		t.Fatal("unknown policy must report ok=false")
	}
}
