package ratelimit

// defaultMaxKeys bounds how many distinct keys each in-memory limiter tracks
// before its store starts evicting the least recently used one.
const defaultMaxKeys = 10000
