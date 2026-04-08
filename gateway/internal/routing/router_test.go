package routing_test

import (
	"gateway/internal/config"
	"gateway/internal/routing"
	"testing"
)

func TestMatch_LongestPrefixWins(t *testing.T) {
	routes := []config.Route{
		{Path: "/api", Upstream: "http://api:9001"},
		{Path: "/api/users", Upstream: "http://users:9002"},
	}
	r := routing.New(routes)

	upstream, ok := r.Match("/api/users/123")
	if !ok {
		t.Fatal("expected a match")
	}
	if upstream != "http://users:9002" {
		t.Fatalf("expected users upstream, got %q", upstream)
	}
}

func TestMatch_ExactPrefix(t *testing.T) {
	routes := []config.Route{
		{Path: "/orders", Upstream: "http://orders:9003"},
	}
	r := routing.New(routes)

	upstream, ok := r.Match("/orders/42")
	if !ok {
		t.Fatal("expected a match")
	}
	if upstream != "http://orders:9003" {
		t.Fatalf("expected orders upstream, got %q", upstream)
	}
}

func TestMatch_NoMatch(t *testing.T) {
	routes := []config.Route{
		{Path: "/users", Upstream: "http://users:9002"},
	}
	r := routing.New(routes)

	_, ok := r.Match("/unknown/path")
	if ok {
		t.Fatal("expected no match")
	}
}

func TestMatch_EmptyRoutes(t *testing.T) {
	r := routing.New([]config.Route{})

	_, ok := r.Match("/anything")
	if ok {
		t.Fatal("expected no match for empty router")
	}
}
