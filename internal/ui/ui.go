package ui

import (
	"embed"
	"html/template"
	"net/http"
	"time"

	"github.com/AbePlays/go-rate-limiter-system-design/ratelimit"
)

//go:embed *.html
var files embed.FS

var layout = template.Must(template.ParseFS(files, "layout.html"))
var demoTmpl = template.Must(template.Must(layout.Clone()).ParseFS(files, "demo.html"))
var aboutTmpl = template.Must(template.Must(layout.Clone()).ParseFS(files, "about.html"))

type Handler struct {
	set      *ratelimit.PolicySet
	policies []string
}

func New(set *ratelimit.PolicySet, policies []string) *Handler {
	return &Handler{set: set, policies: policies}
}

type policyInfo struct {
	Name  string
	Limit int
	Algo  string
}

var policyAlgos = map[string]string{
	"redirects": "token bucket",
	"login":     "sliding window",
	"api":       "sliding window",
}

func (h *Handler) infos() []policyInfo {
	infos := make([]policyInfo, 0, len(h.policies))
	for _, name := range h.policies {
		limit, _ := h.set.Limit(name)
		infos = append(infos, policyInfo{name, limit, policyAlgos[name]})
	}
	return infos
}

func (h *Handler) Demo(w http.ResponseWriter, r *http.Request) {
	render(demoTmpl, w, map[string]any{
		"Title":       "Demo",
		"Policies":    h.policies,
		"PolicyInfos": h.infos(),
	})
}

func (h *Handler) About(w http.ResponseWriter, r *http.Request) {
	render(aboutTmpl, w, map[string]any{"Title": "About"})
}

type outcome struct {
	Allowed   bool
	Remaining int
	Reset     int64
	ResetIn   int64
	Wait      int
}

func (h *Handler) Request(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	policy := r.FormValue("policy")
	key := r.FormValue("key")
	if key == "" {
		key = ratelimit.ClientIP(r)
	}
	apiKey := r.FormValue("api_key")
	effective := key
	if apiKey != "" {
		effective = apiKey
	}

	n := 1
	if r.FormValue("action") == "burst" {
		n = 20
	}

	outs := make([]outcome, 0, n)
	nowUnix := time.Now().Unix()
	var limit int
	for range n {
		d, lim, ok := h.set.Allow(policy, effective, ratelimit.ClientIP(r))
		if !ok {
			http.Error(w, "unknown policy", http.StatusBadRequest)
			return
		}
		limit = lim
		resetIn := d.ResetAt - nowUnix
		if resetIn < 0 {
			resetIn = 0
		}
		outs = append(outs, outcome{d.Allowed, d.Remaining, d.ResetAt, resetIn, d.RetryAfter})
	}

	admitted := 0
	for _, o := range outs {
		if o.Allowed {
			admitted++
		}
	}

	data := map[string]any{
		"Title":       "Demo",
		"Policies":    h.policies,
		"PolicyInfos": h.infos(),
		"Policy":      policy,
		"Key":         key,
		"ApiKey":      apiKey,
		"Outcomes":    outs,
		"Admitted":    admitted,
		"Total":       n,
		"SimNote":     n > 1,
	}
	if n == 1 {
		data["Single"] = outs[0]
		data["Limit"] = limit
	}
	render(demoTmpl, w, data)
}

func render(t *template.Template, w http.ResponseWriter, data map[string]any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "render", http.StatusInternalServerError)
	}
}
