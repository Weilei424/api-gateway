package observability

import (
	"net/http"
	"strconv"
	"time"

	"gateway/internal/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry        *prometheus.Registry
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
}

func NewMetrics() (*Metrics, error) {
	registry := prometheus.NewRegistry()
	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_requests_total",
			Help: "Total number of handled requests.",
		},
		[]string{"method", "status"},
	)
	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "gateway_request_duration_seconds",
			Help: "Duration of handled requests in seconds.",
		},
		[]string{"method", "status"},
	)

	if err := registry.Register(requestsTotal); err != nil {
		return nil, err
	}
	if err := registry.Register(requestDuration); err != nil {
		return nil, err
	}

	return &Metrics{
		registry:        registry,
		requestsTotal:   requestsTotal,
		requestDuration: requestDuration,
	}, nil
}

func (m *Metrics) Middleware() middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := middleware.NewStatusRecorder(w)

			next.ServeHTTP(rec, r)

			status := strconv.Itoa(rec.Status())
			m.requestsTotal.WithLabelValues(r.Method, status).Inc()
			m.requestDuration.WithLabelValues(r.Method, status).Observe(time.Since(start).Seconds())
		})
	}
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
