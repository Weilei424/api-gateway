package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"gateway/internal/config"
	"gateway/internal/health"
	"gateway/internal/middleware"
	"gateway/internal/observability"
	"gateway/internal/proxy"

	"go.uber.org/zap"
)

type Server struct {
	httpServer *http.Server
}

func New(port int, p *proxy.Proxy, logger *zap.Logger, metrics *observability.Metrics, cfg config.ServerConfig) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", health.Handler())
	mux.Handle("/metrics", metrics.Handler())

	timeoutDuration := time.Duration(cfg.TimeoutMs) * time.Millisecond

	mux.Handle("/", middleware.Chain(
		p,
		middleware.RequestID(),
		middleware.Logging(logger),
		metrics.Middleware(),
		middleware.RateLimit(cfg.RateLimit.RequestsPerSecond, cfg.RateLimit.Burst),
		middleware.Timeout(timeoutDuration),
	))

	return &Server{
		httpServer: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: mux,
		},
	}
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
