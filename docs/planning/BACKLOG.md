# Backlog

## Status Legend
- [ ] Not started
- [x] Complete
- [~] In progress

---

### Phase 1 — Foundation

- [x] Initialize Go module
- [x] Create folder structure
- [x] Implement `main.go`
- [x] Start HTTP server (`server.go`)
- [x] Implement `/healthz` endpoint
- [x] Implement config structs
- [x] Load YAML config file
- [x] Validate routes (path, upstream URL, duplicates)

---

### Phase 2 — Core Proxy

- [x] Write router unit tests (`internal/routing/router_test.go`)
- [x] Implement `routing.New` — sort routes longest-prefix-first (`internal/routing/router.go`)
- [x] Implement `routing.Router.Match` — linear scan, return first hit
- [x] Write proxy unit tests (`internal/proxy/proxy_test.go`)
- [x] Implement `proxy.New` and `proxy.Proxy.ServeHTTP` (`internal/proxy/proxy.go`)
- [x] Set custom `ErrorHandler` on `ReverseProxy` (502 on upstream error)
- [x] Update `server.New` to accept `*routing.Router`, register proxy as catch-all
- [x] Update `main.go` to build router from config and pass to server
- [x] Verify `go build ./...` and `go test ./...` pass

---

### Phase 3 — Middleware & Observability

- [x] Add zap and Prometheus module dependencies in `gateway/go.mod`
- [x] Implement `middleware.Middleware` and `middleware.Chain` in `gateway/internal/middleware/chain.go`
- [x] Write middleware chain order tests in `gateway/internal/middleware/chain_test.go`
- [x] Implement request ID middleware and context accessor in `gateway/internal/middleware/requestid.go`
- [x] Write request ID middleware tests in `gateway/internal/middleware/requestid_test.go`
- [x] Implement zap logger constructor in `gateway/internal/observability/logging.go`
- [x] Implement request logging middleware in `gateway/internal/middleware/logging.go`
- [x] Write logging middleware tests using `zaptest/observer`
- [x] Implement Prometheus collectors and `/metrics` handler in `gateway/internal/observability/metrics.go`
- [x] Write metrics tests covering `gateway_requests_total` and `gateway_request_duration_seconds`
- [x] Update `gateway/internal/proxy/proxy.go` to expose matched upstream via request context and add error logging
- [x] Extend proxy tests to assert original path and query string are preserved
- [x] Update `gateway/internal/server/server.go` to accept logger and metrics dependencies and wrap proxy traffic with middleware
- [x] Add server wiring tests for `/healthz`, `/metrics`, proxied request ID propagation, and upstream logging
- [x] Update `gateway/cmd/gateway/main.go` to construct logger and metrics and pass them to `server.New`
- [x] Verify `go test ./...` and `go build ./...` pass

---

### Phase 4 — Reliability

- [ ] Implement request timeout middleware (`internal/middleware/timeout.go`)
- [ ] Configure HTTP transport timeout
- [ ] Implement periodic upstream health checks (`internal/health/checker.go`)
- [ ] Mark upstreams healthy/unhealthy
- [ ] Implement token bucket rate limiter (`internal/middleware/ratelimit.go`)
- [ ] Apply per-client-IP rate limit
- [ ] Implement retry logic with exponential backoff (`internal/proxy/retry.go`)
- [ ] Implement circuit breaker (`internal/proxy/circuitbreaker.go`)
- [ ] Open circuit on failure threshold, allow recovery

---

### Phase 5 — Polish & Demo

- [ ] Implement graceful shutdown (OS signal handling)
- [ ] Allow in-flight requests to finish before exit
- [ ] Write integration tests (`test/integration/`)
- [ ] Add mock backend services for local demo
- [ ] Add example configs
- [ ] Write README
