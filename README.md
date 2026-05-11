# API Gateway

A mini production-inspired API gateway written in Go. Demonstrates networking fundamentals, reliability patterns, structured observability, and clean architecture — modelled after Kong, Envoy, and NGINX Gateway but simplified for learning and interviews.

---

## Quick Start

```bash
cd gateway
go run ./cmd/gateway
```

The gateway reads `configs/gateway.yaml` on startup. Once running:

```bash
curl http://localhost:8080/healthz      # → {"status":"ok"}
curl http://localhost:8080/metrics      # → Prometheus metrics
curl http://localhost:8080/users/123    # → proxied to configured upstream
```

---

## Running the Demo

Start one or more mock backends, then start the gateway.

**Terminal 1 — mock backend for /users:**
```bash
cd gateway
go run ./cmd/mockbackend --port=9001 --status=200 --body='{"id":1,"name":"Alice"}'
```

**Terminal 2 — mock backend for /orders:**
```bash
cd gateway
go run ./cmd/mockbackend --port=9002 --status=200 --body='{"orderId":42}'
```

**Terminal 3 — gateway:**
```bash
cd gateway
go run ./cmd/gateway
```

Send traffic and observe structured JSON logs and Prometheus metrics:
```bash
curl http://localhost:8080/users/1
curl http://localhost:8080/orders/42
curl http://localhost:8080/metrics
```

Stop the gateway with `Ctrl-C` — in-flight requests finish before the process exits.

---

## Configuration Reference

All configuration lives in `gateway/configs/gateway.yaml`. See `gateway/configs/gateway.example.yaml` for an annotated three-route example.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `server.port` | int | — | Port the gateway listens on |
| `server.timeout_ms` | int | 0 | Per-request timeout in ms (0 = disabled) |
| `server.shutdown_timeout_ms` | int | 0 | Graceful shutdown drain window in ms (0 = 15s default) |
| `server.rate_limit.requests_per_second` | float | 0 | Token bucket refill rate per client IP (0 = disabled) |
| `server.rate_limit.burst` | int | 0 | Maximum burst size per client IP |
| `server.health_check.interval_ms` | int | — | Upstream health poll interval in ms |
| `server.circuit_breaker.failure_threshold` | int | 5 | Failures before circuit opens |
| `server.circuit_breaker.recovery_timeout_ms` | int | 30000 | Recovery probe delay in ms |
| `server.retry.max_attempts` | int | 1 | Total attempts per request (1 = no retry) |
| `server.retry.base_delay_ms` | int | 100 | Base exponential backoff delay in ms |
| `routes[].path` | string | — | URL prefix to match (must start with `/`) |
| `routes[].upstream` | string | — | Upstream base URL (`http://host:port`) |

---

## Architecture Overview

```
                        ┌─────────────────────────────────────────┐
                        │               Gateway Process            │
                        │                                          │
Client ─── HTTP ───────►│  Timeout → RateLimit → Log → Proxy      │
                        │               │                          │
                        │        ┌──────▼──────┐                  │
                        │        │   Router    │  prefix match    │
                        │        └──────┬──────┘                  │
                        │               │upstream URL             │
                        │        ┌──────▼──────┐                  │
                        │        │  Forwarder  │  retry + CB      │
                        │        └──────┬──────┘                  │
                        └──────────────┼──────────────────────────┘
                                       │ HTTP
                                  ┌────▼─────┐
                                  │ Upstream │
                                  └──────────┘
```

| Package | Responsibility |
|---------|----------------|
| `config` | Load, parse, and validate `gateway.yaml` |
| `routing` | Match request path → upstream URL (longest-prefix-first) |
| `proxy` | Forward to upstream via `httputil.ReverseProxy`; owns retry and circuit breaker |
| `middleware` | Composable `func(next http.Handler) http.Handler` pipeline: request ID, logging, metrics, rate limit, timeout |
| `health` | `/healthz` handler + background goroutine polling upstream health |
| `observability` | zap logger setup; Prometheus registry and `/metrics` handler |
| `server` | Wires mux, middleware chain, and HTTP server |

---

## Reliability Features

**Request Timeout** — Each request runs with a `context.WithTimeout`. Requests that exceed the configured `timeout_ms` receive `504 Gateway Timeout` before the response headers are written. Configured upstream timeouts are respected by the HTTP transport.

**Rate Limiting** — A token bucket limiter (one per client IP, keyed by `RemoteAddr`) enforces `requests_per_second` with a configurable `burst`. Requests over the limit receive `429 Too Many Requests` immediately, without reaching the upstream.

**Retry with Exponential Backoff** — GET and HEAD requests that fail with a network error are retried up to `max_attempts` times with jitter. Deadline-exceeded errors are not retried. Backoff doubles each attempt starting from `base_delay_ms`.

**Circuit Breaker** — Each upstream has a circuit breaker tracking consecutive failures. After `failure_threshold` failures the circuit opens. Requests to an open circuit receive `503 Service Unavailable` without contacting the upstream. After `recovery_timeout_ms`, one probe is allowed through (half-open state); a success closes the circuit.

**Upstream Health Checking** — A background goroutine polls each upstream's root path (`GET /`) every `interval_ms`. Upstreams that respond with `5xx` or fail to connect are marked unhealthy. Requests to an unhealthy upstream return `503` immediately, bypassing the forwarder.

**Graceful Shutdown** — On `SIGINT` or `SIGTERM`, the gateway stops accepting new connections and waits up to `shutdown_timeout_ms` for in-flight requests to complete before exiting. The health checker stops as soon as the signal is received.

---

## Running Tests

**Unit tests (fast, no external dependencies):**
```bash
cd gateway
go test ./...
```

**Integration tests (start real gateway + mock backends in-process):**
```bash
cd gateway
go test -tags=integration ./test/integration/... -v
```

Integration tests exercise the full reliability contract: happy-path routing, unknown routes, upstream errors, rate limiting, timeouts, circuit breaker tripping, health check marking, and graceful shutdown.
