package ratelimit

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// The tick is derived from Redis TIME so every instance agrees on window
// boundaries regardless of local clock skew. nowOverride is a test seam:
// production passes "" and the server clock is used.
var fixedWindowScript = redis.NewScript(`
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local now
if ARGV[3] ~= '' then
	now = tonumber(ARGV[3])
else
	local t = redis.call('TIME')
	now = tonumber(t[1]) + tonumber(t[2]) / 1000000
end
local tick = math.floor(now / window)
local key = KEYS[1] .. ':' .. tick
local count = redis.call('INCR', key)
if count == 1 then
	redis.call('EXPIRE', key, window)
end
return {count, tostring(now)}
`)

type RedisFixedWindow struct {
	client     *redis.Client
	limit      int
	prefix     string
	window     time.Duration
	now        func() int64
	failClosed bool
}

func NewRedisFixedWindow(client *redis.Client, limit int, window time.Duration) *RedisFixedWindow {
	return &RedisFixedWindow{
		client: client,
		limit:  limit,
		prefix: "rl:fw:",
		window: window,
	}
}

func (r *RedisFixedWindow) Limit() int { return r.limit }

func (r *RedisFixedWindow) SetFailClosed(v bool) { r.failClosed = v }

func (r *RedisFixedWindow) fallback(windowEnd, now int64) Decision {
	if r.failClosed {
		return Decision{Allowed: false, Remaining: 0, ResetAt: windowEnd, RetryAfter: int(windowEnd - now)}
	}
	return Decision{Allowed: true, Remaining: r.limit, ResetAt: windowEnd, RetryAfter: 0, Fallback: true}
}

func (r *RedisFixedWindow) Allow(key string) Decision {
	windowSec := int64(r.window.Seconds())
	override := ""
	if r.now != nil {
		override = strconv.FormatInt(r.now(), 10)
	}

	res, err := fixedWindowScript.Run(
		context.Background(), r.client,
		[]string{r.prefix + key},
		r.limit, windowSec, override,
	).Slice()
	if err != nil || len(res) != 2 {
		now := time.Now().Unix()
		return r.fallback((now/windowSec+1)*windowSec, now)
	}

	count, ok := res[0].(int64)
	if !ok {
		now := time.Now().Unix()
		return r.fallback((now/windowSec+1)*windowSec, now)
	}
	now := redisNumber(res[1])
	tick := int64(now) / windowSec
	windowEnd := (tick + 1) * windowSec

	if count <= int64(r.limit) {
		return Decision{Allowed: true, Remaining: r.limit - int(count), ResetAt: windowEnd, RetryAfter: 0}
	}
	return Decision{Allowed: false, Remaining: 0, ResetAt: windowEnd, RetryAfter: int(windowEnd - int64(now))}
}
