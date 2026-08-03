package server

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"aurago/internal/tsnetnode"
)

func TestRetryTsNetStartupRetriesTransientTimeout(t *testing.T) {
	attempts := 0
	err := retryTsNetStartup(context.Background(), 0, func() error {
		attempts++
		if attempts == 1 {
			return fmt.Errorf("%s: persisted node start timed out", tsnetnode.ErrorTimeout)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retryTsNetStartup() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestRetryTsNetStartupDoesNotRetryAuthenticationFailure(t *testing.T) {
	attempts := 0
	err := retryTsNetStartup(context.Background(), 0, func() error {
		attempts++
		return fmt.Errorf("%s: login required", tsnetnode.ErrorLoginRequired)
	})
	if err == nil || tsnetPublicErrorCode(err) != tsnetnode.ErrorLoginRequired {
		t.Fatalf("retryTsNetStartup() error = %v, want login required", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestRetryTsNetStartupStopsDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	err := retryTsNetStartup(ctx, time.Minute, func() error {
		attempts++
		cancel()
		return fmt.Errorf("%s: transient backend failure", tsnetnode.ErrorBackendUnavailable)
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("retryTsNetStartup() error = %v, want context cancellation", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}
