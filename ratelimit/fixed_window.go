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
}

type entry struct {
	Count int   `json:"count"`
	Tick  int64 `json:"tick"`
}

type FixedWindow struct {
	store   Store
	limit   int
	maxKeys int
	mutex   sync.Mutex
	window  time.Duration
	now     func() int64
}

func NewFixedWindow(limit int, window time.Duration, opts ...FixedOption) *FixedWindow {
	fw := &FixedWindow{
		store:   NewMemStore(),
		limit:   limit,
		maxKeys: defaultMaxKeys,
		window:  window,
		now:     func() int64 { return time.Now().Unix() },
	}
	for _, opt := range opts {
		opt(fw)
	}
	return fw
}

type FixedOption func(*FixedWindow)

func WithFixedStore(s Store) FixedOption {
	return func(fw *FixedWindow) { fw.store = s }
}

func (fw *FixedWindow) SetMaxKeys(n int) { fw.maxKeys = n }

func (fw *FixedWindow) Limit() int { return fw.limit }

func (fw *FixedWindow) evict(currTick int64) {
	if fw.store.Len() < fw.maxKeys {
		return
	}
	for _, k := range fw.store.Keys() {
		if raw, ok := fw.store.Get(k); ok {
			var e entry
			if json.Unmarshal(raw, &e) == nil && e.Tick < currTick-1 {
				fw.store.Delete(k)
			}
		}
	}
	if fw.store.Len() < fw.maxKeys {
		return
	}
	for _, k := range fw.store.Keys() {
		fw.store.Delete(k)
		break
	}
}

func (fw *FixedWindow) Allow(key string) Decision {
	fw.mutex.Lock()
	defer fw.mutex.Unlock()

	currTick := fw.now() / int64(fw.window.Seconds())

	val := &entry{Count: 0, Tick: currTick}
	if raw, ok := fw.store.Get(key); ok {
		if json.Unmarshal(raw, val) != nil {
			val = &entry{Count: 0, Tick: currTick}
		}
	} else {
		fw.evict(currTick)
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
