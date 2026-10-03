package ratelimit

import (
	"container/list"
	"math"
	"sync"
	"time"
)

type item struct {
	key string
	val []byte
	exp int64
}

// MemStore is a bounded in-memory key/value store with per-key TTL and
// least-recently-used eviction. The front of the order list is the most
// recently used entry. Every operation is O(1), so a flood of distinct keys
// can neither grow memory past maxKeys nor slow lookups down.
//
// It is local to one process. Multi-instance deployments must use the
// Redis-backed limiters, whose state lives in Redis behind atomic Lua scripts.
type MemStore struct {
	mu      sync.Mutex
	data    map[string]*list.Element
	order   *list.List
	maxKeys int
	now     func() int64
}

// NewMemStore returns a store holding at most maxKeys entries (defaultMaxKeys
// when maxKeys <= 0). now supplies the clock in Unix seconds and defaults to
// the wall clock; limiters pass their own so TTLs follow one injectable clock.
func NewMemStore(maxKeys int, now func() int64) *MemStore {
	if maxKeys <= 0 {
		maxKeys = defaultMaxKeys
	}
	if now == nil {
		now = func() int64 { return time.Now().Unix() }
	}
	return &MemStore{
		data:    make(map[string]*list.Element),
		order:   list.New(),
		maxKeys: maxKeys,
		now:     now,
	}
}

func (s *MemStore) expired(it *item) bool {
	return it.exp > 0 && s.now() >= it.exp
}

func (s *MemStore) remove(el *list.Element) {
	delete(s.data, el.Value.(*item).key)
	s.order.Remove(el)
}

func (s *MemStore) Get(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	el, ok := s.data[key]
	if !ok {
		return nil, false
	}
	it := el.Value.(*item)
	if s.expired(it) {
		s.remove(el)
		return nil, false
	}
	s.order.MoveToFront(el)
	return it.val, true
}

// Set stores val under key. A ttl of zero or less means the entry never
// expires; otherwise it is rounded up to whole seconds so a sub-second TTL is
// never silently treated as already expired. When the store is full, the
// least recently used entry is evicted to make room.
func (s *MemStore) Set(key string, val []byte, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var exp int64
	if ttl > 0 {
		exp = s.now() + int64(math.Ceil(ttl.Seconds()))
	}

	if el, ok := s.data[key]; ok {
		it := el.Value.(*item)
		it.val, it.exp = val, exp
		s.order.MoveToFront(el)
		return
	}

	for len(s.data) >= s.maxKeys {
		s.remove(s.order.Back())
	}
	s.data[key] = s.order.PushFront(&item{key: key, val: val, exp: exp})
}

func (s *MemStore) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if el, ok := s.data[key]; ok {
		s.remove(el)
	}
}

// Len counts stored entries, including expired ones not yet collected.
func (s *MemStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.data)
}

// SetMaxKeys changes the capacity, evicting least recently used entries
// immediately if the store is now over it.
func (s *MemStore) SetMaxKeys(n int) {
	if n < 1 {
		n = 1
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.maxKeys = n
	for len(s.data) > n {
		s.remove(s.order.Back())
	}
}
