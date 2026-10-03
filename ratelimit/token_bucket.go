package ratelimit

import (
	"encoding/json"
	"math"
	"sync"
	"time"
)

type bucket struct {
	Last   int64   `json:"last"`
	Tokens float64 `json:"tokens"`
}

type TokenBucket struct {
	store    *MemStore
	capacity float64
	mutex    sync.Mutex
	now      func() int64
	refill   float64
}

func NewTokenBucket(capacity, refill float64, opts ...BucketOption) *TokenBucket {
	tb := &TokenBucket{
		capacity: capacity,
		now:      func() int64 { return time.Now().Unix() },
		refill:   refill,
	}
	tb.store = NewMemStore(0, tb.now)
	for _, opt := range opts {
		opt(tb)
	}
	return tb
}

type BucketOption func(*TokenBucket)

func WithBucketStore(s *MemStore) BucketOption {
	return func(tb *TokenBucket) { tb.store = s }
}

func (tb *TokenBucket) Limit() int { return int(tb.capacity) }

func (tb *TokenBucket) Allow(key string) Decision {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()

	now := tb.now()
	val := &bucket{Tokens: tb.capacity, Last: now}
	if raw, ok := tb.store.Get(key); ok {
		if json.Unmarshal(raw, val) != nil {
			val = &bucket{Tokens: tb.capacity, Last: now}
		}
	}

	val.Tokens = min(val.Tokens+float64(now-val.Last)*tb.refill, tb.capacity)
	val.Last = now

	allowed := val.Tokens >= 1
	remaining := 0
	retryAfter := int(math.Ceil((1 - val.Tokens) / tb.refill))
	if allowed {
		val.Tokens--
		remaining = int(val.Tokens)
		retryAfter = 0
	}

	if raw, err := json.Marshal(val); err == nil {
		tb.store.Set(key, raw, 2*time.Duration(tb.capacity/tb.refill)*time.Second)
	}

	resetAt := now + int64((tb.capacity-val.Tokens)/tb.refill)
	return Decision{Allowed: allowed, Remaining: remaining, ResetAt: resetAt, RetryAfter: retryAfter}
}
