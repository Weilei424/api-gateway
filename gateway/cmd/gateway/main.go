package main

import (
	"fmt"
	"os"

	"gateway/internal/config"
	"gateway/internal/observability"
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

	router := routing.New(cfg.Routes)
	metrics, err := observability.NewMetrics()
	if err != nil {
		logger.Error("failed to initialize metrics", zap.Error(err))
		return 1
	}

	srv := server.New(cfg.Server.Port, router, logger, metrics)
	logger.Info("starting gateway server", zap.Int("port", cfg.Server.Port))
	if err := srv.Start(); err != nil {
		logger.Error("server stopped", zap.Error(err))
		return 1
	}

	return 0
}

func main() {
	os.Exit(run())
}
