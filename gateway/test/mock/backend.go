package mock

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"time"
)

// HandlerConfig controls how the mock backend responds.
type HandlerConfig struct {
	StatusCode int           // HTTP status to return (default 200)
	Body       string        // response body (default "ok")
	Delay      time.Duration // pause before responding
	FailFirst  int           // return 500 for the first N requests, then use StatusCode
}

// Backend wraps an httptest.Server with configurable per-request behavior.
type Backend struct {
	Server *httptest.Server
	mu     sync.Mutex
	cfg    HandlerConfig
	count  atomic.Int64
	fails  atomic.Int64
}

// New starts a Backend with the given config.
func New(cfg HandlerConfig) *Backend {
	if cfg.StatusCode == 0 {
		cfg.StatusCode = http.StatusOK
	}
	if cfg.Body == "" {
		cfg.Body = "ok"
	}
	b := &Backend{cfg: cfg}
	b.Server = httptest.NewServer(http.HandlerFunc(b.handle))
	return b
}

func (b *Backend) handle(w http.ResponseWriter, r *http.Request) {
	b.count.Add(1)

	b.mu.Lock()
	cfg := b.cfg
	b.mu.Unlock()

	if cfg.Delay > 0 {
		time.Sleep(cfg.Delay)
	}

	n := b.fails.Load()
	if cfg.FailFirst > 0 && n < int64(cfg.FailFirst) {
		b.fails.Add(1)
		http.Error(w, "injected failure", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(cfg.StatusCode)
	_, _ = w.Write([]byte(cfg.Body))
}

// URL returns the base URL of the backend server.
func (b *Backend) URL() string { return b.Server.URL }

// Close shuts down the backend server.
func (b *Backend) Close() { b.Server.Close() }

// RequestCount returns the total number of requests received.
func (b *Backend) RequestCount() int { return int(b.count.Load()) }

// SetHandler swaps the handler config for subsequent requests.
func (b *Backend) SetHandler(cfg HandlerConfig) {
	if cfg.StatusCode == 0 {
		cfg.StatusCode = http.StatusOK
	}
	if cfg.Body == "" {
		cfg.Body = "ok"
	}
	b.mu.Lock()
	b.cfg = cfg
	b.fails.Store(0)
	b.mu.Unlock()
}
