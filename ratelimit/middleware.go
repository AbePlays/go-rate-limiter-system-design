package ratelimit

import (
	"maps"
	"net/http"
)

type Limiter interface {
	Allow(key string) Decision
	Limit() int
}

func Middleware(limiter Limiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d := limiter.Allow(ClientIP(r))

		maps.Copy(w.Header(), Headers(limiter.Limit(), d))
		if !d.Allowed {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
