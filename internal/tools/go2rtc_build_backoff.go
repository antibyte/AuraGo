package tools

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	// go2RTCBuildRetryInitial is the wait after the first failed build of the
	// managed sidecar image before the background reconciler tries again.
	go2RTCBuildRetryInitial = time.Minute
	// go2RTCBuildRetryMax caps the doubling wait (1, 2, 4, 8, 16, 30, 30, ...
	// minutes). The managed image is a long Go build that downloads source and
	// modules, so a permanent failure must not be retried every 30 seconds.
	go2RTCBuildRetryMax = 30 * time.Minute
)

// go2RTCAutomaticAttemptKey marks a context as belonging to the background
// reconciler rather than to an administrator's explicit action.
type go2RTCAutomaticAttemptKey struct{}

// withGo2RTCAutomaticAttempt marks ctx as a background reconcile attempt. Only
// those attempts honour the build backoff; an explicit start, restart or
// config save always builds immediately and resets the backoff.
func withGo2RTCAutomaticAttempt(ctx context.Context) context.Context {
	return context.WithValue(ctx, go2RTCAutomaticAttemptKey{}, true)
}

func isGo2RTCAutomaticAttempt(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	automatic, _ := ctx.Value(go2RTCAutomaticAttemptKey{}).(bool)
	return automatic
}

// go2RTCBuildPausedError is returned instead of a build while the backoff after
// a failed managed image build is running. It wraps the failure that started it.
type go2RTCBuildPausedError struct {
	RetryAt time.Time
	Err     error
}

func (e *go2RTCBuildPausedError) Error() string {
	return fmt.Sprintf("go2rtc sidecar image build is paused after a failed attempt (next automatic retry at %s; start or restart go2rtc to retry now): %v",
		e.RetryAt.UTC().Format(time.RFC3339), e.Err)
}

func (e *go2RTCBuildPausedError) Unwrap() error { return e.Err }

// go2RTCBuildRetryDelay is the wait after the given number of consecutive
// failed builds: go2RTCBuildRetryInitial, doubling, up to go2RTCBuildRetryMax.
func go2RTCBuildRetryDelay(failures int) time.Duration {
	delay := go2RTCBuildRetryInitial
	for i := 1; i < failures && delay < go2RTCBuildRetryMax; i++ {
		delay *= 2
	}
	if delay > go2RTCBuildRetryMax {
		delay = go2RTCBuildRetryMax
	}
	return delay
}

// go2RTCBuildBackoff remembers consecutive failed builds of one managed image
// target. The zero value is ready to use. The target key is the image tag plus
// the Docker host, so a changed config or Dockerfile (a new tag) or another
// engine starts over without anything having to reset the backoff explicitly.
type go2RTCBuildBackoff struct {
	mu       sync.Mutex
	now      func() time.Time // injected clock; time.Now when nil
	target   string
	failures int
	retryAt  time.Time
	lastErr  error
}

func (b *go2RTCBuildBackoff) clock() time.Time {
	if b.now != nil {
		return b.now()
	}
	return time.Now()
}

// paused reports whether a background attempt for target must wait, with the
// time the wait ends and the failure that caused it. A backoff recorded for a
// different target does not apply.
func (b *go2RTCBuildBackoff) paused(target string) (retryAt time.Time, cause error, paused bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.failures == 0 || b.target != target || !b.clock().Before(b.retryAt) {
		return time.Time{}, nil, false
	}
	return b.retryAt, b.lastErr, true
}

// failed records a failed build of target and returns the number of
// consecutive failures, the wait before the next background attempt and when
// that wait ends. A failure of another target than the recorded one starts at
// the first step again.
func (b *go2RTCBuildBackoff) failed(target string, err error) (failures int, delay time.Duration, retryAt time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.target != target {
		b.target = target
		b.failures = 0
	}
	b.failures++
	delay = go2RTCBuildRetryDelay(b.failures)
	b.retryAt = b.clock().Add(delay)
	b.lastErr = err
	return b.failures, delay, b.retryAt
}

// reset forgets all recorded failures.
func (b *go2RTCBuildBackoff) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.target = ""
	b.failures = 0
	b.retryAt = time.Time{}
	b.lastErr = nil
}
