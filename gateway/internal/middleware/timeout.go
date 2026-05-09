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

			tw := &timeoutWriter{ResponseWriter: w}
			done := make(chan struct{})
			panicChan := make(chan interface{}, 1)

			go func() {
				defer func() {
					if p := recover(); p != nil {
						panicChan <- p
					}
				}()
				next.ServeHTTP(tw, r.WithContext(ctx))
				close(done)
			}()

			select {
			case p := <-panicChan:
				panic(p)
			case <-done:
				// Handler completed normally; response already streamed to w.
			case <-ctx.Done():
				if !tw.markTimedOut() {
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

// markTimedOut marks the writer as timed out. Returns true if a response had
// already started (caller must not write a 504 in that case).
func (tw *timeoutWriter) markTimedOut() (started bool) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	started = tw.started
	tw.timedOut = true
	return started
}
