package ratelimit

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var fixedWindowScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then
	redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return count
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
		now:    func() int64 { return time.Now().Unix() },
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
	now := r.now()
	windowSec := int64(r.window.Seconds())
	tick := now / windowSec
	windowEnd := (tick + 1) * windowSec

	count, err := fixedWindowScript.Run(
		context.Background(), r.client,
		[]string{r.prefix + key + ":" + strconv.FormatInt(tick, 10)},
		windowSec,
	).Int64()
	if err != nil {
		return r.fallback(windowEnd, now)
	}

	if count <= int64(r.limit) {
		return Decision{Allowed: true, Remaining: r.limit - int(count), ResetAt: windowEnd, RetryAfter: 0}
	}
	return Decision{Allowed: false, Remaining: 0, ResetAt: windowEnd, RetryAfter: int(windowEnd - now)}
}
