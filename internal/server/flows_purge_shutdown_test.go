package server

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"aurago/internal/flows"
)

func TestFlowShutdownDefersStoreCloseUntilPurgePreflightDrains(t *testing.T) {
	s := c16StartedServer(t, nil)
	previous := s.Flows
	if err := previous.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	entered, resume := make(chan struct{}), make(chan struct{})
	released := false
	t.Cleanup(func() {
		if !released {
			close(resume)
		}
	})
	s.Flows = flows.NewService(previous.Store(), previous.Registry(), &flows.Services{}, flowMissionBridge{s: s}, flows.ServiceConfig{PurgeKey: func() string {
		close(entered)
		<-resume
		return strings.Repeat("a", 64)
	}}, s.Logger)
	started := make(chan error, 1)
	go func() { _, err := s.Flows.StartCredentialPurge(nil); started <- err }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("preflight did not start")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// The server ignores cancellation but keeps its existing five-second minimum.
	ctx, deadlineCancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
	defer deadlineCancel()
	s.shutdownFlows(ctx)
	if _, err := s.Flows.Store().GetFlow(context.Background(), "flow_missing000"); !errors.Is(err, flows.ErrNotFound) {
		t.Fatalf("store closed before preflight ended: %v", err)
	}
	close(resume)
	released = true
	if err := <-started; !errors.Is(err, flows.ErrRunnerClosed) {
		t.Fatalf("purge admitted after shutdown: %v", err)
	}
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		_, err := s.Flows.Store().GetFlow(context.Background(), "flow_missing000")
		if err != nil && strings.Contains(err.Error(), "closed") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("drained store never closed: %v", err)
		}
	}
}
