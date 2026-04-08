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

- [ ] Implement middleware chain (`internal/middleware/chain.go`)
- [ ] Integrate middleware chain with router/server
- [ ] Integrate zap structured logger
- [ ] Log request metadata (method, path, status, latency, upstream)
- [ ] Implement request ID middleware (`internal/middleware/requestid.go`)
- [ ] Implement Prometheus request counter (`internal/observability/metrics.go`)
- [ ] Implement Prometheus latency histogram
- [ ] Expose `/metrics` endpoint

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
