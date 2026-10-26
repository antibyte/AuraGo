package ruview

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

type backgroundFixture struct {
	starts, creates, probes      atomic.Int32
	owned, failStatus, failStart atomic.Bool
	startEntered, startCancelled chan struct{}
	// imageMissing makes the image inspect answer 404 until a pull completes;
	// exited and foreign change what the container inspect reports.
	imageMissing, exited, foreign atomic.Bool
	pulls, deletes, stops, forced atomic.Int32
	// onInspect runs once, inside the next container inspect, before it answers
	// with the state it saw when the request arrived: a Stop or Start that
	// happens while the caller is still waiting for the Engine.
	onInspect atomic.Pointer[func()]
	// pullEntered is closed when the first pull arrives; the pull then waits
	// for pullRelease (or for its request to be cancelled).
	pullEntered, pullRelease chan struct{}
	pullOnce                 sync.Once
}

// within fails the test when fn does not return in time, instead of hanging.
func within(t *testing.T, d time.Duration, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); fn() }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s did not return within %v", what, d)
	}
}

func managedBackgroundFixture(t *testing.T, statusHandler http.HandlerFunc) (*Manager, *config.Config, *backgroundFixture) {
	t.Helper()
	f := &backgroundFixture{}
	c := &config.Config{RuView: config.DefaultRuViewConfig()}
	c.RuView.Enabled, c.RuView.ReadOnly, c.Docker.Enabled = true, false, true
	c.Directories.DataDir = t.TempDir()
	versionPrefix := regexp.MustCompile(`^/v[0-9.]+/`)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := versionPrefix.ReplaceAllString(r.URL.Path, "/")
		switch {
		case path == "/version":
			io.WriteString(w, `{"ApiVersion":"1.51","MinAPIVersion":"1.24"}`)
		case path == "/containers/aurago-ruview/json":
			if !f.owned.Load() {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			label, status, exit, exitedNow := owner(c), "running", 0, f.exited.Load()
			if hook := f.onInspect.Swap(nil); hook != nil {
				(*hook)()
			}
			if f.foreign.Load() {
				label = "someone-else"
			}
			if exitedNow {
				status, exit = "exited", 137
			}
			json.NewEncoder(w).Encode(map[string]any{"Id": "boot-fixture", "Config": map[string]any{"Labels": map[string]string{"aurago.managed": "ruview", "aurago.ruview.owner": label}}, "State": map[string]any{"Running": !exitedNow, "Status": status, "ExitCode": exit}})
		case path == "/images/create":
			f.pulls.Add(1)
			if f.pullEntered != nil {
				f.pullOnce.Do(func() { close(f.pullEntered) })
			}
			if f.pullRelease != nil {
				select {
				case <-f.pullRelease:
				case <-r.Context().Done():
					return
				}
			}
			f.imageMissing.Store(false)
			io.WriteString(w, `{"status":"Downloaded"}`+"\n")
		case strings.HasPrefix(path, "/images/"):
			if f.imageMissing.Load() {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			io.WriteString(w, `{}`)
		case path == "/containers/create":
			f.creates.Add(1)
			f.owned.Store(true)
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, `{"Id":"boot-fixture"}`)
		case path == "/containers/boot-fixture/start":
			f.starts.Add(1)
			if f.startEntered != nil {
				close(f.startEntered)
				<-r.Context().Done()
				close(f.startCancelled)
				return
			}
			if f.failStart.Load() {
				http.Error(w, "fixture start rejected at /var/lib/docker/overlay2/x via http://registry.example/v2/ from 10.0.0.5:2375", http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case path == "/containers/boot-fixture/stop":
			f.stops.Add(1)
			w.WriteHeader(http.StatusNoContent)
		case path == "/containers/boot-fixture" && r.Method == http.MethodDelete:
			f.deletes.Add(1)
			if r.URL.Query().Get("force") == "1" {
				f.forced.Add(1)
			}
			f.owned.Store(false)
			w.WriteHeader(http.StatusNoContent)
		case path == "/api/v1/status":
			f.probes.Add(1)
			if statusHandler != nil {
				statusHandler(w, r)
				return
			}
			if f.failStatus.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			io.WriteString(w, `{"status":"ready","source":"esp32","source_state":"live_unverified","last_frame_age_ms":0}`)
		case path == "/api/v1/nodes":
			io.WriteString(w, `{"nodes":[{"node_id":5,"csi_status":"active","csi_last_seen_ms":0,"csi_sequence":1}]}`)
		default:
			t.Errorf("unexpected background request: %s %s", r.Method, path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(api.Close)
	u, _ := url.Parse(api.URL)
	c.RuView.HTTPPort, _ = strconv.Atoi(u.Port())
	c.Docker.Host = "tcp://" + u.Host
	c.RuView.URL, c.RuView.AllowPrivate = api.URL, true
	vault, err := security.NewVault(strings.Repeat("01", 32), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	m := New(func() *config.Config { return c }, vault, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(m.Close)
	return m, c, f
}

func awaitBackground(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("background state did not converge")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestBackgroundStartsEnabledManagedRuntimeAndRefreshesStatus(t *testing.T) {
	m, c, f := managedBackgroundFixture(t, nil)
	m.StartBackground(context.Background())
	m.StartBackground(context.Background())
	awaitBackground(t, func() bool { return m.Status().Ready })
	if f.starts.Load() != 1 || f.creates.Load() != 1 || !m.Status().Fresh {
		t.Fatal("startup did not create exactly one ready managed service")
	}
	name, err := nodeFile(c)
	if err != nil {
		t.Fatal(err)
	}
	awaitBackground(t, func() bool { data, _ := os.ReadFile(name); return string(data) == "[5]" })
	f.failStatus.Store(true)
	awaitBackground(t, func() bool { return !m.Status().Ready })
	if m.Status().Error == "" {
		t.Fatal("failed health check hid its error")
	}
	f.failStatus.Store(false)
	awaitBackground(t, func() bool { return m.Status().Ready })
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	probes := f.probes.Load()
	time.Sleep(3200 * time.Millisecond)
	if f.starts.Load() != 1 || f.probes.Load() != probes || m.Status().DataState != "stopped" {
		t.Fatal("background work undid an administrator stop")
	}
}

func TestBackgroundHonorsStartupGates(t *testing.T) {
	for _, gate := range []string{"disabled", "readonly", "external", "docker-readonly", "native-not-installed"} {
		t.Run(gate, func(t *testing.T) {
			m, c, f := managedBackgroundFixture(t, nil)
			switch gate {
			case "disabled":
				c.RuView.Enabled = false
			case "readonly":
				c.RuView.ReadOnly = true
			case "external":
				c.RuView.Mode = "external"
			case "docker-readonly":
				c.Docker.ReadOnly = true
			case "native-not-installed":
				c.RuView.Mode = "native"
			}
			m.Configure()
			m.StartBackground(context.Background())
			if gate == "docker-readonly" || gate == "native-not-installed" {
				awaitBackground(t, func() bool { return m.Status().Error != "" })
			} else {
				time.Sleep(1200 * time.Millisecond)
			}
			if f.creates.Load() != 0 || f.probes.Load() != 0 || m.Status().Ready {
				t.Fatal("startup bypassed a gate or probed an external/unmanaged service")
			}
		})
	}
}

func TestBackgroundStartFailureIsVisibleAndNotRetried(t *testing.T) {
	m, _, f := managedBackgroundFixture(t, nil)
	f.failStart.Store(true)
	m.StartBackground(context.Background())
	awaitBackground(t, func() bool { return m.Status().Error != "" })
	time.Sleep(3200 * time.Millisecond)
	if f.starts.Load() != 1 || m.Status().Ready || m.Status().DataState != "disconnected" {
		t.Fatal("failed startup was retried or reported ready")
	}
}

func TestBackgroundProbeDoesNotBlockStopOrReviveStoppedService(t *testing.T) {
	entered, cancelled := make(chan struct{}), make(chan struct{})
	m, _, f := managedBackgroundFixture(t, func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(cancelled)
	})
	m.StartBackground(context.Background())
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("background probe not started")
	}
	if m.Status().DataState != "starting" || m.Status().Ready {
		t.Fatal("startup claimed readiness without an acknowledged health response")
	}
	time.Sleep(3200 * time.Millisecond)
	if f.probes.Load() != 1 {
		t.Fatal("background probes overlap")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := m.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-cancelled:
	case <-ctx.Done():
		t.Fatal("stop did not cancel the background probe")
	}
	if s := m.Status(); s.Ready || s.DataState != "stopped" {
		t.Fatal("late background probe revived the stopped service")
	}
}

func TestStartupAndProbeFromOldGenerationCannotUndoStop(t *testing.T) {
	m, _, f := managedBackgroundFixture(t, nil)
	generation := m.generation
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.start(context.Background(), generation); err == nil {
		t.Fatal("old startup succeeded after a stop")
	}
	if _, err := m.test(context.Background(), generation); err == nil {
		t.Fatal("old probe succeeded after a stop")
	}
	if f.starts.Load() != 0 || f.probes.Load() != 0 || m.Status().DataState != "stopped" {
		t.Fatal("stale background work changed the stopped state")
	}
}

func TestBackgroundReapsExitDuringPendingHealthRead(t *testing.T) {
	entered, cancelled := make(chan struct{}), make(chan struct{})
	m, _, f := managedBackgroundFixture(t, func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(cancelled)
	})
	m.StartBackground(context.Background())
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("background health read not started")
	}
	p := &nativeProcess{done: make(chan struct{})}
	close(p.done)
	m.mu.Lock()
	m.process = p
	m.mu.Unlock()
	select {
	case <-cancelled:
	case <-time.After(2500 * time.Millisecond):
		t.Fatal("health read blocked the process watcher")
	}
	if f.starts.Load() != 1 || m.Status().Error != "process_exited" || m.Status().Ready {
		t.Fatal("exit was hidden or automatically restarted")
	}
}

func TestBackgroundNativeExitAndStopFailureRemainVisible(t *testing.T) {
	for _, stopFailed := range []bool{false, true} {
		t.Run(strconv.FormatBool(stopFailed), func(t *testing.T) {
			m, p := nativeFixture(t)
			if stopFailed {
				m.status = Status{Error: "stop_failed", DataState: "disconnected"}
			} else {
				close(p.done)
			}
			for i := 0; i < 2; i++ {
				if m.watch(context.Background()) != nil {
					t.Fatal("background scheduled a probe for an exited or uncertain runtime")
				}
			}
			if s := m.Status(); s.Ready || s.Error == "" {
				t.Fatal("background hid the runtime failure")
			}
		})
	}
}

func TestStopAndShutdownCancelAutomaticStart(t *testing.T) {
	for _, shutdown := range []bool{false, true} {
		t.Run(strconv.FormatBool(shutdown), func(t *testing.T) {
			m, _, f := managedBackgroundFixture(t, nil)
			f.startEntered, f.startCancelled = make(chan struct{}), make(chan struct{})
			m.StartBackground(context.Background())
			select {
			case <-f.startEntered:
			case <-time.After(5 * time.Second):
				t.Fatal("automatic start did not reach Docker")
			}
			done := make(chan error, 1)
			go func() {
				if shutdown {
					m.Close()
					done <- nil
				} else {
					done <- m.Stop(context.Background())
				}
			}()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("stop waited for the automatic-start deadline")
			}
			select {
			case <-f.startCancelled:
			case <-time.After(time.Second):
				t.Fatal("automatic start request survived cancellation")
			}
			if f.starts.Load() != 1 || m.Status().DataState != "stopped" {
				t.Fatal("automatic start survived stop/shutdown")
			}
		})
	}
}

func TestRuViewStartPhasesBusyAndStopDuringPull(t *testing.T) {
	t.Run("stop during pull", func(t *testing.T) {
		m, _, f := managedBackgroundFixture(t, nil)
		f.imageMissing.Store(true)
		f.pullEntered, f.pullRelease = make(chan struct{}), make(chan struct{}) // never released
		started := make(chan error, 1)
		go func() { started <- m.Start(context.Background()) }()
		select {
		case <-f.pullEntered:
		case <-time.After(5 * time.Second):
			t.Fatal("the start never reached the image pull")
		}
		s := m.Status()
		if s.Runtime != RuntimeStarting || s.Phase != PhasePullingImage || s.PhaseSince == nil {
			t.Fatalf("a pulling start must report starting/pulling_image with a start time: %+v", s)
		}
		// A pull does not hold opMu: the watch loop, a second start, an install and a stop all return.
		within(t, time.Second, "watch", func() {
			if m.watch(context.Background()) != nil {
				t.Error("watch scheduled a probe for a runtime that is still starting")
			}
		})
		var err error
		within(t, time.Second, "second Start", func() { err = m.Start(context.Background()) })
		if e := AsError(err); e == nil || e.Code != CodeBusy || e.HTTP != http.StatusConflict {
			t.Fatalf("a second start must be busy/409: %v", err)
		}
		within(t, time.Second, "Install", func() { err = m.Install(context.Background()) })
		if e := AsError(err); e == nil || e.Code != CodeBusy {
			t.Fatalf("an install during a start must be busy: %v", err)
		}
		within(t, 2*time.Second, "Stop", func() { err = m.Stop(context.Background()) })
		if err != nil {
			t.Fatalf("stop during a pull: %v", err)
		}
		select {
		case err = <-started:
			if e := AsError(err); e == nil || e.Code != CodeCancelled {
				t.Fatalf("the cancelled start must report cancelled: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("the pending start survived Stop")
		}
		s = m.Status()
		if s.Runtime != RuntimeStopped || s.StopReason != StopUser || s.Phase != "" || s.PhaseSince != nil || f.creates.Load() != 0 || s.ErrorCode != "" {
			t.Fatalf("a stop during a pull must end stopped/user without creating anything: %+v creates=%d", s, f.creates.Load())
		}
	})

	t.Run("pull completes", func(t *testing.T) {
		m, _, f := managedBackgroundFixture(t, nil)
		f.imageMissing.Store(true)
		f.pullRelease = make(chan struct{})
		close(f.pullRelease)
		if err := m.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		if s := m.Status(); s.Runtime != RuntimeStarting || s.Phase != PhaseWaitingReady || f.pulls.Load() != 1 || f.creates.Load() != 1 {
			t.Fatalf("a finished start waits for the first ready probe: %+v", s)
		}
		if s, err := m.Test(context.Background()); err != nil || !s.Ready {
			t.Fatalf("probe: %+v %v", s, err)
		}
		if s := m.Status(); s.Runtime != RuntimeRunning || s.Phase != "" || s.PhaseSince != nil {
			t.Fatalf("the first ready probe must end the wait: %+v", s)
		}
	})
}

func TestRuViewDockerConflictIsReportedAndStopIgnoresForeign(t *testing.T) {
	m, _, f := managedBackgroundFixture(t, nil)
	f.owned.Store(true)
	f.foreign.Store(true)
	err := m.Start(context.Background())
	e := AsError(err)
	if e == nil || e.Code != CodeContainerConflict || HTTPStatus(e.Code) != http.StatusConflict {
		t.Fatalf("a foreign container must be reported as container_conflict: %v", err)
	}
	s := m.Status()
	if s.ErrorCode != CodeContainerConflict || s.Runtime != RuntimeNotStarted {
		t.Fatalf("conflict status: %+v", s)
	}
	// The Engine address and the container URL are internal; none of them may
	// reach the status or the error text, whatever the wrapping.
	blob, _ := json.Marshal(s)
	for _, text := range []string{string(blob), err.Error()} {
		if strings.Contains(text, "127.0.0.1") || strings.Contains(text, "tcp://") || strings.Contains(text, "/containers/") {
			t.Fatalf("the conflict text exposes the Engine: %s", text)
		}
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.stops.Load() != 0 || f.deletes.Load() != 0 {
		t.Fatalf("stop touched a container that is not ours: stops=%d deletes=%d", f.stops.Load(), f.deletes.Load())
	}
}

func TestRuViewDockerExitIsDetected(t *testing.T) {
	m, c, f := managedBackgroundFixture(t, nil)
	c.RuView.UDPEnabled, c.RuView.UDPBind, c.RuView.UDPAllow = true, "192.168.1.2", []string{"192.168.1.4"}
	m.Configure() // record the new baseline: the settings are part of the runtime
	m.dockerEvery = 0
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !m.Status().UDPReady {
		t.Fatal("precondition: a running managed runtime with UDP settings is ready")
	}
	f.exited.Store(true)
	if m.watch(context.Background()) != nil {
		t.Fatal("watch scheduled a probe for an exited container")
	}
	s := m.Status()
	if s.Runtime != RuntimeExited || s.ErrorCode != CodeProcessExited || !strings.Contains(s.ErrorDetail, "137") || s.StopReason != StopProcessExit || s.StoppedAt == nil {
		t.Fatalf("exit not reported: %+v", s)
	}
	if s.UDPReady || s.RevisionVerified || s.Ready {
		t.Fatalf("an exited runtime must not look ready: %+v", s)
	}
	if list, err := m.Nodes(context.Background()); err != nil || list.LiveState != liveStateNotRunning {
		t.Fatalf("an exited runtime must not be dialled: %+v %v", list, err)
	}
	f.exited.Store(false)
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("an explicit start after an exit must not report already_running: %v", err)
	}
	if s := m.Status(); s.Runtime != RuntimeStarting || s.ErrorCode != "" || s.StopReason != "" {
		t.Fatalf("a new start must clear the exit: %+v", s)
	}
}
