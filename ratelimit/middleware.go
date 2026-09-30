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

func PolicyMiddleware(set *PolicySet, name string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, _ := Key(r)
		d, limit, ok := set.Allow(name, key, ClientIP(r))
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		maps.Copy(w.Header(), Headers(limit, d))
		if !d.Allowed {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
