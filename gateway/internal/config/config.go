package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the top-level gateway configuration.
type Config struct {
	Server ServerConfig `yaml:"server"`
	Routes []Route      `yaml:"routes"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port int `yaml:"port"`
}

// Route maps an incoming path prefix to an upstream service URL.
type Route struct {
	Path     string `yaml:"path"`
	Upstream string `yaml:"upstream"`
}

// Load reads and validates the YAML config at the given file path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func validate(cfg *Config) error {
	if cfg.Server.Port <= 0 {
		return fmt.Errorf("server.port must be a positive integer")
	}
	if len(cfg.Routes) == 0 {
		return fmt.Errorf("at least one route must be defined")
	}

	seen := make(map[string]bool)
	for i, r := range cfg.Routes {
		path := strings.TrimSpace(r.Path)
		if path == "" {
			return fmt.Errorf("route[%d]: path is required", i)
		}
		if !strings.HasPrefix(path, "/") {
			return fmt.Errorf("route[%d]: path must start with /", i)
		}
		if seen[path] {
			return fmt.Errorf("route[%d]: duplicate path %q", i, path)
		}
		seen[path] = true

		if err := validateUpstream(r.Upstream, i); err != nil {
			return err
		}
	}
	return nil
}

func validateUpstream(upstream string, i int) error {
	if strings.TrimSpace(upstream) == "" {
		return fmt.Errorf("route[%d]: upstream is required", i)
	}
	u, err := url.Parse(upstream)
	if err != nil {
		return fmt.Errorf("route[%d]: upstream is not a valid URL: %w", i, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("route[%d]: upstream scheme must be http or https, got %q", i, u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("route[%d]: upstream must include a host", i)
	}
	return nil
}
