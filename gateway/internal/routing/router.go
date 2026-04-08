package routing

import (
	"gateway/internal/config"
	"sort"
	"strings"
)

type Router struct {
	routes []config.Route
}

// New creates a new Router with the given routes, sorted longest-first by path prefix.
func New(routes []config.Route) *Router {
	sorted := make([]config.Route, len(routes))
	copy(sorted, routes)

	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i].Path) > len(sorted[j].Path)
	})

	return &Router{routes: sorted}
}

// Match finds the upstream URL for the longest matching path prefix.
func (r *Router) Match(path string) (string, bool) {
	for _, route := range r.routes {
		if strings.HasPrefix(path, route.Path) {
			return route.Upstream, true
		}
	}

	return "", false
}
