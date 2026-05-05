package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"gateway/internal/config"
	"gateway/internal/health"
	"gateway/internal/observability"
	"gateway/internal/proxy"
	"gateway/internal/routing"
	"gateway/internal/server"

	"go.uber.org/zap"
)

var newLogger = observability.NewLogger

func run() int {
	logger, err := newLogger()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		return 1
	}
	defer func() {
		_ = logger.Sync()
	}()

	cfg, err := config.Load("configs/gateway.yaml")
	if err != nil {
		logger.Error("failed to load config", zap.Error(err))
		return 1
	}

	logger.Info("loaded routes", zap.Int("count", len(cfg.Routes)))
	for _, r := range cfg.Routes {
		logger.Info("configured route",
			zap.String("path", r.Path),
			zap.String("upstream", r.Upstream),
		)
	}

	metrics, err := observability.NewMetrics()
	if err != nil {
		logger.Error("failed to initialize metrics", zap.Error(err))
		return 1
	}

	// Collect unique upstream URLs for health checking.
	seen := make(map[string]bool)
	var upstreams []string
	for _, r := range cfg.Routes {
		if !seen[r.Upstream] {
			seen[r.Upstream] = true
			upstreams = append(upstreams, r.Upstream)
		}
	}

	interval := time.Duration(cfg.Server.HealthCheck.IntervalMs) * time.Millisecond
	checker := health.NewChecker(upstreams, interval, logger)
	checker.Start(context.Background())

	forwarders := proxy.BuildForwarders(cfg.Routes, cfg.Server.Retry, cfg.Server.CircuitBreaker, logger)
	router := routing.New(cfg.Routes)
	p := proxy.New(router, logger, checker, forwarders)

	srv := server.New(cfg.Server.Port, p, logger, metrics, cfg.Server)
	logger.Info("starting gateway server", zap.Int("port", cfg.Server.Port))

	// Validate that all upstream URLs parsed correctly (already done in config,
	// but log any that couldn't be resolved to a forwarder for observability).
	for _, upstream := range upstreams {
		if _, err := url.Parse(upstream); err != nil {
			logger.Warn("unparseable upstream URL", zap.String("upstream", upstream))
		}
	}

	if err := srv.Start(); err != nil {
		logger.Error("server stopped", zap.Error(err))
		return 1
	}

	return 0
}

func main() {
	os.Exit(run())
}
