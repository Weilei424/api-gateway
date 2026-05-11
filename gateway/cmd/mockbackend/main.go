package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	port := flag.Int("port", 9001, "port to listen on")
	status := flag.Int("status", 200, "HTTP status code to return")
	delayMs := flag.Int("delay", 0, "response delay in milliseconds")
	body := flag.String("body", "ok", "response body")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(os.Stdout, "%s %s\n", r.Method, r.URL.Path)
		if *delayMs > 0 {
			time.Sleep(time.Duration(*delayMs) * time.Millisecond)
		}
		w.WriteHeader(*status)
		_, _ = w.Write([]byte(*body))
	})

	addr := fmt.Sprintf(":%d", *port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen %s: %v\n", addr, err)
		os.Exit(1)
	}

	srv := &http.Server{Handler: mux}
	go func() {
		fmt.Fprintf(os.Stdout, "mock backend listening on %s (status=%d delay=%dms)\n", addr, *status, *delayMs)
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Fprintln(os.Stdout, "shutting down")
}
