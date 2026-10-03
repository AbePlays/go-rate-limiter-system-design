package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
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
			slog.Error("bad TRUSTED_PROXY_HOPS", "value", v)
			os.Exit(1)
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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("demo listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("shutdown failed", "err", err)
	}
}
