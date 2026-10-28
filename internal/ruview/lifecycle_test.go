package ruview

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
)

func nativeFixture(t *testing.T) (*Manager, *nativeProcess) {
	t.Helper()
	c := &config.Config{RuView: config.DefaultRuViewConfig()}
	c.RuView.Enabled, c.RuView.ReadOnly, c.RuView.Mode = true, false, "native"
	c.Directories.DataDir = t.TempDir()
	m := New(func() *config.Config { return c }, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	p := &nativeProcess{done: make(chan struct{}), kill: func() error { return nil }}
	m.process, m.managed, m.runningConfig = p, true, c
	m.status = Status{Ready: true, Fresh: true, DataState: "live_unverified"}
	return m, p
}

func TestNativeExitReleasesStartAndInvalidatesStatus(t *testing.T) {
	m, p := nativeFixture(t)
	p.err = errors.New("exited")
	close(p.done)
	m.watch(context.Background())
	if s := m.Status(); s.Ready || s.Fresh || s.RevisionVerified || s.Error != "process_exited" {
		t.Fatalf("exit retained live status: %+v", s)
	}
	if m.process != nil || m.runningConfig != nil {
		t.Fatal("exit retained runtime ownership")
	}
	if err := m.Start(context.Background()); err == nil || strings.Contains(err.Error(), "already started") {
		t.Fatalf("restart did not reach normal installation checks: %v", err)
	}
	// A previous completion must not affect a replacement process.
	replacement := &nativeProcess{done: make(chan struct{})}
	m.process, m.managed = replacement, true
	m.mu.Lock()
	m.reapNativeLocked()
	m.mu.Unlock()
	if m.process != replacement || !m.managed {
		t.Fatal("old exit cleared new process")
	}
}

func TestStopRevokesPendingTest(t *testing.T) {
	entered := make(chan struct{})
	m, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		io.WriteString(w, `{"status":"ready","source":"esp32","source_state":"live_unverified","last_frame_age_ms":0}`)
	})
	done := make(chan error, 1)
	go func() { _, err := m.Test(context.Background()); done <- err }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("probe not started")
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("revoked probe succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("probe not revoked")
	}
	if s := m.Status(); s.Ready || s.Fresh || s.DataState != "stopped" {
		t.Fatalf("late probe won: %+v", s)
	}
}

func TestNativeStopFailureAndConcurrentCompletion(t *testing.T) {
	for _, fail := range []bool{true, false} {
		t.Run(map[bool]string{true: "kill-error", false: "timeout"}[fail], func(t *testing.T) {
			m, p := nativeFixture(t)
			if fail {
				p.kill = func() error { return errors.New("kill denied") }
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			if err := m.Stop(ctx); err == nil {
				t.Fatal("uncertain stop succeeded")
			}
			if s := m.Status(); s.Ready || s.Fresh || s.DataState == "stopped" || s.Error != "stop_failed" {
				t.Fatalf("false stop: %+v", s)
			}
			if !m.managed || m.process != p {
				t.Fatal("lost process needed for another stop")
			}
			close(p.done)
			var wg sync.WaitGroup
			for i := 0; i < 3; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := m.Stop(context.Background()); err != nil {
						t.Error(err)
					}
				}()
			}
			wg.Wait()
			if s := m.Status(); s.DataState != "stopped" || s.RevisionVerified {
				t.Fatalf("completion not observed: %+v", s)
			}
		})
	}
}

func TestReadOnlyStopsRemainNarrowAndTimedRecordingIsRejected(t *testing.T) {
	calls := 0
	m, c := fixture(t, func(w http.ResponseWriter, r *http.Request) { calls++; io.WriteString(w, `{"success":true}`) })
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{"POST", "/api/v1/recording/stop", 200}, {"POST", "/api/v1/train/stop", 200},
		{"POST", "/api/v1/recording/start", 403}, {"POST", "/api/v1/train/start", 403},
		{"POST", "/api/v1/train/pretrain", 403}, {"POST", "/api/v1/train/lora", 403},
		{"POST", "/api/v1/calibration/stop", 403}, {"DELETE", "/api/v1/recording/example", 403},
		{"POST", "/api/v1/train/stop/", 404}, {"GET", "/api/v1/train/stop", 404},
	} {
		w := httptest.NewRecorder()
		m.Proxy(w, httptest.NewRequest(tc.method, "/", strings.NewReader(`{}`)), tc.path)
		if w.Code != tc.want {
			t.Errorf("%s %s: %d", tc.method, tc.path, w.Code)
		}
	}
	c.RuView.ReadOnly = false
	for _, body := range []string{`{"duration_secs":60}`, `{"duration_secs":null}`, `{"duration_secs":0}`} {
		w := httptest.NewRecorder()
		m.Proxy(w, httptest.NewRequest("POST", "/", strings.NewReader(body)), "/api/v1/recording/start")
		if w.Code != 400 {
			t.Fatalf("timed recording accepted: %d", w.Code)
		}
	}
	if calls != 2 {
		t.Fatalf("unexpected upstream mutations: %d", calls)
	}
}

func TestRuViewTestKeepsLifecycleError(t *testing.T) {
	for _, code := range []string{"stop_failed", "process_exited"} {
		for _, probeFails := range []bool{false, true} {
			t.Run(code+map[bool]string{false: "/probe-ok", true: "/probe-fails"}[probeFails], func(t *testing.T) {
				m, c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
					if probeFails {
						http.Error(w, "down", http.StatusServiceUnavailable)
						return
					}
					io.WriteString(w, `{"status":"ready","source":"esp32","source_state":"live_unverified","last_frame_age_ms":5,"waiting_for_frames":false}`)
				})
				c.Directories.DataDir = t.TempDir()
				m.mu.Lock()
				m.status = Status{DataState: "disconnected", Error: code}
				if code == "stop_failed" {
					// The runtime is still owned, so the health loop would normally keep probing it.
					m.managed, m.runningConfig = true, c
				}
				m.mu.Unlock()
				for range 2 {
					s, err := m.Test(context.Background())
					if (err != nil) != probeFails || s.Error != code || s.Ready == probeFails {
						t.Fatalf("probe replaced the lifecycle error: %+v %v", s, err)
					}
					if !probeFails && (!s.Fresh || s.DataState != "live_unverified" || s.ObservedAt == nil) {
						t.Fatalf("fresh probe data was not stored: %+v", s)
					}
				}
				if code == "stop_failed" && m.watch(context.Background()) != nil {
					t.Fatal("the stop_failed guard was bypassed after a probe")
				}
				// An explicit stop replaces the status, so the next probe reports plainly.
				if err := m.Stop(context.Background()); err != nil {
					t.Fatal(err)
				}
				if s := m.Status(); s.Error != "" || s.DataState != "stopped" {
					t.Fatalf("explicit stop did not clear the lifecycle error: %+v", s)
				}
				if s, err := m.Test(context.Background()); (err != nil) != probeFails || lifecycleError(s.Error) {
					t.Fatalf("lifecycle error survived an explicit stop: %+v %v", s, err)
				}
			})
		}
	}
}

func TestRuViewConfigureKeepsLifecycleError(t *testing.T) {
	m, _ := nativeFixture(t)
	c := m.config()
	m.mu.Lock()
	m.status = Status{Error: "stop_failed", ErrorCode: CodeStopFailed, ErrorDetail: "kill denied", DataState: "disconnected"}
	m.mu.Unlock()
	m.Configure() // the baseline configuration is recorded
	c.RuView.Source = "wifi"
	m.Configure()
	if s := m.Status(); s.Error != "stop_failed" || s.ErrorCode != CodeStopFailed || s.ErrorDetail != "kill denied" {
		t.Fatalf("configuration change cleared the unconfirmed stop: %+v", s)
	}
	if m.watch(context.Background()) != nil {
		t.Fatal("configuration change re-enabled health probing of an unconfirmed runtime")
	}
	// A change of agent access neither resets the probe data nor revokes anything (M27)...
	m.mu.Lock()
	m.status = Status{Ready: true, DataState: "live_unverified"}
	m.mu.Unlock()
	c.RuView.AgentAccess = !c.RuView.AgentAccess
	m.Configure()
	if s := m.Status(); !s.Ready {
		t.Fatalf("an agent-access change reset the probe data: %+v", s)
	}
	// ...a runtime-relevant one still does.
	c.RuView.Source = "esp32"
	m.Configure()
	if s := m.Status(); s.Ready || s.Error != "" {
		t.Fatalf("configuration change kept probe data: %+v", s)
	}
}

// A change that is reverted before the next tick leaves the runtime running, so
// the stop reason Configure recorded must not outlive it.
func TestRuViewRevertedConfigureLeavesNoStopReason(t *testing.T) {
	m, _ := nativeFixture(t)
	original := m.config()
	changed := *original
	changed.RuView = original.RuView
	changed.RuView.Source = "wifi"
	m.config = func() *config.Config { return &changed }
	m.Configure()
	m.config = func() *config.Config { return original }
	m.Configure()
	if m.watch(context.Background()) == nil {
		t.Fatal("a reverted change stopped the runtime")
	}
	if s := m.Status(); s.Runtime != RuntimeRunning || s.StopReason != "" || s.StoppedAt != nil {
		t.Fatalf("a running runtime must not report a stop reason: %+v", s)
	}
}

func TestRuViewConfigureCancelsOnlyRuntimeChanges(t *testing.T) {
	for _, tc := range []struct {
		name    string
		change  func(*config.Config)
		runtime bool
	}{
		{"agent_access", func(c *config.Config) { c.RuView.AgentAccess = !c.RuView.AgentAccess }, false},
		{"readonly", func(c *config.Config) { c.RuView.ReadOnly = !c.RuView.ReadOnly }, false},
		{"storage_mb", func(c *config.Config) { c.RuView.StorageMB += 512 }, false},
		{"source", func(c *config.Config) { c.RuView.Source = "wifi" }, true},
		{"http_port", func(c *config.Config) { c.RuView.HTTPPort++ }, true},
		// Outside the RuView section, but part of what the runtime is created with.
		{"docker_host", func(c *config.Config) { c.Docker.Host = "tcp://127.0.0.1:1" }, true},
		{"docker_enabled", func(c *config.Config) { c.Docker.Enabled = !c.Docker.Enabled }, true},
		{"docker_readonly", func(c *config.Config) { c.Docker.ReadOnly = !c.Docker.ReadOnly }, true},
		{"data_dir", func(c *config.Config) { c.Directories.DataDir += "-moved" }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, p := nativeFixture(t)
			p.kill = func() error { close(p.done); return nil }
			c := m.config()
			m.Configure() // baseline
			// runningConfig aliases c, so the change must arrive as a new snapshot.
			next := *c
			next.RuView = c.RuView
			tc.change(&next)
			m.config = func() *config.Config { return &next }
			// ctx stands for a pull, a probe or a stream admitted before the change.
			ctx, done := m.bound(context.Background())
			defer done()
			m.Configure()
			if !tc.runtime {
				if ctx.Err() != nil || !m.Status().Ready {
					t.Fatalf("a %s change revoked work in flight or reset the status: %+v", tc.name, m.Status())
				}
				if m.watch(context.Background()) == nil {
					t.Fatalf("a %s change stopped the runtime", tc.name)
				}
				return
			}
			select {
			case <-ctx.Done(): // revoked through context.AfterFunc, which runs asynchronously
			case <-time.After(time.Second):
				t.Fatalf("a %s change left work in flight alive", tc.name)
			}
			if m.Status().Ready {
				t.Fatalf("a %s change kept probe data", tc.name)
			}
			if m.watch(context.Background()) != nil {
				t.Fatalf("a %s change kept the old runtime scheduled", tc.name)
			}
			s := m.Status()
			if s.Runtime != RuntimeStopped || s.StopReason != StopConfigChanged || s.StoppedAt == nil {
				t.Fatalf("runtime change did not end as stopped/config_changed: %+v", s)
			}
		})
	}
}
