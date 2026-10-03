package ratelimit

import (
	"encoding/json"
	"math"
	"sync"
	"time"
)

type windowCounts struct {
	Curr int   `json:"curr"`
	Prev int   `json:"prev"`
	Tick int64 `json:"tick"`
}

type SlidingWindow struct {
	store   Store
	limit   int
	maxKeys int
	mutex   sync.Mutex
	now     func() int64
	window  time.Duration
}

func NewSlidingWindow(limit int, window time.Duration, opts ...SlidingOption) *SlidingWindow {
	sw := &SlidingWindow{
		store:   NewMemStore(),
		limit:   limit,
		maxKeys: defaultMaxKeys,
		window:  window,
		now:     func() int64 { return time.Now().Unix() },
	}
	for _, opt := range opts {
		opt(sw)
	}
	return sw
}

type SlidingOption func(*SlidingWindow)

func WithSlidingStore(s Store) SlidingOption {
	return func(sw *SlidingWindow) { sw.store = s }
}

func (sw *SlidingWindow) Limit() int { return sw.limit }

func (sw *SlidingWindow) SetMaxKeys(n int) { sw.maxKeys = n }

// slidingRemaining is how many more requests the weighted estimate would admit
// right now: the same "estimate < limit" rule Allow applies, run forward. The
// epsilon stops float error from shifting the answer when the estimate lands
// exactly on a whole number.
func slidingRemaining(limit, curr, prev int, overlap float64) int {
	estimate := float64(curr) + float64(prev)*overlap
	return max(0, int(math.Ceil(float64(limit)-estimate-1e-9)))
}

func (sw *SlidingWindow) evict(currTick int64) {
	if sw.store.Len() < sw.maxKeys {
		return
	}
	for _, k := range sw.store.Keys() {
		if raw, ok := sw.store.Get(k); ok {
			var w windowCounts
			if json.Unmarshal(raw, &w) == nil && w.Tick < currTick-1 {
				sw.store.Delete(k)
			}
		}
	}
	if sw.store.Len() < sw.maxKeys {
		return
	}
	for _, k := range sw.store.Keys() {
		sw.store.Delete(k)
		break
	}
}

func (sw *SlidingWindow) Allow(key string) Decision {
	sw.mutex.Lock()
	defer sw.mutex.Unlock()

	now := sw.now()
	windowSec := int64(sw.window.Seconds())
	currTick := now / windowSec

	val := &windowCounts{Tick: currTick}
	if raw, ok := sw.store.Get(key); ok {
		if json.Unmarshal(raw, val) != nil {
			val = &windowCounts{Tick: currTick}
		}
	} else {
		sw.evict(currTick)
	}

	if val.Tick != currTick {
		if val.Tick == currTick-1 {
			val.Prev = val.Curr
		} else {
			val.Prev = 0
		}
		val.Curr = 0
		val.Tick = currTick
	}

	overlap := 1 - float64(now-currTick*windowSec)/float64(windowSec)
	estimate := float64(val.Curr) + float64(val.Prev)*overlap

	windowEnd := (currTick + 1) * windowSec
	allowed := estimate < float64(sw.limit)
	remaining := 0
	retryAfter := int(windowEnd - now)
	if allowed {
		val.Curr++
		remaining = slidingRemaining(sw.limit, val.Curr, val.Prev, overlap)
		retryAfter = 0
	}

	if raw, err := json.Marshal(val); err == nil {
		sw.store.Set(key, raw, 2*sw.window)
	}

	return Decision{Allowed: allowed, Remaining: remaining, ResetAt: windowEnd, RetryAfter: retryAfter}
}
