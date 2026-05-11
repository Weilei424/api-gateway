//go:build integration

package integration

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"gateway/internal/config"
	"gateway/internal/health"
	"gateway/internal/observability"
	"gateway/internal/proxy"
	"gateway/internal/routing"
	"gateway/internal/server"
	"gateway/test/mock"

	"go.uber.org/zap"
)

// testGateway holds a running gateway instance for use in integration tests.
type testGateway struct {
	BaseURL string
	srv     *server.Server
	checker *health.Checker
}

// freePort finds an available TCP port by binding to :0.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

// intPtr returns a pointer to v.
func intPtr(v int) *int { return &v }

// defaultCfg builds a gateway config suitable for integration tests.
func defaultCfg(port int, upstreamURL string) *config.Config {
	one := 1
	return &config.Config{
		Server: config.ServerConfig{
			Port:              port,
			TimeoutMs:         5000,
			ShutdownTimeoutMs: 5000,
			RateLimit:         config.RateLimitConfig{RequestsPerSecond: 10000, Burst: 10000},
			HealthCheck:       config.HealthCheckConfig{IntervalMs: 60000},
			CircuitBreaker:    config.CircuitBreakerConfig{FailureThreshold: intPtr(100), RecoveryTimeoutMs: 60000},
			Retry:             config.RetryConfig{MaxAttempts: &one, BaseDelayMs: 0},
		},
		Routes: []config.Route{{Path: "/test", Upstream: upstreamURL}},
	}
}

// newTestGateway builds and starts the full gateway stack in-process.
// The health checker is NOT started — call gw.checker.Start(ctx) explicitly
// in tests that need health-check behavior. All other tests get upstreams
// that are always considered healthy (the checker's initial state).
func newTestGateway(t *testing.T, cfg *config.Config) *testGateway {
	t.Helper()

	logger := zap.NewNop()
	metrics, err := observability.NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics: %v", err)
	}

	upstreams := uniqueUpstreams(cfg.Routes)
	checker := health.NewChecker(
		upstreams,
		time.Duration(cfg.Server.HealthCheck.IntervalMs)*time.Millisecond,
		logger,
	)

	forwarders := proxy.BuildForwarders(cfg.Routes, cfg.Server.Retry, cfg.Server.CircuitBreaker, logger)
	router := routing.New(cfg.Routes)
	p := proxy.New(router, logger, checker, forwarders)
	srv := server.New(cfg.Server.Port, p, logger, metrics, cfg.Server)

	go func() { _ = srv.Start() }()

	base := fmt.Sprintf("http://localhost:%d", cfg.Server.Port)
	waitReady(t, base)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})

	return &testGateway{BaseURL: base, srv: srv, checker: checker}
}

// waitReady polls /healthz until the server responds or 5 seconds elapse.
func waitReady(t *testing.T, base string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/healthz")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("gateway at %s not ready after 5s", base)
}

func uniqueUpstreams(routes []config.Route) []string {
	seen := make(map[string]bool)
	var out []string
	for _, r := range routes {
		if !seen[r.Upstream] {
			seen[r.Upstream] = true
			out = append(out, r.Upstream)
		}
	}
	return out
}

// --- Tests ---

func TestProxy_HappyPath(t *testing.T) {
	backend := mock.New(mock.HandlerConfig{StatusCode: 200, Body: "hello from upstream"})
	defer backend.Close()

	cfg := defaultCfg(freePort(t), backend.URL())
	gw := newTestGateway(t, cfg)

	resp, err := http.Get(gw.BaseURL + "/test/resource")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Request-ID") == "" {
		t.Error("expected X-Request-ID header in response")
	}
}

func TestProxy_UnknownRoute(t *testing.T) {
	backend := mock.New(mock.HandlerConfig{StatusCode: 200})
	defer backend.Close()

	cfg := defaultCfg(freePort(t), backend.URL())
	gw := newTestGateway(t, cfg)

	resp, err := http.Get(gw.BaseURL + "/no-such-route")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// TestProxy_UpstreamError verifies that when the upstream is unreachable and
// retries are exhausted, the gateway returns 502 Bad Gateway.
func TestProxy_UpstreamError(t *testing.T) {
	// Point to a port nothing is listening on. Port 1 is reserved and always refused.
	deadURL := "http://127.0.0.1:1"

	three := 3
	cfg := defaultCfg(freePort(t), deadURL)
	cfg.Server.Retry = config.RetryConfig{MaxAttempts: &three, BaseDelayMs: 0}

	gw := newTestGateway(t, cfg)

	resp, err := http.Get(gw.BaseURL + "/test")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", resp.StatusCode)
	}
}

// TestRateLimit_Exceeded verifies that requests over the burst limit receive 429.
func TestRateLimit_Exceeded(t *testing.T) {
	backend := mock.New(mock.HandlerConfig{StatusCode: 200})
	defer backend.Close()

	cfg := defaultCfg(freePort(t), backend.URL())
	cfg.Server.RateLimit = config.RateLimitConfig{RequestsPerSecond: 1, Burst: 1}

	gw := newTestGateway(t, cfg)

	// First request consumes the burst token — should succeed.
	resp1, err := http.Get(gw.BaseURL + "/test")
	if err != nil {
		t.Fatalf("request 1: %v", err)
	}
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Errorf("request 1: expected 200, got %d", resp1.StatusCode)
	}

	// Immediate second request — no token available yet.
	resp2, err := http.Get(gw.BaseURL + "/test")
	if err != nil {
		t.Fatalf("request 2: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusTooManyRequests {
		t.Errorf("request 2: expected 429, got %d", resp2.StatusCode)
	}
}

// TestTimeout_SlowUpstream verifies that a backend slower than the gateway
// timeout results in a 504 Gateway Timeout.
func TestTimeout_SlowUpstream(t *testing.T) {
	backend := mock.New(mock.HandlerConfig{StatusCode: 200, Delay: 300 * time.Millisecond})
	defer backend.Close()

	cfg := defaultCfg(freePort(t), backend.URL())
	cfg.Server.TimeoutMs = 100 // 100ms timeout, backend takes 300ms

	gw := newTestGateway(t, cfg)

	resp, err := http.Get(gw.BaseURL + "/test")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusGatewayTimeout {
		t.Errorf("expected 504, got %d", resp.StatusCode)
	}
}

// TestHealthCheck_MarksUnhealthy verifies that after the health checker polls
// a stopped upstream, the gateway rejects requests to that upstream with 503.
func TestHealthCheck_MarksUnhealthy(t *testing.T) {
	backend := mock.New(mock.HandlerConfig{StatusCode: 200})
	// Do not defer backend.Close() — we close it explicitly mid-test.

	cfg := defaultCfg(freePort(t), backend.URL())
	cfg.Server.HealthCheck = config.HealthCheckConfig{IntervalMs: 60000} // use manual polling only

	gw := newTestGateway(t, cfg)

	// Start the checker so it does the initial poll (backend is up → healthy).
	appCtx, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()
	gw.checker.Start(appCtx)
	time.Sleep(20 * time.Millisecond) // let initial poll complete

	// Confirm upstream is reachable.
	resp, err := http.Get(gw.BaseURL + "/test")
	if err != nil {
		t.Fatalf("initial request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("initial request: expected 200, got %d", resp.StatusCode)
	}

	// Stop the backend and force a health poll.
	backend.Close()
	gw.checker.PollAll()

	// Now the upstream is marked unhealthy; proxy should return 503.
	resp2, err := http.Get(gw.BaseURL + "/test")
	if err != nil {
		t.Fatalf("post-failure request: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("post-failure request: expected 503, got %d", resp2.StatusCode)
	}
}

// TestGracefulShutdown verifies that an in-flight request completes before
// the server exits, and that new requests are rejected after shutdown.
func TestGracefulShutdown(t *testing.T) {
	// Backend takes 300ms to respond.
	backend := mock.New(mock.HandlerConfig{StatusCode: 200, Body: "done", Delay: 300 * time.Millisecond})
	defer backend.Close()

	cfg := defaultCfg(freePort(t), backend.URL())
	cfg.Server.TimeoutMs = 10000 // long enough that timeout doesn't interfere

	gw := newTestGateway(t, cfg)

	// Fire a slow request in a goroutine.
	type result struct {
		code int
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		resp, err := http.Get(gw.BaseURL + "/test")
		if err != nil {
			ch <- result{err: err}
			return
		}
		resp.Body.Close()
		ch <- result{code: resp.StatusCode}
	}()

	// Give the request time to reach the backend before triggering shutdown.
	time.Sleep(50 * time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := gw.srv.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	// In-flight request must have completed before Shutdown returned.
	res := <-ch
	if res.err != nil {
		t.Fatalf("in-flight request failed: %v", res.err)
	}
	if res.code != http.StatusOK {
		t.Errorf("in-flight request: expected 200, got %d", res.code)
	}

	// New requests must fail (connection refused).
	_, err := http.Get(gw.BaseURL + "/test")
	if err == nil {
		t.Error("expected connection refused after shutdown, got nil error")
	}
}

// findModuleRoot walks up from the current working directory to find the Go
// module root (the directory containing go.mod).
func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found walking up from %s", dir)
		}
		dir = parent
	}
}

// TestRun_GracefulShutdownOnSIGTERM builds the real gateway binary, starts it
// as a subprocess, sends SIGTERM while a request is in-flight, and asserts that
// the in-flight request completes with 200 and the process exits with code 0.
// This exercises the signal.NotifyContext path in cmd/gateway/main.go that
// TestGracefulShutdown (which calls srv.Shutdown directly) does not cover.
func TestRun_GracefulShutdownOnSIGTERM(t *testing.T) {
	backend := mock.New(mock.HandlerConfig{StatusCode: 200, Body: "done", Delay: 300 * time.Millisecond})
	defer backend.Close()

	port := freePort(t)

	cfgContent := fmt.Sprintf(`
server:
  port: %d
  timeout_ms: 10000
  shutdown_timeout_ms: 5000
  rate_limit:
    requests_per_second: 100
    burst: 20
  health_check:
    interval_ms: 60000
  circuit_breaker:
    failure_threshold: 5
    recovery_timeout_ms: 30000
  retry:
    max_attempts: 1
    base_delay_ms: 0
routes:
  - path: /test
    upstream: %s
`, port, backend.URL())

	workDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workDir, "configs"), 0755); err != nil {
		t.Fatalf("mkdir configs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "configs", "gateway.yaml"), []byte(cfgContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	binPath := filepath.Join(t.TempDir(), "gateway")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/gateway")
	buildCmd.Dir = findModuleRoot(t)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	cmd := exec.Command(binPath)
	cmd.Dir = workDir
	if err := cmd.Start(); err != nil {
		t.Fatalf("start gateway: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	base := fmt.Sprintf("http://localhost:%d", port)
	waitReady(t, base)

	type result struct {
		code int
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		resp, err := http.Get(base + "/test")
		if err != nil {
			ch <- result{err: err}
			return
		}
		resp.Body.Close()
		ch <- result{code: resp.StatusCode}
	}()

	time.Sleep(50 * time.Millisecond)

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}

	exitCh := make(chan error, 1)
	go func() { exitCh <- cmd.Wait() }()

	select {
	case res := <-ch:
		if res.err != nil {
			t.Errorf("in-flight request: %v", res.err)
		} else if res.code != http.StatusOK {
			t.Errorf("in-flight request: expected 200, got %d", res.code)
		}
	case <-time.After(5 * time.Second):
		t.Error("in-flight request did not complete within 5s")
	}

	select {
	case err := <-exitCh:
		if err != nil {
			t.Errorf("expected clean exit after SIGTERM, got: %v", err)
		}
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Error("gateway did not exit within 10s after SIGTERM")
	}
}

// TestCircuitBreaker_Opens verifies that after the failure threshold is reached,
// the circuit breaker rejects subsequent requests with 503 Service Unavailable.
func TestCircuitBreaker_Opens(t *testing.T) {
	// FailFirst: 2 makes the backend return 500 for the first 2 requests.
	backend := mock.New(mock.HandlerConfig{StatusCode: 200, FailFirst: 2})
	defer backend.Close()

	two := 2
	one := 1
	cfg := defaultCfg(freePort(t), backend.URL())
	cfg.Server.CircuitBreaker = config.CircuitBreakerConfig{
		FailureThreshold:  &two,
		RecoveryTimeoutMs: 60000, // long recovery so circuit stays open during the test
	}
	cfg.Server.Retry = config.RetryConfig{MaxAttempts: &one, BaseDelayMs: 0}
	// Health checker is NOT started — upstreams are always considered healthy
	// so the circuit breaker (not the health checker) controls rejection.

	gw := newTestGateway(t, cfg)

	// Requests 1 and 2: backend returns 500 → forwarder records failure.
	for i := 1; i <= 2; i++ {
		resp, err := http.Get(gw.BaseURL + "/test")
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		resp.Body.Close()
		// Backend 500 is proxied as-is on the last (and only) attempt.
		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("request %d: expected 500 from backend, got %d", i, resp.StatusCode)
		}
	}

	// Request 3: circuit is open → 503 Service Unavailable (no upstream contact).
	resp, err := http.Get(gw.BaseURL + "/test")
	if err != nil {
		t.Fatalf("request 3: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("request 3: expected 503 (circuit open), got %d", resp.StatusCode)
	}
}
