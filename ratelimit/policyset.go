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

func (p *PolicySet) Allow(name, key, ip string) (Decision, int, bool) {
	p.mu.RLock()
	l, ok := p.byName[name]
	p.mu.RUnlock()

	if !ok {
		return Decision{}, 0, false
	}

	d := l.Allow(key)
	if !d.Allowed {
		return d, l.Limit(), true
	}

	f := p.floor.Allow(ip)
	if !f.Allowed {
		return f, p.floor.Limit(), true
	}

	return d, l.Limit(), true
}
