package server

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/videostudio"
)

func TestStorymakerFFmpegCache(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	var clockMu sync.Mutex
	cache := &storymakerFFmpegCache{
		check: func(_ context.Context, path string) videostudio.RuntimeStatus {
			calls.Add(1)
			return videostudio.RuntimeStatus{Ready: path == "good", FFmpegPath: path}
		},
		now: func() time.Time { clockMu.Lock(); defer clockMu.Unlock(); return now },
	}
	advance := func(d time.Duration) { clockMu.Lock(); now = now.Add(d); clockMu.Unlock() }
	ctx := context.Background()

	if !cache.status(ctx, "good").Ready || !cache.status(ctx, "good").Ready || calls.Load() != 1 {
		t.Fatalf("two calls within the window must run one check, ran %d", calls.Load())
	}
	if cache.status(ctx, "bad").Ready || calls.Load() != 2 {
		t.Fatalf("a different path must run a new check, ran %d", calls.Load())
	}
	if !cache.status(ctx, "good").Ready || calls.Load() != 3 {
		t.Fatalf("returning to the first path runs a new check, ran %d", calls.Load())
	}
	advance(storymakerFFmpegCacheFor - time.Second)
	cache.status(ctx, "good")
	if calls.Load() != 3 {
		t.Fatalf("an entry inside the window is reused, ran %d", calls.Load())
	}
	advance(2 * time.Second)
	cache.status(ctx, "good")
	if calls.Load() != 4 {
		t.Fatalf("an expired entry runs a new check, ran %d", calls.Load())
	}
	// A failed check is retried after the window too.
	cache.status(ctx, "bad")
	cache.status(ctx, "bad")
	if calls.Load() != 5 {
		t.Fatalf("a failed result is cached inside the window, ran %d", calls.Load())
	}
	advance(storymakerFFmpegCacheFor + time.Second)
	cache.status(ctx, "bad")
	if calls.Load() != 6 {
		t.Fatalf("a failed result is retried after the window, ran %d", calls.Load())
	}
	// An aborted request caches nothing.
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	cache.status(cancelled, "other")
	cache.status(ctx, "other")
	if calls.Load() != 8 {
		t.Fatalf("an aborted check must not be cached, ran %d", calls.Load())
	}
}

func TestStorymakerFFmpegCacheConcurrent(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	cache := &storymakerFFmpegCache{
		check: func(context.Context, string) videostudio.RuntimeStatus {
			calls.Add(1)
			time.Sleep(20 * time.Millisecond)
			return videostudio.RuntimeStatus{Ready: true}
		},
		now: time.Now,
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); cache.status(context.Background(), "p") }()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("concurrent calls must share one check, ran %d", calls.Load())
	}
}

// The status endpoint answers from the cache; a changed path is checked at once.
func TestStorymakerStatusUsesFFmpegCache(t *testing.T) {
	s, readToken, _ := storymakerAPIFixture(t)
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	rt.ffmpeg.check = func(_ context.Context, path string) videostudio.RuntimeStatus {
		calls.Add(1)
		return videostudio.RuntimeStatus{Ready: true, Version: "ffmpeg version 9.9 Copyright (c) test"}
	}
	for i := 0; i < 2; i++ {
		storymakerRequest(s, readToken, http.MethodGet, "/status", nil, nil)
	}
	if calls.Load() != 1 {
		t.Fatalf("two status calls ran %d checks", calls.Load())
	}
	s.Cfg.VideoStudio.FFmpegPath = "other"
	w := storymakerRequest(s, readToken, http.MethodGet, "/status", nil, nil)
	if calls.Load() != 2 || !strings.Contains(w.Body.String(), `"version":"9.9"`) {
		t.Fatalf("a changed path must be checked at once (%d checks): %s", calls.Load(), w.Body.String())
	}
}
