package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func testRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	c := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := c.FlushDB(context.Background()).Err(); err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	t.Cleanup(func() { c.FlushDB(context.Background()) })
	return c
}

func TestRedisFixedWindowAllowsThenDenies(t *testing.T) {
	r := NewRedisFixedWindow(testRedisClient(t), 2, time.Minute)

	if !r.Allow("ip").Allowed || !r.Allow("ip").Allowed {
		t.Fatal("expected first two allowed")
	}
	if d := r.Allow("ip"); d.Allowed || d.RetryAfter <= 0 {
		t.Fatalf("expected deny with RetryAfter, got %+v", d)
	}
}

func TestRedisFixedWindowConcurrentAdmitsBounded(t *testing.T) {
	r := NewRedisFixedWindow(testRedisClient(t), 50, time.Minute)

	var admitted atomic.Int64
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				if r.Allow("ip").Allowed {
					admitted.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	if n := admitted.Load(); n != 50 {
		t.Fatalf("expected exactly 50 admits under concurrency, got %d", n)
	}
}
