package main

import (
	"errors"
	"testing"

	"go.uber.org/zap"
)

func TestRun_ReturnsExitCodeOneWhenLoggerInitializationFails(t *testing.T) {
	originalNewLogger := newLogger
	defer func() {
		newLogger = originalNewLogger
	}()

	newLogger = func() (*zap.Logger, error) {
		return nil, errors.New("logger init failed")
	}

	if code := run(); code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}
