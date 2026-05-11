package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
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

	appCtx, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()

	interval := time.Duration(cfg.Server.HealthCheck.IntervalMs) * time.Millisecond
	checker := health.NewChecker(upstreams, interval, logger)
	checker.Start(appCtx)

	forwarders := proxy.BuildForwarders(cfg.Routes, cfg.Server.Retry, cfg.Server.CircuitBreaker, logger)
	router := routing.New(cfg.Routes)
	p := proxy.New(router, logger, checker, forwarders)

	srv := server.New(cfg.Server.Port, p, logger, metrics, cfg.Server)

	// Validate upstream URL parseability (observability only — already validated by config).
	for _, upstream := range upstreams {
		if _, err := url.Parse(upstream); err != nil {
			logger.Warn("unparseable upstream URL", zap.String("upstream", upstream))
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", zap.Error(err))
		}
	}()

	logger.Info("starting gateway server", zap.Int("port", cfg.Server.Port))
	<-ctx.Done()
	stop()
	cancelApp() // stop health checker before draining

	timeout := time.Duration(cfg.Server.ShutdownTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
		return 1
	}
	logger.Info("gateway stopped cleanly")
	return 0
}

func main() {
	os.Exit(run())
}
