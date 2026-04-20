package proxy

import (
	"context"
	"fmt"
	"gateway/internal/routing"
	"net/http"
	"net/http/httputil"
	"net/url"

	"go.uber.org/zap"
)

type Proxy struct {
	router *routing.Router
	logger *zap.Logger
}

type upstreamKey struct{}
type upstreamValue struct {
	value string
}

// New creates a new Proxy with the given Router.
func New(router *routing.Router, logger *zap.Logger) *Proxy {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Proxy{router: router, logger: logger}
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

// ServeHTTP implements the http.Handler interface. It matches the incoming request path
// against the configured routes and proxies to the appropriate upstream service.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upstream, ok := p.router.Match(r.URL.Path)
	if !ok {
		http.Error(w, "no route matched", http.StatusNotFound)
		return
	}

	target, err := url.Parse(upstream)
	if err != nil {
		p.logger.Error("invalid upstream URL",
			zap.String("upstream", upstream),
			zap.String("path", r.URL.Path),
			zap.Error(err),
		)
		http.Error(w, fmt.Sprintf("invalid upstream URL: %v", err), http.StatusInternalServerError)
		return
	}

	r = r.WithContext(WithUpstream(r.Context(), upstream))

	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		},
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			p.logger.Error("upstream proxy error",
				zap.String("upstream", UpstreamFromContext(req.Context())),
				zap.String("path", req.URL.Path),
				zap.Error(err),
			)
			http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
		},
	}
	rp.ServeHTTP(w, r)
}
