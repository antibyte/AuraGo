package tools

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
)

// go2RTCTestClock is the injected clock of the build backoff.
type go2RTCTestClock struct {
	mu  sync.Mutex
	now time.Time
}

func newGo2RTCTestClock() *go2RTCTestClock {
	return &go2RTCTestClock{now: time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)}
}

func (c *go2RTCTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *go2RTCTestClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// go2RTCBuildFake is a Docker Engine fake that has no sidecar container and no
// managed image, and fails or accepts every image build.
type go2RTCBuildFake struct {
	mu          sync.Mutex
	builds      int
	tags        []string
	failBuilds  bool
	imageExists bool
}

func (f *go2RTCBuildFake) buildCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.builds
}

func (f *go2RTCBuildFake) tagList() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.tags...)
}

func (f *go2RTCBuildFake) set(fail, imageExists bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failBuilds = fail
	f.imageExists = imageExists
}

func (f *go2RTCBuildFake) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		path := r.URL.Path
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/build"):
			_, _ = io.Copy(io.Discard, r.Body)
			f.builds++
			f.tags = append(f.tags, r.URL.Query().Get("t"))
			if f.failBuilds {
				_, _ = w.Write([]byte(`{"errorDetail":{"code":127,"message":"The command returned a non-zero code: 127"},"error":"The command returned a non-zero code: 127"}` + "\n"))
				return
			}
			_, _ = w.Write([]byte(`{"stream":"built"}` + "\n"))
		case r.Method == http.MethodGet && strings.Contains(path, "/images/") && strings.HasSuffix(path, "/json"):
			if f.imageExists {
				_, _ = w.Write([]byte(`{}`))
				return
			}
			http.NotFound(w, r)
		case r.Method == http.MethodGet && strings.Contains(path, "/containers/") && strings.HasSuffix(path, "/json"):
			http.NotFound(w, r)
		default:
			t.Errorf("unexpected Docker request %s %s", r.Method, r.URL.String())
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}
}

// go2RTCLogBuffer is a goroutine-safe slog sink.
type go2RTCLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *go2RTCLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *go2RTCLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *go2RTCLogBuffer) logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(b, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestGo2RTCBuildRetryDelayDoublesUpToTheCap(t *testing.T) {
	for _, tc := range []struct {
		failures int
		want     time.Duration
	}{
		{0, time.Minute},
		{1, time.Minute},
		{2, 2 * time.Minute},
		{3, 4 * time.Minute},
		{4, 8 * time.Minute},
		{5, 16 * time.Minute},
		{6, 30 * time.Minute},
		{7, 30 * time.Minute},
		{1000, 30 * time.Minute},
	} {
		if got := go2RTCBuildRetryDelay(tc.failures); got != tc.want {
			t.Errorf("go2RTCBuildRetryDelay(%d) = %v, want %v", tc.failures, got, tc.want)
		}
	}
}

func TestGo2RTCBuildBackoffFollowsTheInjectedClock(t *testing.T) {
	clock := newGo2RTCTestClock()
	b := &go2RTCBuildBackoff{now: clock.Now}
	cause := errors.New("exit 127")

	if _, _, paused := b.paused("image-a@host"); paused {
		t.Fatal("a backoff without failures pauses")
	}
	failures, delay, retryAt := b.failed("image-a@host", cause)
	if failures != 1 || delay != time.Minute || !retryAt.Equal(clock.Now().Add(time.Minute)) {
		t.Fatalf("first failure = %d, %v, %v", failures, delay, retryAt)
	}
	clock.Advance(59 * time.Second)
	if _, got, paused := b.paused("image-a@host"); !paused || got != cause {
		t.Fatalf("after 59s paused=%v cause=%v, want paused with the failure", paused, got)
	}
	clock.Advance(time.Second)
	if _, _, paused := b.paused("image-a@host"); paused {
		t.Fatal("still paused when the wait is over")
	}

	// The retry fails as well: the wait doubles.
	if failures, delay, _ = b.failed("image-a@host", cause); failures != 2 || delay != 2*time.Minute {
		t.Fatalf("second failure = %d, %v", failures, delay)
	}
	// Another image tag (new config or Dockerfile) or Docker host is not paused
	// and starts over with the short wait.
	if _, _, paused := b.paused("image-b@host"); paused {
		t.Fatal("a new image tag inherited the backoff")
	}
	if _, _, paused := b.paused("image-a@other-host"); paused {
		t.Fatal("a new Docker host inherited the backoff")
	}
	if failures, delay, _ = b.failed("image-b@host", cause); failures != 1 || delay != time.Minute {
		t.Fatalf("failure of a new target = %d, %v, want a fresh first failure", failures, delay)
	}
	b.reset()
	if _, _, paused := b.paused("image-b@host"); paused {
		t.Fatal("reset left the backoff in place")
	}
	if failures, _, _ = b.failed("image-b@host", cause); failures != 1 {
		t.Fatalf("failure after reset = %d, want 1", failures)
	}
}

func TestGo2RTCManagedImageBuildBacksOffAfterAFailure(t *testing.T) {
	fake := &go2RTCBuildFake{failBuilds: true}
	docker := DockerConfig{Host: fakeDockerHost(t, fake.handler(t))}
	logs := &go2RTCLogBuffer{}
	m := NewGo2RTCManager(&config.Config{}, nil, nil, logs.logger())
	defer m.Close()
	clock := newGo2RTCTestClock()
	m.buildBackoff.now = clock.Now
	image := go2RTCManagedImage(config.Go2RTCDefaultImage)
	auto := withGo2RTCAutomaticAttempt(context.Background())

	// First automatic attempt: builds, fails, is logged once at WARN with the
	// error and the next retry time.
	err := m.ensureManagedSidecarImage(auto, docker, image)
	if err == nil || !strings.Contains(err.Error(), "127") {
		t.Fatalf("first attempt error = %v, want the build failure", err)
	}
	var paused *go2RTCBuildPausedError
	if errors.As(err, &paused) {
		t.Fatal("the first failure itself must not be a paused error")
	}
	if got := fake.buildCount(); got != 1 {
		t.Fatalf("builds after the first attempt = %d, want 1", got)
	}
	failedLog := func() int { return strings.Count(logs.String(), "Managed sidecar image build failed") }
	if failedLog() != 1 {
		t.Fatalf("failed builds logged %d times, want once:\n%s", failedLog(), logs.String())
	}
	for _, want := range []string{"level=WARN", "non-zero code: 127", "retry_in=1m0s", "next_retry_at=2026-10-28T12:01:00Z"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("failure log lacks %q:\n%s", want, logs.String())
		}
	}

	// Ticks inside the wait do not touch Docker's build endpoint and carry the
	// original failure plus the retry time; they log nothing at WARN.
	for _, elapsed := range []time.Duration{30 * time.Second, 29 * time.Second} {
		clock.Advance(elapsed)
		err = m.ensureManagedSidecarImage(auto, docker, image)
		if !errors.As(err, &paused) || !strings.Contains(err.Error(), "127") || !strings.Contains(err.Error(), "2026-10-28T12:01:00Z") {
			t.Fatalf("tick inside the wait returned %v, want a paused error with the failure and retry time", err)
		}
	}
	if got := fake.buildCount(); got != 1 {
		t.Fatalf("builds during the wait = %d, want still 1", got)
	}
	if failedLog() != 1 {
		t.Fatalf("a skipped tick logged a failed build:\n%s", logs.String())
	}

	// Wait over: one more attempt, then twice as long.
	clock.Advance(time.Second)
	if err = m.ensureManagedSidecarImage(auto, docker, image); err == nil || errors.As(err, &paused) {
		t.Fatalf("attempt after the wait = %v, want a fresh build failure", err)
	}
	if got := fake.buildCount(); got != 2 {
		t.Fatalf("builds after the wait = %d, want 2", got)
	}
	if !strings.Contains(logs.String(), "retry_in=2m0s") {
		t.Errorf("second failure did not double the wait:\n%s", logs.String())
	}
	clock.Advance(119 * time.Second)
	if err = m.ensureManagedSidecarImage(auto, docker, image); !errors.As(err, &paused) {
		t.Fatalf("attempt 119s into the doubled wait = %v, want paused", err)
	}
	clock.Advance(time.Second)
	_ = m.ensureManagedSidecarImage(auto, docker, image)
	if got := fake.buildCount(); got != 3 {
		t.Fatalf("builds after the doubled wait = %d, want 3", got)
	}

	// A new image tag (new config or Dockerfile) is not held back by the wait of
	// the old one.
	hardware := go2RTCManagedImage(config.Go2RTCHardwareImage)
	if hardware == image {
		t.Fatal("test needs two different managed image tags")
	}
	_ = m.ensureManagedSidecarImage(auto, docker, hardware)
	if got := fake.buildCount(); got != 4 {
		t.Fatalf("builds after a tag change = %d, want 4 (not held back)", got)
	}

	// An explicit start builds at once, in the middle of a wait, and starts the
	// backoff over with the short wait.
	if err = m.ensureManagedSidecarImage(auto, docker, hardware); !errors.As(err, &paused) {
		t.Fatalf("automatic attempt right after a failure = %v, want paused", err)
	}
	if err = m.ensureManagedSidecarImage(context.Background(), docker, hardware); err == nil || errors.As(err, &paused) {
		t.Fatalf("explicit attempt = %v, want a fresh build failure", err)
	}
	if got := fake.buildCount(); got != 5 {
		t.Fatalf("builds after an explicit attempt = %d, want 5", got)
	}
	if m.buildBackoff.failures != 1 {
		t.Fatalf("failures after an explicit attempt = %d, want the backoff started over at 1", m.buildBackoff.failures)
	}
	clock.Advance(time.Minute)
	_ = m.ensureManagedSidecarImage(auto, docker, hardware)
	if got := fake.buildCount(); got != 6 {
		t.Fatalf("builds one minute after the explicit attempt = %d, want 6", got)
	}

	// A build that works ends the backoff.
	fake.set(false, false)
	clock.Advance(time.Hour)
	if err = m.ensureManagedSidecarImage(auto, docker, hardware); err != nil {
		t.Fatalf("working build returned %v", err)
	}
	if m.buildBackoff.failures != 0 {
		t.Fatalf("failures after a working build = %d, want 0", m.buildBackoff.failures)
	}
	for _, tag := range fake.tagList() {
		if tag != image && tag != hardware {
			t.Errorf("build tagged %q", tag)
		}
	}
}

// The reconcile tick is what looped: it reached the image build every 30
// seconds. It must wait out the backoff, while an explicit start builds at once.
func TestGo2RTCTickDoesNotRebuildAFailingImageEveryTime(t *testing.T) {
	h := newGo2RTCLifecycleHarness(t, true, 0)
	fake := &go2RTCBuildFake{failBuilds: true}
	cfg := *h.cfg
	cfg.Docker.Host = fakeDockerHost(t, fake.handler(t))
	h.manager.Configure(&cfg)
	logs := &go2RTCLogBuffer{}
	h.manager.logger = logs.logger()
	clock := newGo2RTCTestClock()
	h.manager.buildBackoff.now = clock.Now
	ctx := context.Background()

	lastError := func() string {
		h.manager.mu.RLock()
		defer h.manager.mu.RUnlock()
		return h.manager.status.LastError
	}

	h.manager.reconcileTick(ctx)
	if got := fake.buildCount(); got != 1 {
		t.Fatalf("builds after the first tick = %d, want 1", got)
	}
	if !strings.Contains(lastError(), "127") {
		t.Fatalf("status error after the first tick = %q, want the build failure", lastError())
	}
	for _, elapsed := range []time.Duration{30 * time.Second, 29 * time.Second} { // 30s and 59s into the 60s wait
		clock.Advance(elapsed)
		h.manager.reconcileTick(ctx)
	}
	if got := fake.buildCount(); got != 1 {
		t.Fatalf("builds during the wait = %d, want 1 (a tick rebuilt)", got)
	}
	if !strings.Contains(lastError(), "paused") || !strings.Contains(lastError(), "127") {
		t.Fatalf("status error during the wait = %q, want the pause and the failure", lastError())
	}

	// The wait is over at 60s: the next tick tries again, once.
	clock.Advance(time.Second)
	h.manager.reconcileTick(ctx)
	if got := fake.buildCount(); got != 2 {
		t.Fatalf("builds after the wait = %d, want 2", got)
	}

	// An explicit start (the /api/go2rtc/start path) does not wait, nor does a
	// restart.
	if err := h.manager.StartContainer(ctx); err == nil {
		t.Fatal("start with a failing build returned no error")
	}
	if got := fake.buildCount(); got != 3 {
		t.Fatalf("builds after an explicit start = %d, want 3", got)
	}
	if err := h.manager.RestartContainer(ctx); err == nil {
		t.Fatal("restart with a failing build returned no error")
	}
	if got := fake.buildCount(); got != 4 {
		t.Fatalf("builds after an explicit restart = %d, want 4", got)
	}

	// ... and the tick right after it is held back again.
	h.manager.reconcileTick(ctx)
	if got := fake.buildCount(); got != 4 {
		t.Fatalf("builds on the tick after an explicit restart = %d, want 4", got)
	}
}
