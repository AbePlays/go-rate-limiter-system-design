package ratelimit

import (
	"net"
	"net/http"
	"strings"
	"sync/atomic"
)

// trustedProxyHops is how many reverse proxies in front of the app append to
// X-Forwarded-For. Zero (the default) means the header is never trusted,
// because anything a client puts in it is attacker-controlled.
var trustedProxyHops atomic.Int32

// SetTrustedProxyHops sets how many trusted proxies sit in front of the app.
// With n > 0, ClientIP reads the n-th entry from the right of X-Forwarded-For,
// which is the address your outermost trusted proxy saw. Entries to its left
// are client-supplied and are ignored.
func SetTrustedProxyHops(n int) {
	if n < 0 {
		n = 0
	}
	trustedProxyHops.Store(int32(n))
}

func ClientIP(r *http.Request) string {
	if hops := int(trustedProxyHops.Load()); hops > 0 {
		if ip, ok := forwardedIP(r.Header.Values("X-Forwarded-For"), hops); ok {
			return ip
		}
	}
	return remoteIP(r)
}

func forwardedIP(values []string, hops int) (string, bool) {
	if len(values) == 0 {
		return "", false
	}
	parts := strings.Split(strings.Join(values, ","), ",")
	if len(parts) < hops {
		return "", false
	}
	ip := net.ParseIP(strings.TrimSpace(parts[len(parts)-hops]))
	if ip == nil {
		return "", false
	}
	return ip.String(), true
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return host
}
