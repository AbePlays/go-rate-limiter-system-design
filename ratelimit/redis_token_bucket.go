package ratelimit

import (
	"context"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// The script reads the clock from Redis (TIME) so every app instance agrees on
// "now" regardless of local clock skew, and clamps elapsed time at zero so a
// backwards step can never drain tokens. ARGV[4] is an optional time override
// used only by tests to simulate elapsed time; production passes "".
// Tokens and now are returned as strings because Redis truncates Lua floats
// to integers in replies.
var tokenBucketScript = redis.NewScript(`
local capacity = tonumber(ARGV[1])
local refill = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])

local now
if ARGV[4] ~= '' then
	now = tonumber(ARGV[4])
else
	local t = redis.call('TIME')
	now = tonumber(t[1]) + tonumber(t[2]) / 1000000
end

local tokens = tonumber(redis.call('HGET', KEYS[1], 'tokens'))
local last = tonumber(redis.call('HGET', KEYS[1], 'last'))
if tokens == nil or last == nil then
	tokens = capacity
	last = now
end

tokens = math.min(capacity, tokens + math.max(0, now - last) * refill)
local allowed = 0
if tokens >= 1 then
	tokens = tokens - 1
	allowed = 1
end

redis.call('HSET', KEYS[1], 'tokens', tokens, 'last', now)
redis.call('EXPIRE', KEYS[1], ttl)
return {allowed, tostring(tokens), tostring(now)}
`)

type RedisTokenBucket struct {
	client     *redis.Client
	capacity   float64
	prefix     string
	refill     float64
	now        func() int64
	failClosed bool
}

// NewRedisTokenBucket leaves now nil: the Redis server clock is used. Tests
// may set now to override it.
func NewRedisTokenBucket(client *redis.Client, capacity, refill float64) *RedisTokenBucket {
	return &RedisTokenBucket{
		client:   client,
		capacity: capacity,
		prefix:   "rl:tb:",
		refill:   refill,
	}
}

func (r *RedisTokenBucket) Limit() int { return int(r.capacity) }

func (r *RedisTokenBucket) SetFailClosed(v bool) { r.failClosed = v }

func (r *RedisTokenBucket) fallback() Decision {
	now := time.Now().Unix()
	if r.now != nil {
		now = r.now()
	}
	wait := int(math.Ceil(1 / r.refill))
	if r.failClosed {
		return Decision{Allowed: false, Remaining: 0, ResetAt: now + int64(wait), RetryAfter: wait}
	}
	return Decision{Allowed: true, Remaining: int(r.capacity), ResetAt: now + int64(math.Ceil(r.capacity/r.refill)), RetryAfter: 0, Fallback: true}
}

func (r *RedisTokenBucket) Allow(key string) Decision {
	ttl := int64(math.Ceil(2 * r.capacity / r.refill))
	if ttl < 1 {
		ttl = 1
	}

	override := ""
	if r.now != nil {
		override = strconv.FormatInt(r.now(), 10)
	}

	res, err := tokenBucketScript.Run(
		context.Background(), r.client,
		[]string{r.prefix + key},
		r.capacity, r.refill, ttl, override,
	).Slice()
	if err != nil || len(res) != 3 {
		return r.fallback()
	}

	allowedFlag, _ := res[0].(int64)
	tokens := redisNumber(res[1])
	now := redisNumber(res[2])

	allowed := allowedFlag == 1
	remaining := 0
	retryAfter := int(math.Ceil((1 - tokens) / r.refill))
	if allowed {
		remaining = int(tokens)
		retryAfter = 0
	}
	resetAt := int64(math.Ceil(now + (r.capacity-tokens)/r.refill))
	return Decision{Allowed: allowed, Remaining: remaining, ResetAt: resetAt, RetryAfter: retryAfter}
}

func redisNumber(v any) float64 {
	switch n := v.(type) {
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	case int64:
		return float64(n)
	case float64:
		return n
	}
	return 0
}
