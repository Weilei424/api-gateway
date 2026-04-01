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

- [ ] Implement prefix-based router (`internal/routing/router.go`)
- [ ] Map route → upstream by path prefix
- [ ] Implement reverse proxy (`internal/proxy/proxy.go`)
- [ ] Configure HTTP transport
- [ ] Propagate headers through proxy

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
