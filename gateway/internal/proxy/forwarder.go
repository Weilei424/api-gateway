package proxy

import (
	"bytes"
	"context"
	"errors"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"gateway/internal/config"

	"go.uber.org/zap"
)

type Forwarder struct {
	target *url.URL
	cb     *CircuitBreaker
	retry  config.RetryConfig
	logger *zap.Logger
}

func NewForwarder(target *url.URL, cb *CircuitBreaker, retry config.RetryConfig, logger *zap.Logger) *Forwarder {
	return &Forwarder{target: target, cb: cb, retry: retry, logger: logger}
}

// BuildForwarders constructs one Forwarder per unique upstream in routes.
func BuildForwarders(routes []config.Route, retryCfg config.RetryConfig, cbCfg config.CircuitBreakerConfig, logger *zap.Logger) map[string]*Forwarder {
	forwarders := make(map[string]*Forwarder)
	for _, route := range routes {
		if _, exists := forwarders[route.Upstream]; exists {
			continue
		}
		target, _ := url.Parse(route.Upstream) // already validated by config
		cb := NewCircuitBreaker(
			cbCfg.FailureThreshold,
			time.Duration(cbCfg.RecoveryTimeoutMs)*time.Millisecond,
		)
		forwarders[route.Upstream] = NewForwarder(target, cb, retryCfg, logger)
	}
	return forwarders
}

// Do forwards the request to the upstream, retrying on eligible failures and recording outcomes in the circuit breaker.
func (f *Forwarder) Do(w http.ResponseWriter, r *http.Request) {
	if !f.cb.Allow() {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}

	maxAttempts := f.retry.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	isIdempotent := r.Method == http.MethodGet || r.Method == http.MethodHead

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			base := time.Duration(f.retry.BaseDelayMs) * time.Millisecond
			if base == 0 {
				base = 100 * time.Millisecond
			}
			delay := base * (1 << uint(attempt-1))
			jitter := time.Duration(rand.Int63n(int64(base)))
			select {
			case <-r.Context().Done():
				http.Error(w, "service unavailable", http.StatusServiceUnavailable)
				return
			case <-time.After(delay + jitter):
			}
		}

		isLast := attempt == maxAttempts-1
		var netErr error

		rp := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = f.target.Scheme
				req.URL.Host = f.target.Host
				req.Host = f.target.Host
			},
			ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
				netErr = err
				if errors.Is(err, context.DeadlineExceeded) {
					w.WriteHeader(http.StatusGatewayTimeout)
				} else {
					w.WriteHeader(http.StatusBadGateway)
				}
			},
		}

		if isLast {
			// Stream directly on the last attempt — no in-memory buffering.
			sc := &statusCapture{ResponseWriter: w}
			rp.ServeHTTP(sc, r)
			if netErr != nil || sc.code >= 500 {
				f.cb.RecordFailure()
			} else {
				f.cb.RecordSuccess()
			}
			return
		}

		// Buffer on non-final attempts so we can inspect status and retry.
		buf := newResponseBuffer()
		rp.ServeHTTP(buf, r)

		failed := netErr != nil || buf.statusCode() >= 500
		shouldRetry := failed && (netErr != nil || isIdempotent) && !errors.Is(netErr, context.DeadlineExceeded)

		if shouldRetry {
			f.cb.RecordFailure()
			f.logger.Warn("upstream error, retrying",
				zap.Int("attempt", attempt+1),
				zap.Int("status", buf.statusCode()),
				zap.Error(netErr),
			)
			continue
		}

		buf.flush(w)
		if failed {
			f.cb.RecordFailure()
		} else {
			f.cb.RecordSuccess()
		}
		return
	}
}

// statusCapture wraps an http.ResponseWriter to record the status code while streaming through.
type statusCapture struct {
	http.ResponseWriter
	code int
}

func (sc *statusCapture) WriteHeader(code int) {
	sc.code = code
	sc.ResponseWriter.WriteHeader(code)
}

func (sc *statusCapture) Write(b []byte) (int, error) {
	if sc.code == 0 {
		sc.code = http.StatusOK
	}
	return sc.ResponseWriter.Write(b)
}

// responseBuffer is a minimal http.ResponseWriter that captures the response for retry inspection.
type responseBuffer struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func newResponseBuffer() *responseBuffer {
	return &responseBuffer{header: make(http.Header)}
}

func (rb *responseBuffer) Header() http.Header { return rb.header }

func (rb *responseBuffer) WriteHeader(code int) {
	if rb.code == 0 {
		rb.code = code
	}
}

func (rb *responseBuffer) Write(b []byte) (int, error) {
	if rb.code == 0 {
		rb.code = http.StatusOK
	}
	return rb.body.Write(b)
}

func (rb *responseBuffer) statusCode() int {
	if rb.code == 0 {
		return http.StatusOK
	}
	return rb.code
}

func (rb *responseBuffer) flush(w http.ResponseWriter) {
	for k, vs := range rb.header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(rb.statusCode())
	_, _ = w.Write(rb.body.Bytes())
}
