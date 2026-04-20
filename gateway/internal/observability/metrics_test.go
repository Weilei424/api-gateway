package observability_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gateway/internal/observability"
)

func TestMetrics_MiddlewareRecordsRequestsAndLatency(t *testing.T) {
	metrics, err := observability.NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics returned error: %v", err)
	}

	handler := metrics.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	handler.ServeHTTP(rec, req)

	metricsRec := httptest.NewRecorder()
	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metrics.Handler().ServeHTTP(metricsRec, metricsReq)

	body := metricsRec.Body.String()
	if !strings.Contains(body, "gateway_requests_total") {
		t.Fatal("expected request counter in metrics output")
	}
	if !strings.Contains(body, "gateway_request_duration_seconds") {
		t.Fatal("expected duration histogram in metrics output")
	}
}
