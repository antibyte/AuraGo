package server

import (
	"context"
	"testing"
)

// A job that has already seen a ready browser, or that used its attempts, or whose context is cancelled, never calls Ensure.
func TestEnsureBrowserSkipsWhenItShould(t *testing.T) {
	j := &storymakerAgentJob{browserReady: true}
	j.ensureBrowser(context.Background()) // would panic on the nil runtime if it went on

	j = &storymakerAgentJob{browserAttempts: storymakerBrowserMaxAttempts}
	j.ensureBrowser(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	j = &storymakerAgentJob{}
	j.ensureBrowser(ctx)
	if j.browserAttempts != 0 {
		t.Fatalf("a cancelled job must not count an attempt: %d", j.browserAttempts)
	}
}
