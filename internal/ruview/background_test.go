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
			json.NewEncoder(w).Encode(map[string]any{"Id": "boot-fixture", "Config": map[string]any{"Labels": map[string]string{"aurago.managed": "ruview", "aurago.ruview.owner": owner(c)}}})
		case strings.HasPrefix(path, "/images/"):
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
				http.Error(w, "fixture start rejected", http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case path == "/containers/boot-fixture/stop":
			w.WriteHeader(http.StatusNoContent)
		case path == "/containers/boot-fixture" && r.Method == http.MethodDelete:
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
