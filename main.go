package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/AbePlays/go-rate-limiter-system-design/internal/ui"
	"github.com/AbePlays/go-rate-limiter-system-design/ratelimit"
)

// ipFloorPerMinute is the per-IP backstop applied on top of every policy. It
// must stay above the highest sustained rate of any policy (redirects refills
// at 5/s = 300/min), otherwise the floor silently overrides that policy.
const ipFloorPerMinute = 600

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	hops := 0
	if v := os.Getenv("TRUSTED_PROXY_HOPS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			log.Fatalf("TRUSTED_PROXY_HOPS must be a non-negative integer, got %q", v)
		}
		hops = n
	}
	ratelimit.SetTrustedProxyHops(hops)
	slog.Info("client ip resolution", "trusted_proxy_hops", hops)

	set := ratelimit.NewPolicySet(ratelimit.NewFixedWindow(ipFloorPerMinute, time.Minute))
	set.Add("redirects", ratelimit.NewTokenBucket(20, 5))
	set.Add("login", ratelimit.NewSlidingWindow(5, time.Minute))
	set.Add("api", ratelimit.NewSlidingWindow(100, time.Minute))

	h := ui.New(set, []string{"redirects", "login", "api"})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.Demo)
	mux.HandleFunc("POST /{$}", h.Request)
	mux.HandleFunc("GET /about", h.About)

	addr := ":8080"
	log.Printf("demo on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
