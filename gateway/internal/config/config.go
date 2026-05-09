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
	Port           int                  `yaml:"port"`
	TimeoutMs      int                  `yaml:"timeout_ms"`
	RateLimit      RateLimitConfig      `yaml:"rate_limit"`
	HealthCheck    HealthCheckConfig    `yaml:"health_check"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
	Retry          RetryConfig          `yaml:"retry"`
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	RequestsPerSecond float64 `yaml:"requests_per_second"`
	Burst             int     `yaml:"burst"`
}

// HealthCheckConfig holds health check settings.
type HealthCheckConfig struct {
	IntervalMs int `yaml:"interval_ms"`
}

// CircuitBreakerConfig holds circuit breaker settings.
type CircuitBreakerConfig struct {
	FailureThreshold  int `yaml:"failure_threshold"`
	RecoveryTimeoutMs int `yaml:"recovery_timeout_ms"`
}

// RetryConfig holds retry settings.
type RetryConfig struct {
	MaxAttempts int `yaml:"max_attempts"`
	BaseDelayMs int `yaml:"base_delay_ms"`
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
	if cfg.Server.TimeoutMs < 0 {
		return fmt.Errorf("server.timeout_ms must be >= 0")
	}
	if cfg.Server.RateLimit.RequestsPerSecond > 0 && cfg.Server.RateLimit.Burst < 1 {
		return fmt.Errorf("server.rate_limit.burst must be >= 1 when rate limiting is enabled")
	}
	cb := cfg.Server.CircuitBreaker
	if cb.FailureThreshold != 0 || cb.RecoveryTimeoutMs != 0 {
		if cb.FailureThreshold < 1 {
			return fmt.Errorf("server.circuit_breaker.failure_threshold must be >= 1")
		}
	}
	retry := cfg.Server.Retry
	if retry.MaxAttempts != 0 || retry.BaseDelayMs != 0 {
		if retry.MaxAttempts < 1 {
			return fmt.Errorf("server.retry.max_attempts must be >= 1")
		}
	}
	if cfg.Server.Retry.MaxAttempts > 1 && cfg.Server.Retry.BaseDelayMs < 0 {
		return fmt.Errorf("server.retry.base_delay_ms must be >= 0")
	}
	if len(cfg.Routes) == 0 {
		return fmt.Errorf("at least one route must be defined")
	}

	seen := make(map[string]bool)
	for i := range cfg.Routes {
		cfg.Routes[i].Path = strings.TrimSpace(cfg.Routes[i].Path)
		if cfg.Routes[i].Path == "" {
			return fmt.Errorf("route[%d]: path is required", i)
		}
		if !strings.HasPrefix(cfg.Routes[i].Path, "/") {
			return fmt.Errorf("route[%d]: path must start with /", i)
		}
		if seen[cfg.Routes[i].Path] {
			return fmt.Errorf("route[%d]: duplicate path %q", i, cfg.Routes[i].Path)
		}
		seen[cfg.Routes[i].Path] = true

		if err := validateUpstream(cfg.Routes[i].Upstream, i); err != nil {
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
