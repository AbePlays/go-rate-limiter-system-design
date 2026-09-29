package ratelimit

import (
	"net"
	"net/http"
	"strings"
)

func ClientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if ip, _, found := strings.Cut(fwd, ","); found {
			return strings.TrimSpace(ip)
		}
		return strings.TrimSpace(fwd)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
