package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	Routes []Route      `yaml:"routes"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

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
	for i, r := range cfg.Routes {
		if r.Path == "" {
			return fmt.Errorf("route[%d]: path is required", i)
		}
		if r.Upstream == "" {
			return fmt.Errorf("route[%d]: upstream is required", i)
		}
	}
	return nil
}
