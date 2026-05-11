package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"gateway/internal/config"

	"go.uber.org/zap"
)

func intPtr(v int) *int { return &v }

func newTestForwarder(t *testing.T, targetURL string, maxAttempts int) *Forwarder {
	t.Helper()
	target, err := url.Parse(targetURL)
	if err != nil {
		t.Fatalf("parse target: %v", err)
	}
	cb := NewCircuitBreaker(100, time.Minute)
	return NewForwarder(target, cb, config.RetryConfig{MaxAttempts: intPtr(maxAttempts), BaseDelayMs: 10}, zap.NewNop())
}

func TestForwarder_ForwardsSuccessfulRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	}))
	defer srv.Close()

	f := newTestForwarder(t, srv.URL, 1)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	f.Do(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "hello" {
		t.Errorf("unexpected body: %q", rec.Body.String())
	}
}

func TestForwarder_NetworkErrorReturns502(t *testing.T) {
	f := newTestForwarder(t, "http://127.0.0.1:1", 1)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	f.Do(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", rec.Code)
	}
}

func TestForwarder_RetriesGETOnNetworkError(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// Close the connection to simulate a network error.
			hj, ok := w.(http.Hijacker)
			if ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := newTestForwarder(t, srv.URL, 3)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	f.Do(rec, req)

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 after retries, got %d", rec.Code)
	}
}

func TestForwarder_DoesNotRetryPOSTOn5xx(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	f := newTestForwarder(t, srv.URL, 3)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	f.Do(rec, req)

	if attempts != 1 {
		t.Errorf("expected exactly 1 attempt for POST, got %d", attempts)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestForwarder_RetriesGETOn503(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := newTestForwarder(t, srv.URL, 3)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	f.Do(rec, req)

	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 after retries, got %d", rec.Code)
	}
}

func TestForwarder_CircuitOpenReturns503(t *testing.T) {
	target, _ := url.Parse("http://127.0.0.1:1")
	cb := NewCircuitBreaker(1, time.Minute)
	f := NewForwarder(target, cb, config.RetryConfig{MaxAttempts: intPtr(1)}, zap.NewNop())

	// Trip the circuit.
	rec1 := httptest.NewRecorder()
	f.Do(rec1, httptest.NewRequest(http.MethodGet, "/", nil))
	// Now the circuit is open.

	rec2 := httptest.NewRecorder()
	f.Do(rec2, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec2.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 from open circuit, got %d", rec2.Code)
	}
}

func TestForwarder_OnlyFinalOutcomeUpdatesCircuitBreaker(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	target, _ := url.Parse(srv.URL)
	// threshold=2: circuit should only open after 2 request-level failures
	cb := NewCircuitBreaker(2, time.Minute)
	// 2 attempts per request: one intermediate, one final
	f := NewForwarder(target, cb, config.RetryConfig{MaxAttempts: intPtr(2), BaseDelayMs: 0}, zap.NewNop())

	rec := httptest.NewRecorder()
	f.Do(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	// After one request-level failure (threshold=2), circuit must still be closed.
	// Bug: old code records RecordFailure per attempt, so 2 attempts = threshold reached = open.
	if !cb.Allow() {
		t.Fatal("circuit should be closed after one request-level failure when threshold=2")
	}
}

func TestForwarder_ContextCancelInHalfOpenRecordsFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	target, _ := url.Parse(srv.URL)
	cb := NewCircuitBreaker(1, 20*time.Millisecond)

	// Trip the circuit instantly using MaxAttempts:1 — no retry backoff.
	trip := NewForwarder(target, cb, config.RetryConfig{MaxAttempts: intPtr(1)}, zap.NewNop())
	trip.Do(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	// Wait for recovery timeout to elapse.
	time.Sleep(30 * time.Millisecond)

	// The next Allow() transitions Open → HalfOpen. The admitted GET fails with 500,
	// enters a 2-second retry backoff, and the context is cancelled before it completes.
	f := NewForwarder(target, cb, config.RetryConfig{MaxAttempts: intPtr(2), BaseDelayMs: 2000}, zap.NewNop())
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	f.Do(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))

	// Circuit must be open again, not permanently stuck in HalfOpen.
	if cb.Allow() {
		t.Fatal("circuit should be open after context-cancelled half-open attempt, but Allow() returned true")
	}
}

func TestForwarder_TimeoutReturns504(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(500 * time.Millisecond):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()

	f := newTestForwarder(t, srv.URL, 1)
	rec := httptest.NewRecorder()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	f.Do(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("expected 504, got %d", rec.Code)
	}
}
