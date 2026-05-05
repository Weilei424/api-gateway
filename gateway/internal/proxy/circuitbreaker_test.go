package proxy

import (
	"testing"
	"time"
)

func TestCircuitBreaker_ClosedAllowsRequests(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Second)
	if !cb.Allow() {
		t.Fatal("expected closed circuit to allow request")
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Second)
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.Allow() {
		t.Fatal("expected open circuit to reject request")
	}
}

func TestCircuitBreaker_SuccessResetsClosed(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Second)
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()
	cb.RecordFailure()
	cb.RecordFailure()
	// Only 2 failures since last success — should still be closed
	if !cb.Allow() {
		t.Fatal("expected circuit to remain closed after success reset")
	}
}

func TestCircuitBreaker_HalfOpenAfterRecoveryTimeout(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)
	cb.RecordFailure() // opens
	if cb.Allow() {
		t.Fatal("expected open circuit to reject immediately after opening")
	}
	time.Sleep(60 * time.Millisecond)
	// First Allow() after timeout transitions to half-open and returns true
	if !cb.Allow() {
		t.Fatal("expected half-open circuit to allow one test request")
	}
	// Second Allow() while half-open must return false
	if cb.Allow() {
		t.Fatal("expected half-open circuit to reject subsequent requests")
	}
}

func TestCircuitBreaker_HalfOpenSuccessCloses(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)
	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)
	cb.Allow() // enter half-open
	cb.RecordSuccess()
	if !cb.Allow() {
		t.Fatal("expected circuit to close after half-open success")
	}
}

func TestCircuitBreaker_HalfOpenFailureReopens(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)
	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)
	cb.Allow() // enter half-open
	cb.RecordFailure()
	if cb.Allow() {
		t.Fatal("expected circuit to reopen after half-open failure")
	}
}
