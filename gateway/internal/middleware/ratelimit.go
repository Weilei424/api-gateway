package middleware

import (
	"net"
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

type rateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	rps      rate.Limit
	burst    int
}

func RateLimit(rps float64, burst int) Middleware {
	if rps <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	rl := &rateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			if !rl.getLimiter(ip).Allow() {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (rl *rateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if l, ok := rl.limiters[key]; ok {
		return l
	}
	l := rate.NewLimiter(rl.rps, rl.burst)
	rl.limiters[key] = l
	return l
}
