package health

import (
	"context"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Checker struct {
	upstreams []string
	interval  time.Duration
	logger    *zap.Logger
	mu        sync.RWMutex
	healthy   map[string]bool
	client    *http.Client
}

func NewChecker(upstreams []string, interval time.Duration, logger *zap.Logger) *Checker {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	c := &Checker{
		upstreams: upstreams,
		interval:  interval,
		logger:    logger,
		healthy:   make(map[string]bool),
		client:    &http.Client{Timeout: 5 * time.Second},
	}
	for _, u := range upstreams {
		c.healthy[u] = true
	}
	return c
}

// Start launches a background goroutine that polls all upstreams on the configured interval.
// It stops when ctx is cancelled.
func (c *Checker) Start(ctx context.Context) {
	go func() {
		c.pollAll()
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.pollAll()
			}
		}
	}()
}

// IsHealthy returns true if the upstream was healthy on the last poll,
// or if it has never been polled (safe default: healthy).
func (c *Checker) IsHealthy(upstream string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	h, exists := c.healthy[upstream]
	return !exists || h
}

// PollAll immediately polls all upstreams. Exported for testing.
func (c *Checker) PollAll() {
	c.pollAll()
}

func (c *Checker) pollAll() {
	for _, upstream := range c.upstreams {
		healthy := c.poll(upstream)
		c.mu.Lock()
		c.healthy[upstream] = healthy
		c.mu.Unlock()
		if !healthy {
			c.logger.Warn("upstream unhealthy", zap.String("upstream", upstream))
		}
	}
}

func (c *Checker) poll(upstream string) bool {
	resp, err := c.client.Get(upstream + "/")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}
