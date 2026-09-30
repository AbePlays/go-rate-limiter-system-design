package ratelimit

import (
	"net/http"
	"strings"
)

func Key(r *http.Request) (key string, identified bool) {
	// your turn: read, trim, decide
	apiKey := strings.TrimSpace(r.Header.Get("api_key"))
	if apiKey != "" {
		return apiKey, true
	}

	return ClientIP(r), false
}
