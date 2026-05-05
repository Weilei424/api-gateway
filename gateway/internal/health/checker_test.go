package health

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestChecker_StartsHealthy(t *testing.T) {
	c := NewChecker([]string{"http://fake-upstream:9999"}, time.Hour, zap.NewNop())
	if !c.IsHealthy("http://fake-upstream:9999") {
		t.Error("expected upstream to start healthy before any poll")
	}
}

func TestChecker_MarksUnhealthyAfterFailedPoll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewChecker([]string{srv.URL}, time.Hour, zap.NewNop())
	c.PollAll()

	if c.IsHealthy(srv.URL) {
		t.Error("expected upstream to be unhealthy after 500 response")
	}
}

func TestChecker_MarksHealthyAfterSuccessfulPoll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewChecker([]string{srv.URL}, time.Hour, zap.NewNop())
	c.PollAll()

	if !c.IsHealthy(srv.URL) {
		t.Error("expected upstream to be healthy after 200 response")
	}
}

func TestChecker_UnknownUpstreamIsHealthy(t *testing.T) {
	c := NewChecker([]string{}, time.Hour, zap.NewNop())
	if !c.IsHealthy("http://unknown:9999") {
		t.Error("expected unknown upstream to be treated as healthy")
	}
}

func TestChecker_MarksUnhealthyOnConnectionRefused(t *testing.T) {
	c := NewChecker([]string{"http://127.0.0.1:1"}, time.Hour, zap.NewNop())
	c.PollAll()

	if c.IsHealthy("http://127.0.0.1:1") {
		t.Error("expected upstream to be unhealthy when connection is refused")
	}
}
