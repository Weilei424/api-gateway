package server

import (
	"fmt"
	"log"
	"net/http"

	"gateway/internal/health"
	"gateway/internal/proxy"
	"gateway/internal/routing"
)

type Server struct {
	httpServer *http.Server
}

func New(port int, router *routing.Router) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", health.Handler())
	mux.Handle("/", proxy.New(router))

	return &Server{
		httpServer: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: mux,
		},
	}
}

func (s *Server) Start() error {
	log.Printf("gateway listening on %s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}
