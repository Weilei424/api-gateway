package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeout_CancelsHandlerContextOnTimeout(t *testing.T) {
	cancelled := make(chan struct{})
	handler := Timeout(30*time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		close(cancelled)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	select {
	case <-cancelled:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("handler context was not cancelled after timeout fired")
	}
}

func TestTimeout_FlushPropagatesWhenNotTimedOut(t *testing.T) {
	handler := Timeout(100*time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			t.Error("timeoutWriter must implement http.Flusher when underlying writer does")
			return
		}
		f.Flush()
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !rec.Flushed {
		t.Error("expected Flush to propagate to the underlying ResponseWriter")
	}
}

func TestTimeout_ResponseControllerFlushesViaUnwrap(t *testing.T) {
	handler := Timeout(100*time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)
		if err := rc.Flush(); err != nil {
			t.Errorf("ResponseController.Flush should succeed via Unwrap, got: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !rec.Flushed {
		t.Error("expected ResponseController.Flush to propagate via Unwrap chain")
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

func TestTimeout_WritesRejectedAfterTimeoutEvenIfStarted(t *testing.T) {
	goroutineDone := make(chan error, 1)

	handler := Timeout(30*time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		<-r.Context().Done()
		_, err := w.Write([]byte("after timeout"))
		goroutineDone <- err
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	select {
	case err := <-goroutineDone:
		if err == nil {
			t.Error("expected write after timeout to return an error, but got nil")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handler goroutine did not complete in time")
	}
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
