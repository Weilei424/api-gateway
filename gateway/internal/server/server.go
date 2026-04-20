package server

import (
	"fmt"
	"net/http"

	"gateway/internal/health"
	"gateway/internal/middleware"
	"gateway/internal/observability"
	"gateway/internal/proxy"
	"gateway/internal/routing"

	"go.uber.org/zap"
)

type Server struct {
	httpServer *http.Server
}

func New(port int, router *routing.Router, logger *zap.Logger, metrics *observability.Metrics) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", health.Handler())
	mux.Handle("/metrics", metrics.Handler())
	mux.Handle("/", middleware.Chain(
		proxy.New(router, logger),
		middleware.RequestID(),
		middleware.Logging(logger),
		metrics.Middleware(),
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
