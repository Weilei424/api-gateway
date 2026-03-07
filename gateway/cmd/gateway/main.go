package main

import (
	"log"

	"gateway/internal/server"
)

func main() {
	srv := server.New(8080)
	if err := srv.Start(); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
