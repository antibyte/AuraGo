package gamemaker

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"
)

func TestPublicationWaitsForPolicyBeforeTakingBuildLock(t *testing.T) {
	s := newTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Hold the policy writer so publication must stop at policyMu. Its source
	// lock should already be held, while buildMu remains available to an import
	// that took the policy read lock first.
	s.policyMu.Lock()
	policyHeld := true
	defer func() {
		if policyHeld {
			s.policyMu.Unlock()
		}
	}()

	done := make(chan error, 1)
	started := make(chan struct{})
	go func() {
		close(started)
		_, err := s.publishValidated(ctx, "", Project{}, Job{}, BuildResult{})
		done <- err
	}()
	<-started

	deadline := time.Now().Add(time.Second)
	for s.fileMu.TryLock() {
		s.fileMu.Unlock()
		if time.Now().After(deadline) {
			cancel()
			s.policyMu.Unlock()
			policyHeld = false
			<-done
			t.Fatal("publication did not acquire the source lock")
		}
		runtime.Gosched()
	}

	// Give the publication goroutine time to reach its policy wait. Under the
	// inverse build-before-policy order, it would already own buildMu here.
	time.Sleep(20 * time.Millisecond)
	buildAvailable := s.buildMu.TryLock()
	if buildAvailable {
		s.buildMu.Unlock()
	}

	cancel()
	s.policyMu.Unlock()
	policyHeld = false
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("publication returned %v, want context cancellation after lock release", err)
		}
	case <-time.After(time.Second):
		t.Fatal("publication remained blocked after policy lock release")
	}
	if !buildAvailable {
		t.Fatal("publication held buildMu while waiting for policyMu")
	}
}
