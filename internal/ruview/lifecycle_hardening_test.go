package ruview

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
)

// A Stop that lands in the unlocked part of a watch tick (the storage walk, the
// Engine inspect) must not hand the loop the generation Stop just created: a
// probe under it would report a deliberately stopped service as unreachable.
func TestRuViewWatchSchedulesNoProbeAfterStopInItsUnlockedPart(t *testing.T) {
	m, p := nativeFixture(t)
	p.kill = func() error { close(p.done); return nil }
	var once sync.Once
	m.walkStorage = func(context.Context, string) (int64, error) {
		once.Do(func() {
			if err := m.Stop(context.Background()); err != nil {
				t.Error(err)
			}
		})
		return 0, nil
	}
	if m.watch(context.Background()) != nil {
		t.Fatal("a tick that was overtaken by a Stop must not schedule a probe")
	}
	if s := m.Status(); s.Runtime != RuntimeStopped || s.StopReason != StopUser {
		t.Fatalf("the stop was not recorded: %+v", s)
	}
	if m.watch(context.Background()) != nil {
		t.Fatal("a stopped service has nothing to probe")
	}
}

// Stop followed by Start with an identical configuration, while the loop is
// still waiting for the Engine's answer about the old container, must not mark
// the healthy new run as exited. The configuration pointer is the same on both
// sides, so only the run token tells them apart.
func TestRuViewWatchDoesNotMarkANewRunAsExited(t *testing.T) {
	m, _, f := managedBackgroundFixture(t, nil)
	m.dockerEvery = 0
	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	f.exited.Store(true) // the old container has ended: this is what the inspect will report
	restart := func() {
		f.exited.Store(false) // the replacement is healthy
		if err := m.Stop(ctx); err != nil {
			t.Error(err)
		}
		if err := m.Start(ctx); err != nil {
			t.Error(err)
		}
	}
	f.onInspect.Store(&restart)
	if m.watch(ctx) != nil {
		t.Fatal("watch must not probe a run it did not inspect")
	}
	s := m.Status()
	if s.ErrorCode != "" || s.Runtime == RuntimeExited || !s.RevisionVerified || s.StopReason != "" {
		t.Fatalf("the healthy new run was marked as ended: %+v", s)
	}
	// The next tick looks at the new run and finds it alive.
	if m.watch(ctx) == nil {
		t.Fatal("the new run was not scheduled for probing")
	}
}

// A Docker host or data directory change is runtime-relevant (the same key as
// sameRuntime), so it cancels a start in its pull. The cancelled start leaves
// neither a "starting" status nor an error behind.
func TestRuViewConfigureDuringPullCancelsTheStartCleanly(t *testing.T) {
	m, c, f := managedBackgroundFixture(t, nil)
	f.imageMissing.Store(true)
	f.pullEntered, f.pullRelease = make(chan struct{}), make(chan struct{}) // never released
	started := make(chan error, 1)
	go func() { started <- m.Start(context.Background()) }()
	select {
	case <-f.pullEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("the start never reached the image pull")
	}
	next := *c
	next.RuView = c.RuView
	next.Docker.Host = "tcp://127.0.0.1:1"
	m.config = func() *config.Config { return &next }
	m.Configure()
	select {
	case err := <-started:
		if e := AsError(err); e == nil || e.Code != CodeCancelled {
			t.Fatalf("the start must end cancelled: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("a Docker host change did not cancel the pull")
	}
	s := m.Status()
	if s.DataState == "starting" || s.Runtime != RuntimeStopped || s.StopReason != StopConfigChanged || s.ErrorCode != "" || s.Phase != "" || f.creates.Load() != 0 {
		t.Fatalf("a start cancelled by Configure must leave a settled status: %+v creates=%d", s, f.creates.Load())
	}
}

// The caller (a closed tab, a cancelled agent run) giving up is not a start
// failure: nothing is recorded, and the next start is not blocked.
func TestRuViewCallerGivingUpIsNotAStartFailure(t *testing.T) {
	m, _, f := managedBackgroundFixture(t, nil)
	f.imageMissing.Store(true)
	f.pullEntered, f.pullRelease = make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan error, 1)
	go func() { started <- m.Start(ctx) }()
	select {
	case <-f.pullEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("the start never reached the image pull")
	}
	cancel()
	select {
	case err := <-started:
		if e := AsError(err); e == nil || e.Code != CodeCancelled {
			t.Fatalf("the abandoned start must end cancelled: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the start survived its caller")
	}
	if s := m.Status(); s.ErrorCode != "" || s.Error != "" || s.DataState == "starting" || s.Runtime != RuntimeNotStarted || s.Phase != "" {
		t.Fatalf("a caller cancellation was recorded as a failure: %+v", s)
	}
	close(f.pullRelease)
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("the next start must not be blocked: %v", err)
	}
}

// Start and install failures stay in the status while nothing runs: a later
// probe (failed or not) and a Configure of an unrelated setting keep them, and
// only an explicit Stop or Start or a runtime-relevant Configure replaces them.
func TestRuViewStartFailuresAreSticky(t *testing.T) {
	codes := []string{CodeImageDenied, CodeImagePullFailed, CodeDockerUnavailable, CodeStartFailed, CodeArtifactMissing,
		CodeDownloadFailed, CodeContainerConflict, CodePlatformUnsupported, CodeTimeout}
	for _, code := range codes {
		t.Run(code, func(t *testing.T) {
			m, c, f := managedBackgroundFixture(t, nil)
			ctx := context.Background()
			record := func() {
				m.mu.Lock()
				m.recordFailureLocked(fail(code, "start failed", errors.New("cause")))
				m.mu.Unlock()
			}
			record()
			want := m.Status()
			if want.ErrorCode != code || want.ErrorDetail != "cause" || want.Runtime != RuntimeNotStarted {
				t.Fatalf("recorded failure: %+v", want)
			}
			// A probe that succeeds stores its data but keeps the failure.
			if s, err := m.Test(ctx); err != nil || !s.Ready || s.ErrorCode != code || s.ErrorDetail != "cause" || s.Error != want.Error {
				t.Fatalf("a ready probe replaced the failure: %+v %v", s, err)
			}
			// A probe that fails must not put "unreachable" or "upstream_error" in front of the cause.
			f.failStatus.Store(true)
			if s, err := m.Test(ctx); err == nil || s.ErrorCode != code {
				t.Fatalf("a failing probe replaced the failure: %+v %v", s, err)
			}
			// An unrelated setting keeps it...
			c.RuView.AgentAccess = !c.RuView.AgentAccess
			m.Configure()
			if s := m.Status(); s.ErrorCode != code {
				t.Fatalf("an agent-access change dropped the failure: %+v", s)
			}
			// ...a runtime-relevant one replaces it.
			c.RuView.Source = "wifi"
			m.Configure()
			if s := m.Status(); s.ErrorCode != "" {
				t.Fatalf("a runtime change kept the failure: %+v", s)
			}
			c.RuView.Source = "esp32"
			m.Configure()
			f.failStatus.Store(false)
			// An explicit Stop replaces it.
			record()
			if err := m.Stop(ctx); err != nil {
				t.Fatal(err)
			}
			if s := m.Status(); s.ErrorCode != "" || s.Runtime != RuntimeStopped {
				t.Fatalf("Stop kept the failure: %+v", s)
			}
			// An explicit Start replaces it.
			record()
			if err := m.Start(ctx); err != nil {
				t.Fatal(err)
			}
			if s := m.Status(); s.ErrorCode != "" || s.Error != "" {
				t.Fatalf("Start kept the failure: %+v", s)
			}
		})
	}

	t.Run("a real failed start", func(t *testing.T) {
		m, _, f := managedBackgroundFixture(t, nil)
		f.failStart.Store(true)
		if err := m.Start(context.Background()); err == nil {
			t.Fatal("start succeeded")
		}
		f.failStatus.Store(true)
		if s, err := m.Test(context.Background()); err == nil || s.ErrorCode != CodeStartFailed {
			t.Fatalf("the start failure did not survive a probe: %+v %v", s, err)
		}
	})
}

// The text of a failure is redacted like its detail, and a start that fails
// after the container exists removes it with force (it may have half started).
func TestRuViewStartFailureTextCarriesNoPathOrURL(t *testing.T) {
	m, _, f := managedBackgroundFixture(t, nil)
	f.failStart.Store(true)
	err := m.Start(context.Background())
	if err == nil {
		t.Fatal("start succeeded")
	}
	s := m.Status()
	for name, text := range map[string]string{"error": err.Error(), "status.error": s.Error, "status.detail": s.ErrorDetail} {
		if strings.Contains(text, "/var/lib") || strings.Contains(text, "registry.example") || strings.Contains(text, "10.0.0.5") || !strings.Contains(text, "rejected") {
			t.Fatalf("%s leaks the Engine's text or lost the reason: %q", name, text)
		}
	}
	if f.forced.Load() != 1 {
		t.Fatalf("the cleanup after a failed container start must delete with force: forced=%d deletes=%d", f.forced.Load(), f.deletes.Load())
	}
}

// Every entry point gives up instead of hanging behind a stuck opMu holder.
func TestRuViewLifecycleEntryPointsGiveUpWhenOpMuIsHeld(t *testing.T) {
	m, _, _ := managedBackgroundFixture(t, nil)
	m.lockWait = 30 * time.Millisecond
	if err := m.opMu.acquire(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	var err error
	within(t, 2*time.Second, "Start", func() { err = m.Start(context.Background()) })
	if e := AsError(err); e == nil || e.Code != CodeBusy {
		t.Fatalf("Start behind a held lock must be busy: %v", err)
	}
	within(t, 2*time.Second, "Install", func() { err = m.Install(context.Background()) })
	if e := AsError(err); e == nil || e.Code != CodeBusy {
		t.Fatalf("Install behind a held lock must be busy: %v", err)
	}
	within(t, 2*time.Second, "watch", func() {
		if m.watch(context.Background()) != nil {
			t.Error("watch behind a held lock must schedule nothing")
		}
	})
	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	within(t, 2*time.Second, "Stop", func() { err = m.Stop(stopCtx) })
	if e := AsError(err); e == nil || e.Code != CodeTimeout {
		t.Fatalf("Stop behind a held lock must end with its context: %v", err)
	}
	cancelled, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	within(t, 2*time.Second, "Start with a cancelled context", func() { err = m.Start(cancelled) })
	if e := AsError(err); e == nil || e.Code != CodeCancelled {
		t.Fatalf("a cancelled caller must not wait: %v", err)
	}
	m.opMu.release()
	if err := m.Stop(context.Background()); err != nil {
		t.Fatalf("the lock was not usable after the holder left: %v", err)
	}
}

// A process that ignores its kill ends the stop as unconfirmed; it does not hold
// opMu forever when the caller has no deadline.
func TestRuViewStopWaitForAKilledProcessIsBounded(t *testing.T) {
	m, p := nativeFixture(t)
	p.kill = func() error { return nil } // the kill is accepted, the process lingers
	m.stopWait = 30 * time.Millisecond
	var err error
	within(t, 2*time.Second, "Stop", func() { err = m.Stop(context.Background()) })
	if e := AsError(err); e == nil || e.Code != CodeStopFailed {
		t.Fatalf("a lingering process must be stop_failed: %v", err)
	}
	if s := m.Status(); s.Runtime != RuntimeStopUnconfirmed {
		t.Fatalf("an unconfirmed stop must say so: %+v", s)
	}
	close(p.done)
	if err := m.Stop(context.Background()); err != nil {
		t.Fatalf("the process ended, so the next stop must confirm: %v", err)
	}
}

// A hint's state change and its delivery are one step: a slow "open" callback
// must not be overtaken by the "close" that follows it.
func TestRuViewHintDeliveriesKeepTheOrderOfStateChanges(t *testing.T) {
	m, _ := fixture(t, func(http.ResponseWriter, *http.Request) {})
	var mu sync.Mutex
	var delivered []hintCall
	entered, release := make(chan struct{}), make(chan struct{})
	m.SetOperationalHint(func(event string, open bool) {
		mu.Lock()
		delivered = append(delivered, hintCall{event, open})
		mu.Unlock()
		if open {
			close(entered)
			<-release
		}
	})
	go m.emitHint(hintSensingStale, true)
	<-entered
	done := make(chan struct{})
	go func() { m.emitHint(hintSensingStale, false); close(done) }()
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	early := len(delivered)
	mu.Unlock()
	if early != 1 {
		t.Fatalf("the close was delivered while the open was still in flight: %d deliveries", early)
	}
	close(release)
	<-done
	mu.Lock()
	defer mu.Unlock()
	if len(delivered) != 2 || delivered[0].open != true || delivered[1].open != false {
		t.Fatalf("deliveries out of order: %+v", delivered)
	}
}

// The saved device list lives in a file whose path the OS puts into its errors;
// the agent tool forwards err.Error() to the model, so the error is typed with a
// fixed sentence and a redacted detail.
func TestRuViewDeviceListErrorsCarryNoPath(t *testing.T) {
	m, c := fixture(t, func(http.ResponseWriter, *http.Request) {})
	c.RuView.ReadOnly = false
	c.Directories.DataDir = t.TempDir()
	// A file where the "ruview" directory belongs makes both reading and creating fail.
	if err := os.WriteFile(filepath.Join(c.Directories.DataDir, "ruview"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := m.RememberNode(context.Background(), 5)
	if err == nil {
		t.Fatal("a blocked device directory was accepted")
	}
	e := AsError(err)
	for name, text := range map[string]string{"error": err.Error(), "detail": e.Detail} {
		if strings.Contains(text, c.Directories.DataDir) || strings.Contains(text, "nodes-") {
			t.Fatalf("%s carries the file path: %q", name, text)
		}
	}
	if e.Code != CodeUnknown || e.HTTP != 500 {
		t.Fatalf("a device list failure is unknown/500: %+v", e)
	}
}
