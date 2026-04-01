# CLAUDE.md
Implementation Instructions for Claude Code (Coding Agent)

## Role

You are the **primary implementation agent** for the API Gateway project.

Your responsibility is to:
- Implement production-quality Go code
- Follow the project architecture strictly
- Write clear, idiomatic, maintainable Go
- Implement features step-by-step according to `docs/planning/BACKLOG.md`
- Ensure code remains simple, understandable, and interview-friendly

You are NOT responsible for final approval of code.  
All code must pass review by the **CODEX reviewer agent**.

---

# Core Principles

1. **Prefer Go standard library**
2. **Avoid unnecessary abstraction**
3. **Keep code easy to explain in interviews**
4. **Keep modules small and focused**
5. **Avoid premature optimization**
6. **Write readable code over clever code**
7. **Use clear naming over short naming**

---

# Implementation Rules

## 1. Follow Architecture

All implementation must follow the structure defined in:

- `ARCHITECTURE.md`
- `docs/planning/ARCHITECTURE_NOTES.md`

You must not introduce new architecture layers unless absolutely necessary.

---

## 2. Folder Structure (Strict)
gateway/
cmd/gateway/
internal/
config/
routing/
proxy/
middleware/
observability/
health/
server/
configs/
test/

Do not create additional top-level directories.

---

## 3. Code Style

Use idiomatic Go.

Examples:

Good:
func NewRouter(routes []Route) *Router

Bad:
func BuildRouterManagerServiceObject(...)

---

## 4. Interfaces

Do NOT introduce interfaces unless:

- there are **multiple implementations**
- the interface **improves testability**
- the interface **reduces coupling**

Avoid Java-style abstractions.

---

## 5. Middleware Pattern

All middleware must follow:
func(next http.Handler) http.Handler

Example:
func LoggingMiddleware(next http.Handler) http.Handler

---

## 6. Error Handling

Always return meaningful errors.

Avoid silent failures.

Example:
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}

---

## 7. Logging

Use structured logging with **zap**.

Never use:
fmt.Println
log.Println


Always include fields such as:

- request_id
- method
- path
- status
- latency
- upstream

---

## 8. Context Usage

Always propagate context:
req = req.WithContext(ctx)


Timeouts and cancellations must respect context.

---

## 9. Reverse Proxy

Use:
net/http/httputil.ReverseProxy


Customize:

- Transport
- Error handler
- Timeout

Do NOT implement raw TCP proxying.

---

## 10. Keep Code Interview Friendly

Code should be explainable in interviews within:

- 2–5 minutes per module

Avoid complex generics or patterns that obscure logic.

---

# Testing Expectations

Claude should implement code that supports:

- Unit tests
- Integration tests
- Local multi-service demo

Tests should live in:
test/
or
*_test.go

---

# When Implementing Features

Always:

1. Implement **one feature at a time**
2. Ensure code compiles
3. Ensure tests pass
4. Ensure CODEX review passes

---

# Feature Development Order

Follow **`docs/planning/BACKLOG.md`** strictly.

Do NOT skip steps.

---

# Code Output Rules

When generating code:

- Provide **complete file content**
- Include package name
- Include imports
- Include comments only when useful

---

# Anti-Patterns to Avoid

Do NOT implement:

- dependency injection frameworks
- service containers
- reflection-heavy code
- generic frameworks
- overuse of interfaces

---

# Goal of the Project

This project is meant to resemble a **mini production API gateway** similar to:

- Kong
- Envoy
- NGINX gateway

But simplified for learning and interviews.

Focus on demonstrating:

- networking fundamentals
- reliability patterns
- observability
- clean Go architecture
