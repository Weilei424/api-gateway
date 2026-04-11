package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gateway/internal/middleware"
)

func TestRequestID_PreservesIncomingHeader(t *testing.T) {
	var got string
	handler := middleware.RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = middleware.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "abc-123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got != "abc-123" {
		t.Fatalf("expected context request ID %q, got %q", "abc-123", got)
	}
	if rec.Header().Get("X-Request-ID") != "abc-123" {
		t.Fatalf("expected response header to preserve request ID")
	}
}

func TestRequestID_GeneratesWhenMissing(t *testing.T) {
	var got string
	handler := middleware.RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = middleware.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got == "" {
		t.Fatal("expected generated request ID in context")
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected generated request ID in response header")
	}
}
