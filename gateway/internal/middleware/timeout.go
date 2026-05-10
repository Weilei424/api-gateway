package middleware

import (
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

			// handlerCtx is a plain cancel context passed to the handler goroutine.
			// Because it is NOT the same channel as ctx, the handler goroutine cannot
			// unblock from <-r.Context().Done() until cancelHandler() is called
			// explicitly — which only happens AFTER markTimedOut() has set timedOut=true.
			// This eliminates the race where the handler writes between ctx.Done()
			// firing and timedOut being set.
			// deadlineCtx wraps handlerCtx but reports the deadline from ctx so
			// that downstream code (e.g. proxy transports) can observe the request
			// deadline via r.Context().Deadline() without being cancelled when the
			// timer fires — cancellation is gated through cancelHandler().
			deadline, _ := ctx.Deadline()
			handlerCtx, cancelHandler := context.WithCancel(r.Context())
			defer cancelHandler()

			tw := &timeoutWriter{ResponseWriter: w}
			done := make(chan struct{})
			panicChan := make(chan interface{}, 1)

			go func() {
				defer func() {
					if p := recover(); p != nil {
						panicChan <- p
					}
				}()
				next.ServeHTTP(tw, r.WithContext(deadlineCtx{handlerCtx, deadline}))
				close(done)
			}()

			select {
			case p := <-panicChan:
				panic(p)
			case <-done:
				// Handler completed normally; response already streamed to w.
			case <-ctx.Done():
				started := tw.markTimedOut()
				cancelHandler()
				if !started {
					http.Error(w, "gateway timeout", http.StatusGatewayTimeout)
				}
			}
		})
	}
}

// timeoutWriter streams directly to the underlying ResponseWriter while
// coordinating with the timeout goroutine via a mutex. No response body is
// buffered; if a timeout fires after headers have already been sent, the
// connection is left for the proxy to close via context cancellation.
type timeoutWriter struct {
	http.ResponseWriter
	mu       sync.Mutex
	started  bool
	timedOut bool
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut {
		return
	}
	tw.started = true
	tw.ResponseWriter.WriteHeader(code)
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut {
		return 0, http.ErrHandlerTimeout
	}
	tw.started = true
	return tw.ResponseWriter.Write(b)
}

// markTimedOut prevents further writes and returns whether a response had already
// started. Callers should write a 504 only when this returns false.
func (tw *timeoutWriter) markTimedOut() (started bool) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	started = tw.started
	tw.timedOut = true
	return started
}

// Unwrap allows http.ResponseController to traverse the wrapper chain and
// discover optional capabilities (Flusher, Hijacker, etc.) on the underlying
// ResponseWriter.
func (tw *timeoutWriter) Unwrap() http.ResponseWriter {
	return tw.ResponseWriter
}

// Flush implements http.Flusher for callers that type-assert directly.
// The flush is suppressed if the timeout has already fired.
func (tw *timeoutWriter) Flush() {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut {
		return
	}
	if f, ok := tw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// deadlineCtx wraps a cancel-only context but exposes the deadline from the
// parent timeout context. This lets downstream handlers observe the request
// deadline via r.Context().Deadline() while cancellation remains gated
// through cancelHandler (called only after markTimedOut sets timedOut=true).
type deadlineCtx struct {
	context.Context
	deadline time.Time
}

func (d deadlineCtx) Deadline() (time.Time, bool) { return d.deadline, true }
