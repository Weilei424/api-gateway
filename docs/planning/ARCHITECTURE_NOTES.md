# Architecture Notes

## Stack

- **Language:** Go (standard library preferred)
- **Config:** YAML via `gopkg.in/yaml.v3`
- **Logging:** `go.uber.org/zap` (structured)
- **Metrics:** Prometheus (`github.com/prometheus/client_golang`)
- **Proxy:** `net/http/httputil.ReverseProxy`
- **No frameworks** — pure `net/http`

---

## Key Decisions

| Decision | Rationale |
|----------|-----------|
| Prefix-based routing only | Keeps router simple and interview-explainable; no regex overhead |
| `httputil.ReverseProxy` | Standard library; avoids raw TCP proxying; customizable via `Transport` and `ErrorHandler` |
| Middleware as `func(next http.Handler) http.Handler` | Idiomatic Go; composes cleanly; matches `http.Handler` contract |
| zap for logging | Structured, fast; required by CLAUDE.md |
| No DI framework | Explicit wiring in `main.go`; easy to trace and explain |
| Config validated at startup | Fail fast — bad config is caught before the server binds a port |

---

## Component Responsibilities

| Component | Owns |
|-----------|------|
| `config` | Loading, parsing, and validating gateway.yaml |
| `routing` | Matching request path → upstream URL (prefix match) |
| `proxy` | Forwarding request to upstream, error handling, retry, circuit breaker |
| `middleware` | Request ID, logging, rate limiting, timeout — composable chain |
| `health` | `/healthz` handler + background upstream health checker |
| `observability` | zap logger init, Prometheus registry, `/metrics` handler |
| `server` | Wiring mux, middleware chain, graceful shutdown |

---

## Design Constraints

- All middleware must follow `func(next http.Handler) http.Handler`
- Never use `fmt.Println` or `log.Println` in production paths — always zap
- Always propagate `context.Context` through requests
- Do not introduce interfaces unless there are multiple implementations or testability requires it
- Code must be explainable per module in 2–5 minutes (interview standard)

---

## Security Constraints

- No open proxy vulnerabilities — only forward to explicitly configured upstreams
- No unsafe header forwarding — strip or sanitize hop-by-hop headers
- No silent failures — errors must be logged and returned as appropriate HTTP responses
- No header injection from upstream responses into gateway-controlled headers
