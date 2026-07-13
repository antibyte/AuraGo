package server

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
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
		case virtualcomputers.ManagementBasePath + "/aurago-revision":
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

func TestSavingSudoPasswordRetriesLocalAutoSetupDuringCooldown(t *testing.T) {
	restore := setVirtualComputersAutoSetupTestHooks(t)
	defer restore()
	started := make(chan struct{}, 1)
	virtualComputersAutoSetupNeeded = func(*Server, virtualcomputers.ToolConfig) bool { return true }
	virtualComputersAutoSetupRunner = func(context.Context, *Server, virtualcomputers.ToolConfig) error {
		started <- struct{}{}
		return nil
	}
	s, cfg := virtualComputersAutoSetupVaultTestServer(t)
	toolCfg := virtualcomputers.FromAuraConfig(cfg)
	virtualComputersAutoSetupState.Lock()
	virtualComputersAutoSetupState.desiredServer = s
	virtualComputersAutoSetupState.desiredConfig = toolCfg
	virtualComputersAutoSetupState.retryAfter = time.Now().Add(time.Hour)
	virtualComputersAutoSetupState.Unlock()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/vault/secrets", strings.NewReader(`{"key":"sudo_password","value":"vault-sudo-secret"}`))
	handleSetVaultSecret(s, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("saving sudo_password did not bypass the failed auto-setup cooldown")
	}
	waitForVirtualComputersAutoSetupIdle(t)
}

func TestSavingSudoPasswordSupersedesRunningLocalAutoSetup(t *testing.T) {
	restore := setVirtualComputersAutoSetupTestHooks(t)
	defer restore()
	firstStarted := make(chan struct{})
	firstCancelled := make(chan struct{})
	secondStarted := make(chan struct{})
	var runs atomic.Int32
	virtualComputersAutoSetupNeeded = func(*Server, virtualcomputers.ToolConfig) bool { return true }
	virtualComputersAutoSetupRunner = func(ctx context.Context, _ *Server, _ virtualcomputers.ToolConfig) error {
		if runs.Add(1) == 1 {
			close(firstStarted)
			<-ctx.Done()
			close(firstCancelled)
			return ctx.Err()
		}
		close(secondStarted)
		return nil
	}
	s, cfg := virtualComputersAutoSetupVaultTestServer(t)
	if !virtualComputersTriggerAutoSetup(s, virtualcomputers.FromAuraConfig(cfg)) {
		t.Fatal("initial auto setup was not started")
	}
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("initial auto setup did not run")
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/vault/secrets", strings.NewReader(`{"key":"sudo_password","value":"replacement-secret"}`))
	handleSetVaultSecret(s, rec, req)

	select {
	case <-firstCancelled:
	case <-time.After(time.Second):
		t.Fatal("saving sudo_password did not cancel the stale setup attempt")
	}
	select {
	case <-secondStarted:
	case <-time.After(time.Second):
		t.Fatal("saving sudo_password did not start a fresh setup attempt")
	}
	waitForVirtualComputersAutoSetupIdle(t)
}

func TestSavingSudoPasswordDoesNotResetManualLocalIntegration(t *testing.T) {
	restore := setVirtualComputersAutoSetupTestHooks(t)
	defer restore()
	s, cfg := virtualComputersAutoSetupVaultTestServer(t)
	cfg.VirtualComputers.AutoSetup = false
	var tunnelCloses atomic.Int32
	virtualComputersManagementTunnel.Lock()
	virtualComputersManagementTunnel.key = "manual-local"
	virtualComputersManagementTunnel.close = func() { tunnelCloses.Add(1) }
	virtualComputersManagementTunnel.Unlock()
	defer func() {
		virtualComputersManagementTunnel.Lock()
		virtualComputersManagementTunnel.key = ""
		virtualComputersManagementTunnel.close = nil
		virtualComputersManagementTunnel.Unlock()
	}()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/vault/secrets", strings.NewReader(`{"key":"sudo_password","value":"manual-secret"}`))
	handleSetVaultSecret(s, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := tunnelCloses.Load(); got != 0 {
		t.Fatalf("manual management tunnel closed %d time(s)", got)
	}
}

func virtualComputersAutoSetupVaultTestServer(t *testing.T) (*Server, *config.Config) {
	t.Helper()
	vault, err := security.NewVault(strings.Repeat("d", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	cfg := &config.Config{}
	cfg.VirtualComputers.Enabled = true
	cfg.VirtualComputers.AutoSetup = true
	cfg.VirtualComputers.ControlPlane.Mode = virtualcomputers.ControlPlaneLocalHost
	return &Server{Cfg: cfg, Vault: vault, Logger: slog.Default()}, cfg
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
