package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeout_SetsContextDeadline(t *testing.T) {
	var hadDeadline bool
	handler := Timeout(100*time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadDeadline = r.Context().Deadline()
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if !hadDeadline {
		t.Fatal("expected context to have a deadline")
	}
}

func TestTimeout_DeadlineExceeded_Returns504(t *testing.T) {
	handler := Timeout(20*time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("expected 504, got %d", rec.Code)
	}
}

func TestTimeout_HandlerPanic_RePanics(t *testing.T) {
	handler := Timeout(100*time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("downstream panic")
	}))

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected the handler panic to be re-panicked on the serving goroutine")
		}
	}()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)
}

func TestTimeout_Zero_NoDeadline(t *testing.T) {
	var hadDeadline bool
	handler := Timeout(0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadDeadline = r.Context().Deadline()
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if hadDeadline {
		t.Fatal("expected no deadline when timeout is 0")
	}
}
