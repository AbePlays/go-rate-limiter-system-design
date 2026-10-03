package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/AbePlays/go-rate-limiter-system-design/internal/ui"
	"github.com/AbePlays/go-rate-limiter-system-design/ratelimit"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	set := ratelimit.NewPolicySet(ratelimit.NewFixedWindow(30, time.Minute))
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
