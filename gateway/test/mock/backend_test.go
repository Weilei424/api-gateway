package mock_test

import (
	"net/http"
	"testing"
	"time"

	"gateway/test/mock"
)

func TestBackend_ServesConfiguredStatus(t *testing.T) {
	b := mock.New(mock.HandlerConfig{StatusCode: 201, Body: "created"})
	defer b.Close()

	resp, err := http.Get(b.URL() + "/anything")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
	if b.RequestCount() != 1 {
		t.Errorf("expected 1 request, got %d", b.RequestCount())
	}
}

func TestBackend_DelaysResponse(t *testing.T) {
	b := mock.New(mock.HandlerConfig{StatusCode: 200, Delay: 80 * time.Millisecond})
	defer b.Close()

	start := time.Now()
	resp, err := http.Get(b.URL() + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if elapsed := time.Since(start); elapsed < 80*time.Millisecond {
		t.Errorf("expected delay >= 80ms, got %v", elapsed)
	}
}

func TestBackend_FailFirst(t *testing.T) {
	b := mock.New(mock.HandlerConfig{StatusCode: 200, FailFirst: 2})
	defer b.Close()

	for i, want := range []int{500, 500, 200} {
		resp, err := http.Get(b.URL() + "/")
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("request %d: expected %d, got %d", i, want, resp.StatusCode)
		}
	}
}

func TestBackend_SetHandler(t *testing.T) {
	b := mock.New(mock.HandlerConfig{StatusCode: 200})
	defer b.Close()

	b.SetHandler(mock.HandlerConfig{StatusCode: 503})

	resp, err := http.Get(b.URL() + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 503 {
		t.Errorf("expected 503 after SetHandler, got %d", resp.StatusCode)
	}
}
