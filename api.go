package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	summaryTTL   = 3 * time.Second
	maxRange     = 365 * 24 * time.Hour
	maxLimit     = 500
	maxPageSize  = 500
	defaultRange = 24 * time.Hour
)

type API struct {
	store    *Store
	ingest   *Ingester
	poller   *Poller // nil when the REST API is not configured
	resolver *net.Resolver
	cfg      Config
	log      *slog.Logger
	started  time.Time

	cacheMu sync.Mutex
	cache   map[string]cachedSummary
}

type cachedSummary struct {
	at   time.Time
	data []byte
}

func NewAPI(cfg Config, store *Store, ingest *Ingester, poller *Poller, log *slog.Logger) *API {
	return &API{
		store:    store,
		ingest:   ingest,
		poller:   poller,
		resolver: newResolver(cfg.DNSServer),
		cfg:      cfg,
		log:      log,
		started:  time.Now(),
		cache:    map[string]cachedSummary{},
	}
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/summary", a.summary)
	mux.HandleFunc("GET /api/timeline", a.timeline)
	mux.HandleFunc("GET /api/domains", a.domains)
	mux.HandleFunc("GET /api/clients", a.clients)
	mux.HandleFunc("GET /api/queries", a.queries)
	mux.HandleFunc("GET /api/export.csv", a.export)
	mux.HandleFunc("GET /api/resolve", a.resolve)
	mux.HandleFunc("GET /api/status", a.status)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) { httpError(w, "not found", http.StatusNotFound) })
	if a.cfg.WebDir != "" {
		mux.Handle("/", staticHandler(a.cfg.WebDir))
	}
	return a.logRequests(mux)
}

func (a *API) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			a.log.Debug("request", "path", r.URL.RequestURI(), "took", time.Since(start))
		}
	})
}

// webFS hides directory listings; only directories with an index.html resolve.
type webFS struct{ http.FileSystem }

func (w webFS) Open(name string) (http.File, error) {
	f, err := w.FileSystem.Open(name)
	if err != nil {
		return nil, err
	}
	if fi, err := f.Stat(); err == nil && fi.IsDir() {
		index, err := w.FileSystem.Open(path.Join(name, "index.html"))
		if err != nil {
			f.Close()
			return nil, fs.ErrNotExist
		}
		index.Close()
	}
	return f, nil
}

// staticHandler serves the exported dashboard; hashed Next.js assets are immutable.
func staticHandler(dir string) http.Handler {
	fs := http.FileServer(webFS{http.Dir(dir)})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/_next/static/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		fs.ServeHTTP(w, r)
	})
}

func (a *API) summary(w http.ResponseWriter, r *http.Request) {
	window, err := rangeParam(r)
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}
	limit := intParam(r, "limit", 10, maxLimit)
	key := window.String() + "/" + strconv.Itoa(limit)

	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	if c, ok := a.cache[key]; ok && time.Since(c.at) < summaryTTL {
		writeRaw(w, c.data)
		return
	}
	s, err := a.store.Summary(r.Context(), window, limit)
	if err != nil {
		a.fail(w, "summary", err)
		return
	}
	data, err := json.Marshal(s)
	if err != nil {
		a.fail(w, "summary", err)
		return
	}
	a.cache[key] = cachedSummary{at: time.Now(), data: data}
	writeRaw(w, data)
}

func (a *API) timeline(w http.ResponseWriter, r *http.Request) {
	f, window, err := filterParams(r)
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}
	bucket := bucketFor(window)
	buckets, err := a.store.Timeline(r.Context(), f, bucket, time.Now())
	if err != nil {
		a.fail(w, "timeline", err)
		return
	}
	writeJSON(w, map[string]any{"bucket_secs": bucket, "since": f.Since.Unix(), "buckets": buckets})
}

func (a *API) domains(w http.ResponseWriter, r *http.Request) {
	f, _, err := filterParams(r)
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}
	out, err := a.store.Domains(r.Context(), f, intParam(r, "limit", 50, maxLimit))
	if err != nil {
		a.fail(w, "domains", err)
		return
	}
	writeJSON(w, out)
}

func (a *API) clients(w http.ResponseWriter, r *http.Request) {
	f, _, err := filterParams(r)
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}
	out, err := a.store.Clients(r.Context(), f, intParam(r, "limit", 50, maxLimit))
	if err != nil {
		a.fail(w, "clients", err)
		return
	}
	writeJSON(w, out)
}

func (a *API) queries(w http.ResponseWriter, r *http.Request) {
	f, _, err := filterParams(r)
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}
	page := intParam(r, "page", 1, 1<<30)
	size := intParam(r, "page_size", 50, maxPageSize)
	items, total, err := a.store.Queries(r.Context(), f, page, size)
	if err != nil {
		a.fail(w, "queries", err)
		return
	}
	writeJSON(w, map[string]any{"items": items, "total": total, "page": page, "page_size": size})
}

func (a *API) export(w http.ResponseWriter, r *http.Request) {
	f, _, err := filterParams(r)
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="dns-queries-`+time.Now().Format("20060102-150405")+`.csv"`)
	if err := a.store.ExportCSV(r.Context(), f, w); err != nil && !errors.Is(err, context.Canceled) {
		a.log.Error("export", "err", err)
	}
}

func (a *API) resolve(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSuffix(strings.TrimSpace(r.URL.Query().Get("name")), ".")
	if name == "" {
		httpError(w, "name is required", http.StatusBadRequest)
		return
	}
	writeJSON(w, resolve(r.Context(), a.resolver, name, r.URL.Query().Get("type")))
}

func (a *API) status(w http.ResponseWriter, r *http.Request) {
	written, dropped := a.store.Stats()
	out := map[string]any{
		"version":        version,
		"uptime_secs":    int64(time.Since(a.started).Seconds()),
		"retention_secs": int64(a.cfg.Retention.Seconds()),
		"ingest":         a.ingest.Stats(),
		"store": map[string]any{
			"written":    written,
			"dropped":    dropped,
			"size_bytes": a.store.Size(),
		},
		"router": nil,
	}
	if a.poller != nil {
		out["router"] = a.poller.Info()
	}
	writeJSON(w, out)
}

func (a *API) fail(w http.ResponseWriter, what string, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	a.log.Error(what, "err", err)
	httpError(w, "database error", http.StatusInternalServerError)
}

func filterParams(r *http.Request) (Filter, time.Duration, error) {
	window, err := rangeParam(r)
	if err != nil {
		return Filter{}, 0, err
	}
	q := r.URL.Query()
	f := Filter{
		Since:  time.Now().Add(-window),
		Client: strings.TrimSpace(q.Get("client")),
		Domain: strings.TrimSpace(q.Get("domain")),
		Search: strings.TrimSpace(q.Get("search")),
		Type:   strings.TrimSpace(q.Get("type")),
		Status: strings.TrimSpace(q.Get("status")),
	}
	return f, window, nil
}

func rangeParam(r *http.Request) (time.Duration, error) {
	s := r.URL.Query().Get("range")
	if s == "" {
		return defaultRange, nil
	}
	d, err := parseDuration(s)
	if err != nil || d <= 0 || d > maxRange {
		return 0, errors.New("invalid range")
	}
	return d, nil
}

func intParam(r *http.Request, name string, def, max int) int {
	s := r.URL.Query().Get(name)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return min(n, max)
}

// parseDuration extends time.ParseDuration with a day suffix ("7d").
func parseDuration(s string) (time.Duration, error) {
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Debug("write response", "err", err)
	}
}

func writeRaw(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func httpError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// getenv returns the value of the first set variable.
func getenv(names ...string) string {
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return ""
}
