package ratelimit

import (
	"sync"
)

type PolicySet struct {
	mu     sync.RWMutex
	byName map[string]Limiter
	floor  Limiter
}

func NewPolicySet(floor Limiter) *PolicySet {
	return &PolicySet{byName: make(map[string]Limiter), floor: floor}
}

func (p *PolicySet) Add(name string, l Limiter) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.byName[name] = l
}

func (p *PolicySet) Limit(name string) (int, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	l, ok := p.byName[name]
	if !ok {
		return 0, false
	}
	return l.Limit(), true
}

// Allow checks the per-IP floor first, then the named per-key policy. Floor
// first means a flood of forged keys is rejected before it can create any
// per-key state, and a floor denial never spends per-key quota. The returned
// Decision and limit always describe the constraint the client is closest to
// hitting, so headers never mix one limiter's limit with the other's counts.
func (p *PolicySet) Allow(name, key, ip string) (Decision, int, bool) {
	p.mu.RLock()
	l, ok := p.byName[name]
	p.mu.RUnlock()

	if !ok {
		return Decision{}, 0, false
	}

	f := p.floor.Allow(ip)
	if !f.Allowed {
		return f, p.floor.Limit(), true
	}

	d := l.Allow(key)
	if !d.Allowed {
		return d, l.Limit(), true
	}

	fallback := f.Fallback || d.Fallback
	f.Fallback, d.Fallback = fallback, fallback

	if f.Remaining < d.Remaining {
		return f, p.floor.Limit(), true
	}
	return d, l.Limit(), true
}
