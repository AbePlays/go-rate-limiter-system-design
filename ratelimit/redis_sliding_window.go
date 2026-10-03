package ratelimit

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var slidingWindowScript = redis.NewScript(`
local curr = tonumber(redis.call('GET', KEYS[1]) or '0')
local prev = tonumber(redis.call('GET', KEYS[2]) or '0')
local estimate = curr + prev * tonumber(ARGV[1])
if estimate < tonumber(ARGV[2]) then
	curr = redis.call('INCR', KEYS[1])
	redis.call('EXPIRE', KEYS[1], ARGV[3])
	redis.call('EXPIRE', KEYS[2], ARGV[3])
	return {1, curr}
end
return {0, curr}
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
		now:    func() int64 { return time.Now().Unix() },
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
	now := r.now()
	windowSec := int64(r.window.Seconds())
	tick := now / windowSec
	windowEnd := (tick + 1) * windowSec
	overlap := 1 - float64(now-tick*windowSec)/float64(windowSec)

	res, err := slidingWindowScript.Run(
		context.Background(), r.client,
		[]string{
			r.prefix + key + ":" + strconv.FormatInt(tick, 10),
			r.prefix + key + ":" + strconv.FormatInt(tick-1, 10),
		},
		overlap, r.limit, 2*windowSec,
	).Slice()
	if err != nil {
		return r.fallback(windowEnd, now)
	}

	allowed := res[0].(int64) == 1
	curr := res[1].(int64)
	if allowed {
		return Decision{Allowed: true, Remaining: r.limit - int(curr), ResetAt: windowEnd, RetryAfter: 0}
	}
	return Decision{Allowed: false, Remaining: 0, ResetAt: windowEnd, RetryAfter: int(windowEnd - now)}
}
