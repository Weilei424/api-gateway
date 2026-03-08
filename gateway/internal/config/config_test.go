package config

import (
	"os"
	"path/filepath"
	"testing"
)

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
			if !containsString(err.Error(), tt.wantErr) {
				t.Errorf("expected error to contain %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
