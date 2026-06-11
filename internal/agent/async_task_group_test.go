package agent

import (
	"context"
	"testing"
	"time"
)

func TestAsyncTaskGroupShutdownCancelsAndDrains(t *testing.T) {
	group := NewAsyncTaskGroup(context.Background())
	started := make(chan struct{})
	done := make(chan struct{})

	if !group.Go(func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		close(done)
	}) {
		t.Fatal("expected task to start")
	}
	<-started

	if err := group.Shutdown(time.Second); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("task did not observe cancellation")
	}
	if group.Go(func(context.Context) {}) {
		t.Fatal("expected closed group to reject new tasks")
	}
}
