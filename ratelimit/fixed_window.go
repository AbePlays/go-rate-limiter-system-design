package ratelimit

import (
	"encoding/json"
	"sync"
	"time"
)

type Decision struct {
	Allowed    bool
	Remaining  int
	ResetAt    int64
	RetryAfter int
	Fallback   bool
}

type entry struct {
	Count int   `json:"count"`
	Tick  int64 `json:"tick"`
}

type FixedWindow struct {
	store  *MemStore
	limit  int
	mutex  sync.Mutex
	window time.Duration
	now    func() int64
}

func NewFixedWindow(limit int, window time.Duration, opts ...FixedOption) *FixedWindow {
	fw := &FixedWindow{
		limit:  limit,
		window: window,
		now:    func() int64 { return time.Now().Unix() },
	}
	fw.store = NewMemStore(0, fw.now)
	for _, opt := range opts {
		opt(fw)
	}
	return fw
}

type FixedOption func(*FixedWindow)

func WithFixedStore(s *MemStore) FixedOption {
	return func(fw *FixedWindow) { fw.store = s }
}

func (fw *FixedWindow) Limit() int { return fw.limit }

func (fw *FixedWindow) Allow(key string) Decision {
	fw.mutex.Lock()
	defer fw.mutex.Unlock()

	currTick := fw.now() / int64(fw.window.Seconds())

	val := &entry{Count: 0, Tick: currTick}
	if raw, ok := fw.store.Get(key); ok {
		if json.Unmarshal(raw, val) != nil {
			val = &entry{Count: 0, Tick: currTick}
		}
	}

	if val.Tick != currTick {
		val.Count = 0
		val.Tick = currTick
	}

	now := fw.now()
	windowEnd := (currTick + 1) * int64(fw.window.Seconds())

	allowed := val.Count < fw.limit
	remaining := 0
	retryAfter := int(windowEnd - now)
	if allowed {
		val.Count++
		remaining = fw.limit - val.Count
		retryAfter = 0
	}

	if raw, err := json.Marshal(val); err == nil {
		fw.store.Set(key, raw, 2*fw.window)
	}

	return Decision{Allowed: allowed, Remaining: remaining, ResetAt: windowEnd, RetryAfter: retryAfter}
}
