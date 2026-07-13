package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/virtualcomputers"
)

func TestVirtualComputersAutoSetupSkipsDisabledConfiguration(t *testing.T) {
	restore := setVirtualComputersAutoSetupTestHooks(t)
	defer restore()
	var runs atomic.Int32
	virtualComputersAutoSetupRunner = func(context.Context, *Server, virtualcomputers.ToolConfig) error {
		runs.Add(1)
		return nil
	}

	if virtualComputersTriggerAutoSetup(&Server{}, virtualcomputers.ToolConfig{AutoSetup: true}) {
		t.Fatal("disabled integration must not start auto setup")
	}
	if virtualComputersTriggerAutoSetup(&Server{}, virtualcomputers.ToolConfig{Enabled: true}) {
		t.Fatal("auto_setup=false must not start auto setup")
	}
	if runs.Load() != 0 {
		t.Fatalf("auto setup runs = %d", runs.Load())
	}
}

func TestVirtualComputersAutoSetupRunsSingleFlight(t *testing.T) {
	restore := setVirtualComputersAutoSetupTestHooks(t)
	defer restore()
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var runs atomic.Int32
	virtualComputersAutoSetupNeeded = func(*Server, virtualcomputers.ToolConfig) bool { return true }
	virtualComputersAutoSetupRunner = func(context.Context, *Server, virtualcomputers.ToolConfig) error {
		runs.Add(1)
		started <- struct{}{}
		<-release
		return nil
	}
	cfg := virtualcomputers.ToolConfig{Enabled: true, AutoSetup: true}

	if !virtualComputersTriggerAutoSetup(&Server{}, cfg) {
		t.Fatal("first auto setup was not started")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("auto setup runner did not start")
	}
	if virtualComputersTriggerAutoSetup(&Server{}, cfg) {
		t.Fatal("second concurrent auto setup must be coalesced")
	}
	close(release)
	waitForVirtualComputersAutoSetupIdle(t)
	if runs.Load() != 1 {
		t.Fatalf("auto setup runs = %d", runs.Load())
	}
}

func TestVirtualComputersAutoSetupReconcilesLatestConfiguration(t *testing.T) {
	restore := setVirtualComputersAutoSetupTestHooks(t)
	defer restore()
	firstStarted := make(chan struct{})
	firstCancelled := make(chan struct{})
	secondStarted := make(chan struct{})
	emergencyRelease := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(emergencyRelease) })
	virtualComputersAutoSetupNeeded = func(*Server, virtualcomputers.ToolConfig) bool { return true }
	virtualComputersAutoSetupRunner = func(ctx context.Context, _ *Server, cfg virtualcomputers.ToolConfig) error {
		if cfg.ControlPlane.Host == "first.example" {
			close(firstStarted)
			select {
			case <-ctx.Done():
				close(firstCancelled)
				return ctx.Err()
			case <-emergencyRelease:
				return nil
			}
		}
		if cfg.ControlPlane.Host == "second.example" {
			close(secondStarted)
		}
		return nil
	}
	first := virtualcomputers.ToolConfig{Enabled: true, AutoSetup: true, ControlPlane: virtualcomputers.ControlPlaneConfig{Host: "first.example"}}
	second := first
	second.ControlPlane.Host = "second.example"

	if !virtualComputersTriggerAutoSetup(&Server{}, first) {
		t.Fatal("first auto setup was not started")
	}
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first configuration did not start")
	}
	virtualComputersTriggerAutoSetup(&Server{}, second)
	select {
	case <-firstCancelled:
	case <-time.After(time.Second):
		releaseOnce.Do(func() { close(emergencyRelease) })
		t.Fatal("superseded setup was not cancelled")
	}
	select {
	case <-secondStarted:
	case <-time.After(time.Second):
		t.Fatal("latest configuration was not reconciled")
	}
	waitForVirtualComputersAutoSetupIdle(t)
}

func TestVirtualComputersAutoSetupDisableCancelsAndCleansUp(t *testing.T) {
	restore := setVirtualComputersAutoSetupTestHooks(t)
	defer restore()
	started := make(chan struct{})
	cancelled := make(chan struct{})
	emergencyRelease := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(emergencyRelease) })
	virtualComputersAutoSetupNeeded = func(*Server, virtualcomputers.ToolConfig) bool { return true }
	virtualComputersAutoSetupRunner = func(ctx context.Context, _ *Server, _ virtualcomputers.ToolConfig) error {
		close(started)
		select {
		case <-ctx.Done():
			close(cancelled)
			return ctx.Err()
		case <-emergencyRelease:
			return nil
		}
	}
	closedTunnel := make(chan struct{}, 1)
	virtualComputersManagementTunnel.Lock()
	virtualComputersManagementTunnel.key = "test"
	virtualComputersManagementTunnel.close = func() { closedTunnel <- struct{}{} }
	virtualComputersManagementTunnel.Unlock()
	defer resetVirtualComputersManagementTunnel()

	if !virtualComputersTriggerAutoSetup(&Server{}, virtualcomputers.ToolConfig{Enabled: true, AutoSetup: true}) {
		t.Fatal("auto setup was not started")
	}
	<-started
	virtualComputersTriggerAutoSetup(&Server{}, virtualcomputers.ToolConfig{})
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		releaseOnce.Do(func() { close(emergencyRelease) })
		t.Fatal("disabling integration did not cancel setup")
	}
	select {
	case <-closedTunnel:
	case <-time.After(time.Second):
		t.Fatal("disabling integration did not close management tunnel")
	}
	waitForVirtualComputersAutoSetupIdle(t)
}

func TestDefaultVirtualComputersAutoSetupNeededChecksPinnedRevision(t *testing.T) {
	revision := "outdated"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz", virtualcomputers.ManagementBasePath + "/":
			w.WriteHeader(http.StatusOK)
		case virtualcomputers.ManagementBasePath + "/.aurago-revision":
			_, _ = w.Write([]byte(revision))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	restore := setVirtualComputersManagementTestHooks(t, upstream.URL)
	defer restore()
	cfg := virtualcomputers.ToolConfig{
		Enabled:    true,
		AutoSetup:  true,
		BoringdURL: upstream.URL,
		ControlPlane: virtualcomputers.ControlPlaneConfig{
			Mode: virtualcomputers.ControlPlaneLocalHost,
		},
	}

	if !defaultVirtualComputersAutoSetupNeeded(&Server{}, cfg) {
		t.Fatal("outdated management revision must trigger setup")
	}
	revision = virtualcomputers.PinnedUpstreamRevision
	if defaultVirtualComputersAutoSetupNeeded(&Server{}, cfg) {
		t.Fatal("healthy pinned management revision must not trigger setup")
	}
}

func TestRegisterVirtualComputersRoutesTriggersAutoSetup(t *testing.T) {
	restore := setVirtualComputersAutoSetupTestHooks(t)
	defer restore()
	started := make(chan struct{}, 1)
	virtualComputersAutoSetupNeeded = func(*Server, virtualcomputers.ToolConfig) bool { return true }
	virtualComputersAutoSetupRunner = func(context.Context, *Server, virtualcomputers.ToolConfig) error {
		started <- struct{}{}
		return nil
	}
	cfg := &config.Config{}
	cfg.VirtualComputers.Enabled = true
	cfg.VirtualComputers.AutoSetup = true

	registerVirtualComputersRoutes(http.NewServeMux(), &Server{Cfg: cfg})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("route registration did not trigger automatic provisioning")
	}
	waitForVirtualComputersAutoSetupIdle(t)
}

func TestVirtualComputersConfigChangeTriggersAutoSetupWhenEnabled(t *testing.T) {
	restore := setVirtualComputersAutoSetupTestHooks(t)
	defer restore()
	started := make(chan struct{}, 1)
	virtualComputersAutoSetupNeeded = func(*Server, virtualcomputers.ToolConfig) bool { return true }
	virtualComputersAutoSetupRunner = func(context.Context, *Server, virtualcomputers.ToolConfig) error {
		started <- struct{}{}
		return nil
	}
	oldCfg := config.Config{}
	newCfg := config.Config{}
	newCfg.VirtualComputers.Enabled = true
	newCfg.VirtualComputers.AutoSetup = true

	virtualComputersAutoSetupAfterConfigChange(&Server{}, oldCfg, newCfg)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("enabling integration did not trigger automatic provisioning")
	}
	waitForVirtualComputersAutoSetupIdle(t)
}

func setVirtualComputersAutoSetupTestHooks(t *testing.T) func() {
	t.Helper()
	oldNeeded := virtualComputersAutoSetupNeeded
	oldRunner := virtualComputersAutoSetupRunner
	resetVirtualComputersAutoSetupState()
	return func() {
		waitForVirtualComputersAutoSetupIdle(t)
		virtualComputersAutoSetupNeeded = oldNeeded
		virtualComputersAutoSetupRunner = oldRunner
		resetVirtualComputersAutoSetupState()
	}
}

func waitForVirtualComputersAutoSetupIdle(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		virtualComputersAutoSetupState.Lock()
		running := virtualComputersAutoSetupState.running
		virtualComputersAutoSetupState.Unlock()
		if !running {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("auto setup did not become idle")
}
