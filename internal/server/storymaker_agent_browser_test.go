package server

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/storymaker/render"
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

func TestStorymakerRuntimeBrowserBackoffWindow(t *testing.T) {
	rt := &storymakerRuntime{}
	now := time.Now()
	if rt.browserBackedOff(now) {
		t.Fatal("a fresh runtime has no back-off")
	}
	rt.noteBrowserFailure(now)
	if !rt.browserBackedOff(now) || !rt.browserBackedOff(now.Add(storymakerBrowserBackoff-time.Second)) {
		t.Fatal("a failure must hold the jobs back for the whole back-off")
	}
	if rt.browserBackedOff(now.Add(storymakerBrowserBackoff + time.Second)) {
		t.Fatal("the back-off must end after storymakerBrowserBackoff")
	}
}

// A failed setup holds back the next jobs of the runtime without a new attempt; the back-off ends by time, and a ready
// browser clears it.
func TestEnsureBrowserBacksOffAfterAFailedSetup(t *testing.T) {
	ctx := context.Background()

	// A backed-off runtime without a resolver: the job returns before it counts an attempt.
	held := &storymakerRuntime{}
	held.noteBrowserFailure(time.Now())
	j := &storymakerAgentJob{rt: held}
	j.ensureBrowser(ctx)
	if j.browserAttempts != 0 {
		t.Fatalf("a backed-off runtime must not count an attempt: %d", j.browserAttempts)
	}

	lookups, ready := 0, false
	rt := &storymakerRuntime{browsers: render.NewBrowsersWithProbe(t.TempDir(),
		func() (string, bool) {
			lookups++
			return "system-browser", ready
		},
		func(context.Context, string) (string, error) { return "Fake/1.0", nil })}
	newJob := func() *storymakerAgentJob {
		return &storymakerAgentJob{rt: rt, route: &storymakerAgentRoute{cfg: &config.Config{}}, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	}

	// The first job finds no browser and its downloads are switched off: one attempt, and the back-off starts.
	first := newJob()
	first.ensureBrowser(ctx)
	if first.browserAttempts != 1 || first.browserReady || lookups != 1 || !rt.browserBackedOff(time.Now()) {
		t.Fatalf("after a failed setup: attempts %d, ready %v, lookups %d, backed off %v", first.browserAttempts, first.browserReady, lookups, rt.browserBackedOff(time.Now()))
	}

	// The next job does not ask the resolver at all.
	second := newJob()
	second.ensureBrowser(ctx)
	if second.browserAttempts != 0 || lookups != 1 {
		t.Fatalf("a job during the back-off: attempts %d, lookups %d", second.browserAttempts, lookups)
	}

	// When the back-off is over the job asks again; a browser that is ready by then clears the back-off.
	rt.browserRetryAfter.Store(time.Now().Add(-time.Second).UnixNano())
	ready = true
	third := newJob()
	third.ensureBrowser(ctx)
	if third.browserAttempts != 1 || !third.browserReady || lookups != 2 || rt.browserRetryAfter.Load() != 0 {
		t.Fatalf("after the back-off: attempts %d, ready %v, lookups %d, retry-after %d", third.browserAttempts, third.browserReady, lookups, rt.browserRetryAfter.Load())
	}

	// A job that is stopped while the browser is being set up is no failure: it must not start a back-off.
	stopped, cancel := context.WithCancel(ctx)
	defer cancel()
	rt2 := &storymakerRuntime{browsers: render.NewBrowsersWithProbe(t.TempDir(),
		func() (string, bool) { cancel(); return "", false },
		func(context.Context, string) (string, error) { return "", nil })}
	j = &storymakerAgentJob{rt: rt2, route: &storymakerAgentRoute{cfg: &config.Config{}}, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	j.ensureBrowser(stopped)
	if j.browserAttempts != 1 || rt2.browserBackedOff(time.Now()) {
		t.Fatalf("a job stopped during the setup: attempts %d, backed off %v", j.browserAttempts, rt2.browserBackedOff(time.Now()))
	}
}
