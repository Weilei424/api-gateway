package server

import (
	"fmt"
	"log"
	"net/http"

	"gateway/internal/health"
)

type Server struct {
	httpServer *http.Server
}

func New(port int) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", health.Handler())

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
