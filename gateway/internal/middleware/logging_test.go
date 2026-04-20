package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gateway/internal/middleware"
	"gateway/internal/proxy"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestLogging_EmitsStructuredRequestLog(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	handler := middleware.RequestID()(middleware.Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})))

	req := httptest.NewRequest(http.MethodPost, "/users/123", nil)
	req.Header.Set("X-Request-ID", "req-1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0].Message != "request completed" {
		t.Fatalf("unexpected message: %q", entries[0].Message)
	}
	fields := entries[0].ContextMap()
	if fields["request_id"] != "req-1" {
		t.Fatalf("expected request_id field")
	}
	if fields["method"] != "POST" {
		t.Fatalf("expected method field")
	}
	if fields["path"] != "/users/123" {
		t.Fatalf("expected path field")
	}
	if fields["status"] != int64(http.StatusCreated) {
		t.Fatalf("expected status field")
	}
}

func TestLogging_IncludesUpstreamFieldWhenPresent(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	handler := middleware.Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	req = req.WithContext(proxy.WithUpstream(req.Context(), "http://users:9002"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	fields := entries[0].ContextMap()
	if fields["upstream"] != "http://users:9002" {
		t.Fatalf("expected upstream field, got %v", fields["upstream"])
	}
}
