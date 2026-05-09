package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gateway/internal/config"
	"gateway/internal/health"
	"gateway/internal/proxy"
	"gateway/internal/routing"

	"go.uber.org/zap"
)

func intPtr(v int) *int { return &v }

func minimalProxy(routes []config.Route) *proxy.Proxy {
	router := routing.New(routes)
	upstreams := make([]string, len(routes))
	for i, r := range routes {
		upstreams[i] = r.Upstream
	}
	checker := health.NewChecker(upstreams, time.Hour, zap.NewNop())
	forwarders := proxy.BuildForwarders(routes,
		config.RetryConfig{MaxAttempts: intPtr(1)},
		config.CircuitBreakerConfig{FailureThreshold: intPtr(100), RecoveryTimeoutMs: 30000},
		zap.NewNop(),
	)
	return proxy.New(router, zap.NewNop(), checker, forwarders)
}

func TestProxy_ForwardsRequest(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123" {
			t.Errorf("expected upstream to receive path /users/123, got %s", r.URL.Path)
		}
		if r.URL.RawQuery != "active=true" {
			t.Errorf("expected upstream to receive query active=true, got %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("upstream response"))
	}))
	defer upstream.Close()

	routes := []config.Route{{Path: "/users", Upstream: upstream.URL}}
	p := minimalProxy(routes)

	req := httptest.NewRequest(http.MethodGet, "/users/123?active=true", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "upstream response" {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestProxy_UnknownPathReturns404(t *testing.T) {
	routes := []config.Route{{Path: "/users", Upstream: "http://localhost:9999"}}
	p := minimalProxy(routes)

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestProxy_UnreachableUpstreamReturns502(t *testing.T) {
	routes := []config.Route{{Path: "/users", Upstream: "http://127.0.0.1:1"}}
	p := minimalProxy(routes)

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestProxy_UnhealthyUpstreamReturns503(t *testing.T) {
	routes := []config.Route{{Path: "/users", Upstream: "http://127.0.0.1:1"}}
	router := routing.New(routes)
	checker := health.NewChecker([]string{"http://127.0.0.1:1"}, time.Hour, zap.NewNop())
	checker.PollAll() // mark as unhealthy (connection refused)
	forwarders := proxy.BuildForwarders(routes,
		config.RetryConfig{MaxAttempts: intPtr(1)},
		config.CircuitBreakerConfig{FailureThreshold: intPtr(100), RecoveryTimeoutMs: 30000},
		zap.NewNop(),
	)
	p := proxy.New(router, zap.NewNop(), checker, forwarders)

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for unhealthy upstream, got %d", rec.Code)
	}
}
