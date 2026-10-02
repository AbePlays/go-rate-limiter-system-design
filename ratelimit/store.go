package ratelimit

import (
	"sync"
	"time"
)

type Store interface {
	Get(key string) ([]byte, bool)
	Set(key string, val []byte, ttl time.Duration)
	Delete(key string)
	Len() int
	Keys() []string
}

type item struct {
	val []byte
	exp int64
}

type MemStore struct {
	mu   sync.Mutex
	data map[string]item
	now  func() int64
}

func NewMemStore() *MemStore {
	return &MemStore{data: make(map[string]item), now: func() int64 { return time.Now().Unix() }}
}

func (s *MemStore) expired(it item) bool {
	return it.exp > 0 && s.now() >= it.exp
}

func (s *MemStore) Get(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.data[key]
	if !ok || s.expired(it) {
		return nil, false
	}
	return it.val, true
}

func (s *MemStore) Set(key string, val []byte, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var exp int64
	if ttl > 0 {
		exp = s.now() + int64(ttl.Seconds())
	}
	s.data[key] = item{val: val, exp: exp}
}

func (s *MemStore) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

func (s *MemStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, it := range s.data {
		if !s.expired(it) {
			n++
		}
	}
	return n
}

func (s *MemStore) Keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]string, 0, len(s.data))
	for k, it := range s.data {
		if !s.expired(it) {
			keys = append(keys, k)
		}
	}
	return keys
}
