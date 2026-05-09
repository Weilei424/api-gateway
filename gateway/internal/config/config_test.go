package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func intPtr(v int) *int { return &v }

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "gateway-*.yaml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

func TestLoad_Valid(t *testing.T) {
	yaml := `
server:
  port: 8080
routes:
  - path: /users
    upstream: http://localhost:9001
  - path: /orders
    upstream: http://localhost:9002
`
	cfg, err := Load(writeTemp(t, yaml))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if len(cfg.Routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(cfg.Routes))
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "valid",
			cfg: Config{
				Server: ServerConfig{Port: 8080},
				Routes: []Route{{Path: "/a", Upstream: "http://localhost:9001"}},
			},
		},
		{
			name:    "port zero",
			cfg:     Config{Server: ServerConfig{Port: 0}, Routes: []Route{{Path: "/a", Upstream: "http://localhost:9001"}}},
			wantErr: "server.port",
		},
		{
			name:    "no routes",
			cfg:     Config{Server: ServerConfig{Port: 8080}},
			wantErr: "at least one route",
		},
		{
			name:    "empty path",
			cfg:     Config{Server: ServerConfig{Port: 8080}, Routes: []Route{{Path: "", Upstream: "http://localhost:9001"}}},
			wantErr: "path is required",
		},
		{
			name:    "path missing leading slash",
			cfg:     Config{Server: ServerConfig{Port: 8080}, Routes: []Route{{Path: "users", Upstream: "http://localhost:9001"}}},
			wantErr: "must start with /",
		},
		{
			name: "duplicate paths",
			cfg: Config{
				Server: ServerConfig{Port: 8080},
				Routes: []Route{
					{Path: "/a", Upstream: "http://localhost:9001"},
					{Path: "/a", Upstream: "http://localhost:9002"},
				},
			},
			wantErr: "duplicate path",
		},
		{
			name:    "empty upstream",
			cfg:     Config{Server: ServerConfig{Port: 8080}, Routes: []Route{{Path: "/a", Upstream: ""}}},
			wantErr: "upstream is required",
		},
		{
			name:    "upstream bad scheme",
			cfg:     Config{Server: ServerConfig{Port: 8080}, Routes: []Route{{Path: "/a", Upstream: "ftp://localhost:9001"}}},
			wantErr: "scheme must be http or https",
		},
		{
			name:    "upstream no host",
			cfg:     Config{Server: ServerConfig{Port: 8080}, Routes: []Route{{Path: "/a", Upstream: "http://"}}},
			wantErr: "must include a host",
		},
		{
			name: "negative timeout_ms",
			cfg: Config{
				Server: ServerConfig{Port: 8080, TimeoutMs: -1},
				Routes: []Route{{Path: "/a", Upstream: "http://localhost:9001"}},
			},
			wantErr: "timeout_ms",
		},
		{
			name: "negative retry base_delay_ms",
			cfg: Config{
				Server: ServerConfig{
					Port:  8080,
					Retry: RetryConfig{MaxAttempts: intPtr(3), BaseDelayMs: -1},
				},
				Routes: []Route{{Path: "/a", Upstream: "http://localhost:9001"}},
			},
			wantErr: "base_delay_ms",
		},
		{
			name: "rate limit enabled with zero burst",
			cfg: Config{
				Server: ServerConfig{
					Port:      8080,
					RateLimit: RateLimitConfig{RequestsPerSecond: 10, Burst: 0},
				},
				Routes: []Route{{Path: "/a", Upstream: "http://localhost:9001"}},
			},
			wantErr: "burst",
		},
		{
			name: "negative circuit_breaker failure_threshold",
			cfg: Config{
				Server: ServerConfig{
					Port:           8080,
					CircuitBreaker: CircuitBreakerConfig{FailureThreshold: intPtr(-1), RecoveryTimeoutMs: 5000},
				},
				Routes: []Route{{Path: "/a", Upstream: "http://localhost:9001"}},
			},
			wantErr: "failure_threshold",
		},
		{
			name: "zero circuit_breaker failure_threshold with recovery configured",
			cfg: Config{
				Server: ServerConfig{
					Port:           8080,
					CircuitBreaker: CircuitBreakerConfig{FailureThreshold: intPtr(0), RecoveryTimeoutMs: 5000},
				},
				Routes: []Route{{Path: "/a", Upstream: "http://localhost:9001"}},
			},
			wantErr: "failure_threshold",
		},
		{
			name: "zero circuit_breaker failure_threshold without sibling field",
			cfg: Config{
				Server: ServerConfig{
					Port:           8080,
					CircuitBreaker: CircuitBreakerConfig{FailureThreshold: intPtr(0)},
				},
				Routes: []Route{{Path: "/a", Upstream: "http://localhost:9001"}},
			},
			wantErr: "failure_threshold",
		},
		{
			name: "negative retry max_attempts",
			cfg: Config{
				Server: ServerConfig{
					Port:  8080,
					Retry: RetryConfig{MaxAttempts: intPtr(-1)},
				},
				Routes: []Route{{Path: "/a", Upstream: "http://localhost:9001"}},
			},
			wantErr: "max_attempts",
		},
		{
			name: "zero retry max_attempts with base_delay configured",
			cfg: Config{
				Server: ServerConfig{
					Port:  8080,
					Retry: RetryConfig{MaxAttempts: intPtr(0), BaseDelayMs: 100},
				},
				Routes: []Route{{Path: "/a", Upstream: "http://localhost:9001"}},
			},
			wantErr: "max_attempts",
		},
		{
			name: "zero retry max_attempts without sibling field",
			cfg: Config{
				Server: ServerConfig{
					Port:  8080,
					Retry: RetryConfig{MaxAttempts: intPtr(0)},
				},
				Routes: []Route{{Path: "/a", Upstream: "http://localhost:9001"}},
			},
			wantErr: "max_attempts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(&tt.cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error to contain %q, got: %v", tt.wantErr, err)
			}
		})
	}
}
