package proxy

import (
	"sync"
	"time"
)

type cbState int

const (
	cbClosed   cbState = iota
	cbOpen
	cbHalfOpen
)

type CircuitBreaker struct {
	mu               sync.Mutex
	state            cbState
	failureCount     int
	failureThreshold int
	recoveryTimeout  time.Duration
	openedAt         time.Time
}

func NewCircuitBreaker(threshold int, recoveryTimeout time.Duration) *CircuitBreaker {
	if threshold < 1 {
		threshold = 5
	}
	if recoveryTimeout <= 0 {
		recoveryTimeout = 30 * time.Second
	}
	return &CircuitBreaker{
		failureThreshold: threshold,
		recoveryTimeout:  recoveryTimeout,
	}
}

// Allow returns true if the request should proceed.
// Transitions Open→HalfOpen when the recovery timeout elapses.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case cbClosed:
		return true
	case cbOpen:
		if time.Since(cb.openedAt) >= cb.recoveryTimeout {
			cb.state = cbHalfOpen
			return true
		}
		return false
	case cbHalfOpen:
		return false
	}
	return false
}

// RecordSuccess resets the circuit to closed.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = cbClosed
	cb.failureCount = 0
}

// RecordFailure increments the failure counter and opens the circuit when the threshold is reached.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount++
	if cb.state == cbHalfOpen || cb.failureCount >= cb.failureThreshold {
		cb.state = cbOpen
		cb.openedAt = time.Now()
		cb.failureCount = 0
	}
}
