package ratelimit

import (
	"sync"
	"time"
)

type Decision struct {
	Allowed    bool
	Remaining  int
	ResetAt    int64
	RetryAfter int
}

type entry struct {
	count int
	tick  int64
}

type FixedWindow struct {
	counts map[string]*entry
	limit  int
	mutex  sync.Mutex
	window time.Duration
	now    func() int64
}

func New(limit int, window time.Duration) *FixedWindow {
	return &FixedWindow{
		counts: make(map[string]*entry),
		limit:  limit,
		window: window,
		now:    func() int64 { return time.Now().Unix() },
	}
}

func (fw *FixedWindow) Allow(key string) Decision {
	fw.mutex.Lock()
	defer fw.mutex.Unlock()

	currTick := fw.now() / int64(fw.window.Seconds())

	val, ok := fw.counts[key]
	if !ok {
		fw.counts[key] = &entry{count: 0, tick: currTick}
		val = fw.counts[key]
	}

	if val.tick != currTick {
		val.count = 0
		val.tick = currTick
	}

	now := fw.now()
	windowEnd := (currTick + 1) * int64(fw.window.Seconds())

	if val.count < fw.limit {
		val.count++
		return Decision{Allowed: true, Remaining: fw.limit - val.count, ResetAt: windowEnd, RetryAfter: 0}
	}

	return Decision{Allowed: false, Remaining: 0, ResetAt: windowEnd, RetryAfter: int(windowEnd - now)}
}
