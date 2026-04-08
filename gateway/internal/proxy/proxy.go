package proxy

import (
	"fmt"
	"gateway/internal/routing"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type Proxy struct {
	router *routing.Router
}

// New creates a new Proxy with the given Router.
func New(router *routing.Router) *Proxy {
	return &Proxy{router: router}
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
		http.Error(w, fmt.Sprintf("invalid upstream URL: %v", err), http.StatusInternalServerError)
		return
	}

	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		},
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
		},
	}
	rp.ServeHTTP(w, r)
}
