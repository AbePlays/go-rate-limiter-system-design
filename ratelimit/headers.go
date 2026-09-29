package ratelimit

import (
	"net/http"
	"strconv"
)

func Headers(limit int, d Decision) http.Header {
	h := make(http.Header)
	h.Set("X-RateLimit-Limit", strconv.Itoa(limit))
	h.Set("X-RateLimit-Remaining", strconv.Itoa(d.Remaining))
	h.Set("X-RateLimit-Reset", strconv.FormatInt(d.ResetAt, 10))
	if !d.Allowed {
		h.Set("Retry-After", strconv.Itoa(d.RetryAfter))
	}
	return h
}
