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
	store    Store
	capacity float64
	maxKeys  int
	mutex    sync.Mutex
	now      func() int64
	refill   float64
}

func NewTokenBucket(capacity, refill float64, opts ...BucketOption) *TokenBucket {
	tb := &TokenBucket{
		store:    NewMemStore(),
		capacity: capacity,
		maxKeys:  defaultMaxKeys,
		now:      func() int64 { return time.Now().Unix() },
		refill:   refill,
	}
	for _, opt := range opts {
		opt(tb)
	}
	return tb
}

type BucketOption func(*TokenBucket)

func WithBucketStore(s Store) BucketOption {
	return func(tb *TokenBucket) { tb.store = s }
}

func (tb *TokenBucket) Limit() int { return int(tb.capacity) }

func (tb *TokenBucket) SetMaxKeys(n int) { tb.maxKeys = n }

func (tb *TokenBucket) evict(now int64) {
	if tb.store.Len() < tb.maxKeys {
		return
	}
	for _, k := range tb.store.Keys() {
		if raw, ok := tb.store.Get(k); ok {
			var b bucket
			if json.Unmarshal(raw, &b) == nil && float64(now-b.Last)*tb.refill >= 2*tb.capacity {
				tb.store.Delete(k)
			}
		}
	}
	if tb.store.Len() < tb.maxKeys {
		return
	}
	for _, k := range tb.store.Keys() {
		tb.store.Delete(k)
		break
	}
}

func (tb *TokenBucket) Allow(key string) Decision {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()

	now := tb.now()
	val := &bucket{Tokens: tb.capacity, Last: now}
	if raw, ok := tb.store.Get(key); ok {
		if json.Unmarshal(raw, val) != nil {
			val = &bucket{Tokens: tb.capacity, Last: now}
		}
	} else {
		tb.evict(now)
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
