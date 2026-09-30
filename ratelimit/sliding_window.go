package ratelimit

import (
	"sync"
	"time"
)

type windowCounts struct {
	curr int
	prev int
	tick int64
}

type SlidingWindow struct {
	counts map[string]*windowCounts
	limit  int
	mutex  sync.Mutex
	now    func() int64
	window time.Duration
}

func NewSlidingWindow(limit int, window time.Duration) *SlidingWindow {
	return &SlidingWindow{
		counts: make(map[string]*windowCounts),
		limit:  limit,
		window: window,
		now:    func() int64 { return time.Now().Unix() },
	}
}

func (sw *SlidingWindow) Limit() int { return sw.limit }

func (sw *SlidingWindow) Allow(key string) Decision {
	sw.mutex.Lock()
	defer sw.mutex.Unlock()

	now := sw.now()
	windowSec := int64(sw.window.Seconds())
	currTick := now / windowSec

	val, ok := sw.counts[key]
	if !ok {
		val = &windowCounts{tick: currTick}
		sw.counts[key] = val
	} else if val.tick != currTick {
		if val.tick == currTick-1 {
			val.prev = val.curr
		} else {
			val.prev = 0
		}
		val.curr = 0
		val.tick = currTick
	}

	overlap := 1 - float64(now-currTick*windowSec)/float64(windowSec)
	estimate := float64(val.curr) + float64(val.prev)*overlap

	windowEnd := (currTick + 1) * windowSec
	if estimate < float64(sw.limit) {
		val.curr++
		return Decision{Allowed: true, Remaining: sw.limit - val.curr, ResetAt: windowEnd, RetryAfter: 0}
	}

	return Decision{Allowed: false, Remaining: 0, ResetAt: windowEnd, RetryAfter: int(windowEnd - now)}
}
