package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gateway/internal/config"
	"gateway/internal/observability"
	"gateway/internal/routing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestServer_HealthzReturnsOK(t *testing.T) {
	router := routing.New([]config.Route{{Path: "/users", Upstream: "http://127.0.0.1:1"}})
	metrics, err := observability.NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics returned error: %v", err)
	}

	srv := New(8080, router, zap.NewNop(), metrics)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	srv.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestServer_MetricsEndpointReturnsPrometheusOutput(t *testing.T) {
	router := routing.New([]config.Route{{Path: "/users", Upstream: "http://127.0.0.1:1"}})
	metrics, err := observability.NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics returned error: %v", err)
	}

	srv := New(8080, router, zap.NewNop(), metrics)

	requestRec := httptest.NewRecorder()
	requestReq := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	srv.httpServer.Handler.ServeHTTP(requestRec, requestReq)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	srv.httpServer.Handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(body, "gateway_requests_total") {
		t.Fatal("expected request counter in metrics output")
	}
	if !strings.Contains(body, "gateway_request_duration_seconds") {
		t.Fatal("expected duration histogram in metrics output")
	}
}

func TestServer_ProxyPreservesRequestIDResponseHeader(t *testing.T) {
	router := routing.New([]config.Route{{Path: "/users", Upstream: "http://127.0.0.1:1"}})
	metrics, err := observability.NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics returned error: %v", err)
	}

	srv := New(8080, router, zap.NewNop(), metrics)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	req.Header.Set("X-Request-ID", "req-123")
	srv.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
	if rec.Header().Get("X-Request-ID") != "req-123" {
		t.Fatalf("expected response header to preserve request ID, got %q", rec.Header().Get("X-Request-ID"))
	}
}

func TestServer_ProxyLogsMatchedUpstream(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)
	router := routing.New([]config.Route{{Path: "/users", Upstream: "http://127.0.0.1:1"}})
	metrics, err := observability.NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics returned error: %v", err)
	}

	srv := New(8080, router, logger, metrics)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	srv.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}

	for _, entry := range logs.All() {
		if entry.Message != "request completed" {
			continue
		}

		fields := entry.ContextMap()
		if fields["upstream"] != "http://127.0.0.1:1" {
			t.Fatalf("expected upstream field, got %v", fields["upstream"])
		}

		return
	}

	t.Fatal("expected request completed log entry")
}
