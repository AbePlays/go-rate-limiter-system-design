package ratelimit

import (
	"math"
	"sync"
	"time"
)

type bucket struct {
	last   int64
	tokens float64
}

type TokenBucket struct {
	buckets  map[string]*bucket
	capacity float64
	maxKeys  int
	mutex    sync.Mutex
	now      func() int64
	refill   float64
}

func NewTokenBucket(capacity, refill float64) *TokenBucket {
	return &TokenBucket{
		buckets:  make(map[string]*bucket),
		capacity: capacity,
		maxKeys:  defaultMaxKeys,
		now:      func() int64 { return time.Now().Unix() },
		refill:   refill,
	}
}

func (tb *TokenBucket) Limit() int { return int(tb.capacity) }

func (tb *TokenBucket) SetMaxKeys(n int) { tb.maxKeys = n }

func (tb *TokenBucket) Allow(key string) Decision {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()

	now := tb.now()
	val, ok := tb.buckets[key]
	if !ok {
		evictIfFull(tb.buckets, tb.maxKeys, func(b *bucket) bool {
			return float64(now-b.last)*tb.refill >= 2*tb.capacity
		})
		tb.buckets[key] = &bucket{tokens: tb.capacity, last: now}
		val = tb.buckets[key]
	}

	newTokens := val.tokens + float64(now-val.last)*tb.refill
	val.tokens = min(newTokens, tb.capacity)
	val.last = now

	if val.tokens < 1 {
		return Decision{Allowed: false, Remaining: 0, ResetAt: now + int64((tb.capacity-val.tokens)/tb.refill), RetryAfter: int(math.Ceil((1 - val.tokens) / tb.refill))}
	}

	val.tokens = val.tokens - 1
	return Decision{Allowed: true, Remaining: int(val.tokens), ResetAt: now + int64((tb.capacity-val.tokens)/tb.refill), RetryAfter: 0}
}
