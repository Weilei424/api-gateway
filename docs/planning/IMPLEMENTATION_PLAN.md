# Implementation Plan

## Overview

Mini API Gateway implemented in Go — inspired by Kong, Envoy, and NGINX Gateway. Designed to be production-inspired, interview-explainable, and incrementally buildable. Features are delivered in five phases. Target completion: 4–8 weeks.

---

## Phases

### Phase 1 — Foundation (Steps 1–2) ✅ COMPLETE

**Goal:** Bootable server with config loading.

**Deliverables:**
- `cmd/gateway/main.go` — entry point
- `internal/server/server.go` — HTTP server, `/healthz` registration
- `internal/health/health.go` — health handler returning `{"status":"ok"}`
- `internal/config/config.go` — YAML config loader with route validation
- `configs/gateway.yaml` — example config

**Success criteria:**
- `go run ./cmd/gateway` starts without error
- `curl localhost:8080/healthz` returns `{"status":"ok"}`
- Invalid config (bad port, missing upstream, duplicate path) is rejected at startup

---

### Phase 2 — Core Proxy (Steps 3–4) ✅ COMPLETE

**Goal:** Route incoming requests to upstream services via reverse proxy.

**Deliverables:**
- `internal/routing/router.go` — prefix-based router mapping path → upstream URL
- `internal/proxy/proxy.go` — `net/http/httputil.ReverseProxy` wrapper with custom transport and error handler

**Success criteria:**
- Requests to `/users/...` are forwarded to the configured upstream
- Unknown paths return `404`
- Upstream errors return `502`

---

### Phase 3 — Middleware & Observability (Steps 5–7) ✅ COMPLETE

**Goal:** Per-request middleware pipeline, structured logging, Prometheus metrics.

**Deliverables:**
- `internal/middleware/chain.go` — `func(next http.Handler) http.Handler` chaining
- `internal/middleware/requestid.go` — inject `X-Request-ID` header
- `internal/observability/logging.go` — zap logger setup
- `internal/middleware/logging.go` — request logging middleware (method, path, status, latency, upstream, request_id)
- `internal/observability/metrics.go` — Prometheus counter + histogram, `/metrics` endpoint

**Success criteria:**
- Every request produces a structured zap log line
- `/metrics` exposes `gateway_requests_total` and `gateway_request_duration_seconds`
- Matched upstream is available in request completion logs for proxied traffic

---

### Phase 4 — Reliability (Steps 8–12)

**Goal:** Protect gateway and upstreams from overload and transient failures.

**Deliverables:**
- `internal/middleware/timeout.go` — per-request timeout via `context.WithTimeout`
- `internal/health/checker.go` — background goroutine polling upstreams
- `internal/middleware/ratelimit.go` — token bucket limiter keyed by client IP
- `internal/proxy/retry.go` — retry on 5xx/network error with exponential backoff
- `internal/proxy/circuitbreaker.go` — failure counter, open/half-open/closed states

**Success criteria:**
- Requests exceeding timeout return `504`
- Requests over rate limit return `429`
- Unhealthy upstreams are skipped
- Transient failures are retried (up to configured max)
- Circuit opens after threshold failures, recovers after cooldown

---

### Phase 5 — Polish & Demo (Steps 13–15)

**Goal:** Production-grade shutdown, integration tests, demo setup.

**Deliverables:**
- Graceful shutdown in `main.go` (SIGINT/SIGTERM → `server.Shutdown`)
- `test/integration/` — end-to-end tests with mock backends
- Mock backend services for local demo
- `README.md`

**Success criteria:**
- In-flight requests complete before server exits
- Integration tests pass with `go test ./test/integration/...`
- `README.md` explains how to run and demo the gateway
