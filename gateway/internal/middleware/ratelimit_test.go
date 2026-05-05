package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimit_AllowsRequestsUnderLimit(t *testing.T) {
	handler := RateLimit(10, 5)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:1000"
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}
}

func TestRateLimit_RejectsRequestsOverBurst(t *testing.T) {
	handler := RateLimit(0.001, 2)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	pass := 0
	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:1000"
		handler.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			pass++
		}
	}

	if pass > 2 {
		t.Errorf("expected at most 2 allowed requests (burst=2), got %d", pass)
	}
	if pass == 0 {
		t.Error("expected at least some requests to pass")
	}
}

func TestRateLimit_ZeroRPS_Passthrough(t *testing.T) {
	called := 0
	handler := RateLimit(0, 0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 10; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		handler.ServeHTTP(rec, req)
	}

	if called != 10 {
		t.Errorf("expected all 10 requests to pass with rps=0, called %d", called)
	}
}

func TestRateLimit_PerClientIP(t *testing.T) {
	handler := RateLimit(0.001, 1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First IP: use its burst of 1
	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "1.1.1.1:1000"
	handler.ServeHTTP(rec1, req1)

	// Second IP: separate limiter, should pass
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "2.2.2.2:1000"
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("expected second client IP to have its own limiter, got %d", rec2.Code)
	}
}
