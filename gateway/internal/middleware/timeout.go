package middleware

import (
	"bytes"
	"context"
	"net/http"
	"sync"
	"time"
)

func Timeout(d time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		if d == 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()

			tw := &timeoutWriter{}
			done := make(chan struct{})

			go func() {
				defer close(done)
				next.ServeHTTP(tw, r.WithContext(ctx))
			}()

			select {
			case <-done:
				tw.flush(w)
			case <-ctx.Done():
				tw.mu.Lock()
				tw.timedOut = true
				tw.mu.Unlock()
				http.Error(w, "gateway timeout", http.StatusGatewayTimeout)
			}
		})
	}
}

type timeoutWriter struct {
	mu       sync.Mutex
	headers  http.Header
	code     int
	body     bytes.Buffer
	timedOut bool
}

func (tw *timeoutWriter) Header() http.Header {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.headers == nil {
		tw.headers = make(http.Header)
	}
	return tw.headers
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut || tw.code != 0 {
		return
	}
	tw.code = code
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut {
		return 0, http.ErrHandlerTimeout
	}
	if tw.code == 0 {
		tw.code = http.StatusOK
	}
	return tw.body.Write(b)
}

func (tw *timeoutWriter) flush(w http.ResponseWriter) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	for k, vs := range tw.headers {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	code := tw.code
	if code == 0 {
		code = http.StatusOK
	}
	w.WriteHeader(code)
	_, _ = w.Write(tw.body.Bytes())
}
