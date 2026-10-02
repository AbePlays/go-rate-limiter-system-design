package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
	prefix string
}

func NewRedisStore(addr string) *RedisStore {
	return &RedisStore{
		client: redis.NewClient(&redis.Options{Addr: addr}),
		prefix: "rl:",
	}
}

func (s *RedisStore) ctx() context.Context { return context.Background() }

func (s *RedisStore) key(k string) string { return s.prefix + k }

func (s *RedisStore) Get(key string) ([]byte, bool) {
	raw, err := s.client.Get(s.ctx(), s.key(key)).Bytes()
	if err != nil {
		return nil, false
	}
	return raw, true
}

func (s *RedisStore) Set(key string, val []byte, ttl time.Duration) {
	s.client.Set(s.ctx(), s.key(key), val, ttl)
}

func (s *RedisStore) Delete(key string) {
	s.client.Del(s.ctx(), s.key(key))
}

func (s *RedisStore) Len() int {
	n := 0
	s.each(func(string) { n++ })
	return n
}

func (s *RedisStore) Keys() []string {
	var out []string
	s.each(func(k string) { out = append(out, k) })
	return out
}

func (s *RedisStore) each(fn func(string)) {
	var cursor uint64
	for {
		ks, next, err := s.client.Scan(s.ctx(), cursor, s.prefix+"*", 100).Result()
		if err != nil {
			return
		}
		for _, k := range ks {
			fn(k[len(s.prefix):])
		}
		cursor = next
		if cursor == 0 {
			return
		}
	}
}
