package middleware

import (
	"net/http"
	"time"

	"gateway/internal/proxy"

	"go.uber.org/zap"
)

func Logging(logger *zap.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := newStatusRecorder(w)

			next.ServeHTTP(rec, r)

			logger.Info("request completed",
				zap.String("request_id", RequestIDFromContext(r.Context())),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", rec.Status()),
				zap.Duration("latency", time.Since(start)),
				zap.String("upstream", proxy.UpstreamFromContext(r.Context())),
			)
		})
	}
}
