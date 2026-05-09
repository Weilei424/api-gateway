package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gateway/internal/config"
	"gateway/internal/health"
	"gateway/internal/observability"
	"gateway/internal/proxy"
	"gateway/internal/routing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func intPtr(v int) *int { return &v }

func minimalServer(t *testing.T, routes []config.Route, logger *zap.Logger) *Server {
	t.Helper()
	router := routing.New(routes)
	upstreams := make([]string, len(routes))
	for i, r := range routes {
		upstreams[i] = r.Upstream
	}
	checker := health.NewChecker(upstreams, time.Hour, logger)
	forwarders := proxy.BuildForwarders(routes,
		config.RetryConfig{MaxAttempts: intPtr(1)},
		config.CircuitBreakerConfig{FailureThreshold: intPtr(100), RecoveryTimeoutMs: 30000},
		logger,
	)
	p := proxy.New(router, logger, checker, forwarders)
	metrics, err := observability.NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics: %v", err)
	}
	return New(8080, p, logger, metrics, config.ServerConfig{Port: 8080})
}

func TestServer_HealthzReturnsOK(t *testing.T) {
	routes := []config.Route{{Path: "/users", Upstream: "http://127.0.0.1:1"}}
	srv := minimalServer(t, routes, zap.NewNop())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	srv.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestServer_MetricsEndpointReturnsPrometheusOutput(t *testing.T) {
	routes := []config.Route{{Path: "/users", Upstream: "http://127.0.0.1:1"}}
	srv := minimalServer(t, routes, zap.NewNop())

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
	routes := []config.Route{{Path: "/users", Upstream: "http://127.0.0.1:1"}}
	srv := minimalServer(t, routes, zap.NewNop())

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
	routes := []config.Route{{Path: "/users", Upstream: "http://127.0.0.1:1"}}
	srv := minimalServer(t, routes, logger)

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
