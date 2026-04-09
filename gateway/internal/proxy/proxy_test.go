package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gateway/internal/config"
	"gateway/internal/proxy"
	"gateway/internal/routing"
)

func TestProxy_ForwardsRequest(t *testing.T) {
	// Start a fake upstream that echoes a known response.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123" {
			t.Errorf("expected upstream to receive path /users/123, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("upstream response"))
	}))
	defer upstream.Close()

	routes := []config.Route{
		{Path: "/users", Upstream: upstream.URL},
	}
	router := routing.New(routes)
	p := proxy.New(router)

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
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
	routes := []config.Route{
		{Path: "/users", Upstream: "http://localhost:9999"},
	}
	router := routing.New(routes)
	p := proxy.New(router)

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestProxy_UnreachableUpstreamReturns502(t *testing.T) {
	routes := []config.Route{
		// Port 1 is reserved and will always refuse connections.
		{Path: "/users", Upstream: "http://127.0.0.1:1"},
	}
	router := routing.New(routes)
	p := proxy.New(router)

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}
