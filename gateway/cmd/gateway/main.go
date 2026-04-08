package main

import (
	"log"

	"gateway/internal/config"
	"gateway/internal/routing"
	"gateway/internal/server"
)

func main() {
	cfg, err := config.Load("configs/gateway.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	log.Printf("loaded %d route(s):", len(cfg.Routes))
	for _, r := range cfg.Routes {
		log.Printf("  %s -> %s", r.Path, r.Upstream)
	}

	router := routing.New(cfg.Routes)
	srv := server.New(cfg.Server.Port, router)
	if err := srv.Start(); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
