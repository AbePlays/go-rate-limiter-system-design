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

func TestUnknownPolicy(t *testing.T) {
	p := NewPolicySet(NewFixedWindow(100, time.Minute))
	if _, _, ok := p.Allow("nope", "k", "1.2.3.4"); ok {
		t.Fatal("unknown policy must report ok=false")
	}
}
