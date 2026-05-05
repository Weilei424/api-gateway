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

func newTestForwarder(t *testing.T, targetURL string, maxAttempts int) *Forwarder {
	t.Helper()
	target, err := url.Parse(targetURL)
	if err != nil {
		t.Fatalf("parse target: %v", err)
	}
	cb := NewCircuitBreaker(100, time.Minute)
	return NewForwarder(target, cb, config.RetryConfig{MaxAttempts: maxAttempts, BaseDelayMs: 10}, zap.NewNop())
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
	f := NewForwarder(target, cb, config.RetryConfig{MaxAttempts: 1}, zap.NewNop())

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
