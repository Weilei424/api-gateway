package proxy

import (
	"context"
	"net/http"

	"gateway/internal/health"
	"gateway/internal/routing"

	"go.uber.org/zap"
)

type Proxy struct {
	router     *routing.Router
	logger     *zap.Logger
	checker    *health.Checker
	forwarders map[string]*Forwarder
}

type upstreamKey struct{}
type upstreamValue struct {
	value string
}

// New creates a new Proxy. checker and forwarders must be pre-built from the same route list.
func New(router *routing.Router, logger *zap.Logger, checker *health.Checker, forwarders map[string]*Forwarder) *Proxy {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Proxy{
		router:     router,
		logger:     logger,
		checker:    checker,
		forwarders: forwarders,
	}
}

func WithUpstream(ctx context.Context, upstream string) context.Context {
	if value, ok := ctx.Value(upstreamKey{}).(*upstreamValue); ok {
		value.value = upstream
		return ctx
	}
	return context.WithValue(ctx, upstreamKey{}, &upstreamValue{value: upstream})
}

func UpstreamFromContext(ctx context.Context) string {
	switch value := ctx.Value(upstreamKey{}).(type) {
	case *upstreamValue:
		return value.value
	case string:
		return value
	}
	return ""
}

// ServeHTTP matches the request path, checks upstream health, and delegates to the Forwarder.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upstream, ok := p.router.Match(r.URL.Path)
	if !ok {
		http.Error(w, "no route matched", http.StatusNotFound)
		return
	}

	if !p.checker.IsHealthy(upstream) {
		p.logger.Warn("upstream marked unhealthy, rejecting request",
			zap.String("upstream", upstream),
			zap.String("path", r.URL.Path),
		)
		http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
		return
	}

	r = r.WithContext(WithUpstream(r.Context(), upstream))

	fwd, ok := p.forwarders[upstream]
	if !ok {
		p.logger.Error("no forwarder for upstream", zap.String("upstream", upstream))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	fwd.Do(w, r)
}
