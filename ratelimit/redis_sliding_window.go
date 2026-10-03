package ratelimit

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Tick, overlap, and remaining are all computed from Redis TIME so every
// instance agrees on window boundaries. nowOverride is a test seam:
// production passes "" and the server clock is used.
var slidingWindowScript = redis.NewScript(`
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
local overlap = 1 - (now - tick * window) / window
local currkey = KEYS[1] .. ':' .. tick
local prevkey = KEYS[1] .. ':' .. (tick - 1)
local curr = tonumber(redis.call('GET', currkey) or '0')
local prev = tonumber(redis.call('GET', prevkey) or '0')
local estimate = curr + prev * overlap
if estimate < limit then
	curr = redis.call('INCR', currkey)
	redis.call('EXPIRE', currkey, 2 * window)
	redis.call('EXPIRE', prevkey, 2 * window)
	local remaining = math.max(0, math.ceil(limit - (curr + prev * overlap) - 1e-9))
	return {1, remaining, tostring(now)}
end
return {0, 0, tostring(now)}
`)

type RedisSlidingWindow struct {
	client     *redis.Client
	limit      int
	prefix     string
	window     time.Duration
	now        func() int64
	failClosed bool
}

func NewRedisSlidingWindow(client *redis.Client, limit int, window time.Duration) *RedisSlidingWindow {
	return &RedisSlidingWindow{
		client: client,
		limit:  limit,
		prefix: "rl:sw:",
		window: window,
	}
}

func (r *RedisSlidingWindow) Limit() int { return r.limit }

func (r *RedisSlidingWindow) SetFailClosed(v bool) { r.failClosed = v }

func (r *RedisSlidingWindow) fallback(windowEnd, now int64) Decision {
	if r.failClosed {
		return Decision{Allowed: false, Remaining: 0, ResetAt: windowEnd, RetryAfter: int(windowEnd - now)}
	}
	return Decision{Allowed: true, Remaining: r.limit, ResetAt: windowEnd, RetryAfter: 0, Fallback: true}
}

func (r *RedisSlidingWindow) Allow(key string) Decision {
	windowSec := int64(r.window.Seconds())
	override := ""
	if r.now != nil {
		override = strconv.FormatInt(r.now(), 10)
	}

	res, err := slidingWindowScript.Run(
		context.Background(), r.client,
		[]string{r.prefix + key},
		r.limit, windowSec, override,
	).Slice()
	if err != nil || len(res) != 3 {
		now := time.Now().Unix()
		return r.fallback((now/windowSec+1)*windowSec, now)
	}

	allowed, ok1 := res[0].(int64)
	remaining, ok2 := res[1].(int64)
	if !ok1 || !ok2 {
		now := time.Now().Unix()
		return r.fallback((now/windowSec+1)*windowSec, now)
	}
	now := redisNumber(res[2])
	tick := int64(now) / windowSec
	windowEnd := (tick + 1) * windowSec

	if allowed == 1 {
		return Decision{Allowed: true, Remaining: int(remaining), ResetAt: windowEnd, RetryAfter: 0}
	}
	return Decision{Allowed: false, Remaining: 0, ResetAt: windowEnd, RetryAfter: int(windowEnd - int64(now))}
}
