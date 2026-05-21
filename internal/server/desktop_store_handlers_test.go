package server

import (
	"context"
	"testing"
	"time"
)

func TestDesktopStoreOperationContextCancelsOnShutdown(t *testing.T) {
	shutdownCh := make(chan struct{})
	ctx, cancel := desktopStoreOperationContext(shutdownCh, time.Minute)
	defer cancel()

	close(shutdownCh)
	select {
	case <-ctx.Done():
		if ctx.Err() != context.Canceled {
			t.Fatalf("context error = %v, want canceled", ctx.Err())
		}
	case <-time.After(time.Second):
		t.Fatal("operation context did not cancel on shutdown")
	}
}
