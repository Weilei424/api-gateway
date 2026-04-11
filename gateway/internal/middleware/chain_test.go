package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"gateway/internal/middleware"
)

func TestChain_AppliesMiddlewareInDeclarationOrder(t *testing.T) {
	var calls []string

	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls = append(calls, "mw1-before")
			next.ServeHTTP(w, r)
			calls = append(calls, "mw1-after")
		})
	}
	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls = append(calls, "mw2-before")
			next.ServeHTTP(w, r)
			calls = append(calls, "mw2-after")
		})
	}

	handler := middleware.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, "handler")
		w.WriteHeader(http.StatusNoContent)
	}), mw1, mw2)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	want := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("unexpected call order: got %v want %v", calls, want)
	}
}
