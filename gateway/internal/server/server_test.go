package server

import (
	"context"
	"net"
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

func TestServer_ShutdownDrainsInFlightRequests(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()

	routes := []config.Route{{Path: "/", Upstream: slow.URL}}
	srv := minimalServer(t, routes, zap.NewNop())

	// Override the server address to use a random free port.
	srv.httpServer.Addr = "127.0.0.1:0"
	ln, err := net.Listen("tcp", srv.httpServer.Addr)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	started := make(chan struct{})
	go func() {
		close(started)
		_ = srv.httpServer.Serve(ln)
	}()
	<-started

	addr := ln.Addr().String()

	done := make(chan int, 1)
	go func() {
		resp, err := http.Get("http://" + addr + "/")
		if err != nil {
			done <- 0
			return
		}
		resp.Body.Close()
		done <- resp.StatusCode
	}()

	time.Sleep(20 * time.Millisecond) // let request reach handler

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	code := <-done
	if code != http.StatusOK {
		t.Fatalf("expected in-flight request to complete with 200, got %d", code)
	}
}
