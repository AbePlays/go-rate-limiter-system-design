package ratelimit

import (
	"context"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var tokenBucketScript = redis.NewScript(`
local tokens = tonumber(redis.call('HGET', KEYS[1], 'tokens'))
local last = tonumber(redis.call('HGET', KEYS[1], 'last'))
local now = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local refill = tonumber(ARGV[3])
if tokens == nil then
	tokens = capacity
	last = now
end
tokens = math.min(capacity, tokens + (now - last) * refill)
local allowed = 0
if tokens >= 1 then
	tokens = tokens - 1
	allowed = 1
end
redis.call('HSET', KEYS[1], 'tokens', tokens, 'last', now)
redis.call('EXPIRE', KEYS[1], ARGV[4])
return {allowed, tokens}
`)

type RedisTokenBucket struct {
	client     *redis.Client
	capacity   float64
	prefix     string
	refill     float64
	now        func() int64
	failClosed bool
}

func NewRedisTokenBucket(client *redis.Client, capacity, refill float64) *RedisTokenBucket {
	return &RedisTokenBucket{
		client:   client,
		capacity: capacity,
		prefix:   "rl:tb:",
		refill:   refill,
		now:      func() int64 { return time.Now().Unix() },
	}
}

func (r *RedisTokenBucket) Limit() int { return int(r.capacity) }

func (r *RedisTokenBucket) SetFailClosed(v bool) { r.failClosed = v }

func (r *RedisTokenBucket) Allow(key string) Decision {
	now := r.now()
	fullIn := int64(2 * r.capacity / r.refill)

	res, err := tokenBucketScript.Run(
		context.Background(), r.client,
		[]string{r.prefix + key},
		now, r.capacity, r.refill, fullIn,
	).Slice()
	if err != nil {
		wait := int(math.Ceil(1 / r.refill))
		if r.failClosed {
			return Decision{Allowed: false, Remaining: 0, ResetAt: now + int64(wait), RetryAfter: wait}
		}
		return Decision{Allowed: true, Remaining: int(r.capacity), ResetAt: now + fullIn/2, RetryAfter: 0, Fallback: true}
	}

	allowed := res[0].(int64) == 1
	var left float64
	switch v := res[1].(type) {
	case string:
		left, _ = strconv.ParseFloat(v, 64)
	case int64:
		left = float64(v)
	}

	remaining := 0
	retryAfter := int(math.Ceil((1 - left) / r.refill))
	if allowed {
		remaining = int(left)
		retryAfter = 0
	}
	resetAt := now + int64((r.capacity-left)/r.refill)
	return Decision{Allowed: allowed, Remaining: remaining, ResetAt: resetAt, RetryAfter: retryAfter}
}
