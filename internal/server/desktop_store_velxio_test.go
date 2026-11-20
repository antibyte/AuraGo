package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/desktopstore"
	"aurago/internal/tools"
)

var velxioAllTools = []string{"create_circuit", "update_circuit", "import_wokwi_json", "export_wokwi_json", "generate_code_files", "compile_project", "run_project"}

func TestVelxioStoreMCPUsesGeneratedTokenAndGrantAllowlists(t *testing.T) {
	s, store := storeIntegrationFixture(t, "velxio")
	const tok = "velxio-token-unique-1234567890abcdef"
	if err := s.Vault.WriteSecret(desktopstore.VelxioMCPTokenKey(), tok); err != nil {
		t.Fatal(err)
	}
	if storeMCPName("velxio") != "aurago-store-velxio" {
		t.Fatalf("name = %q", storeMCPName("velxio"))
	}
	err := s.saveStoreIntegration(t.Context(), store, "velxio", storeIntegrationUpdate{URL: "http://evil.example:1", Credential: "user-typed", Read: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := desktopstore.ReadIntegration(s.Vault, "velxio")
	if err != nil {
		t.Fatal(err)
	}
	if settings.URL != "http://127.0.0.1:18080" || settings.Credential != "" {
		t.Fatalf("saved settings = %#v", settings)
	}

	cfg, err := s.storeMCPConfig("velxio", settings)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(cfg.URL, "/mcp") || cfg.Headers["Authorization"] != "Bearer "+tok || cfg.AllowDestructive || !slices.Equal(cfg.AllowedTools, velxioDesignTools) {
		t.Fatalf("read config = %#v", cfg)
	}
	exec := settings
	exec.Execute = true
	cfg, err = s.storeMCPConfig("velxio", exec)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AllowDestructive || !slices.Equal(cfg.AllowedTools, append(slices.Clone(velxioDesignTools), "compile_project", "run_project")) {
		t.Fatalf("execute config = %#v", cfg)
	}

	original := storeMCPProbeTools
	t.Cleanup(func() { storeMCPProbeTools = original })
	names := slices.Clone(velxioAllTools)
	storeMCPProbeTools = func(context.Context, tools.MCPServerConfig, *slog.Logger) ([]string, error) { return names, nil }
	if err = s.probeStoreMCP(t.Context(), "velxio", exec); err != nil {
		t.Fatal(err)
	}
	names = slices.DeleteFunc(slices.Clone(velxioAllTools), func(n string) bool { return n == "compile_project" })
	if err = s.probeStoreMCP(t.Context(), "velxio", exec); err == nil {
		t.Fatal("missing compile_project accepted")
	}

	status, err := s.storeIntegrationStatus(t.Context(), store, "velxio")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(status)
	if strings.Contains(string(raw), tok) || status["credential_stored"] != true {
		t.Fatalf("unsafe status: %s", raw)
	}

	app, ok, err := store.GetInstalled(t.Context(), "velxio")
	if err != nil || !ok {
		t.Fatal("velxio not installed")
	}
	app.TailscaleEnabled = true
	specs, _ := desktopStoreTailscaleProxySpecs([]desktopstore.InstalledApp{app})
	if len(specs) == 0 {
		t.Fatal("web port should still be proxied")
	}
	for _, spec := range specs {
		if strings.HasSuffix(spec.ID, "-agent-mcp") {
			t.Fatalf("agent port reached Tailscale: %#v", spec)
		}
	}

	if err = s.Vault.DeleteSecret(desktopstore.VelxioMCPTokenKey()); err != nil {
		t.Fatal(err)
	}
	if _, err = s.storeMCPConfig("velxio", settings); err == nil {
		t.Fatal("config built without a vault token")
	}
}

func TestVelxioConnectionSettlesWhenMCPStartsLate(t *testing.T) {
	s, store := storeIntegrationFixture(t, "velxio")
	if err := s.Vault.WriteSecret(desktopstore.VelxioMCPTokenKey(), "velxio-token-settle-1234567890abcdef"); err != nil {
		t.Fatal(err)
	}
	if err := s.saveStoreIntegration(t.Context(), store, "velxio", storeIntegrationUpdate{Read: true}, false); err != nil {
		t.Fatal(err)
	}
	overrideSettleVars(t, s)
	var calls atomic.Int32
	storeMCPProbeTools = func(context.Context, tools.MCPServerConfig, *slog.Logger) ([]string, error) {
		if calls.Add(1) <= 2 {
			return nil, errors.New("connection refused")
		}
		return velxioAllTools, nil
	}
	s.refreshStoreIntegration(store, "velxio")
	deadline := time.Now().Add(3 * time.Second)
	for !s.storeApplicationEnabled("velxio") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !s.storeApplicationEnabled("velxio") {
		t.Fatalf("connection never settled (probe calls: %d)", calls.Load())
	}
	if calls.Load() < 3 {
		t.Fatalf("probe calls = %d, want at least 3", calls.Load())
	}
}

// overrideSettleVars shortens the settle interval and restores the package
// vars only after no settle loop is running for velxio, so a loop never reads
// a var while the cleanup writes it.
func overrideSettleVars(t *testing.T, s *Server) {
	t.Helper()
	originalProbe, originalInterval := storeMCPProbeTools, storeIntegrationSettleInterval
	storeIntegrationSettleInterval = 20 * time.Millisecond
	t.Cleanup(func() {
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			s.storeIntegrations.Lock()
			running := s.storeIntegrations.settling["velxio"]
			s.storeIntegrations.Unlock()
			if !running {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		storeMCPProbeTools, storeIntegrationSettleInterval = originalProbe, originalInterval
	})
}

func TestVelxioReadOnlyDesktopKeepsOnlyDesignTools(t *testing.T) {
	s, _ := storeIntegrationFixture(t, "velxio")
	if err := s.Vault.WriteSecret(desktopstore.VelxioMCPTokenKey(), "velxio-token-readonly-1234567890abcdef"); err != nil {
		t.Fatal(err)
	}
	s.CfgMu.Lock()
	s.Cfg.VirtualDesktop.ReadOnly = true
	s.CfgMu.Unlock()
	cfg, err := s.storeMCPConfig("velxio", desktopstore.IntegrationSettings{Revision: "r1", URL: "http://127.0.0.1:18080", Read: true, Execute: true, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.AllowedTools) != 5 || !slices.Equal(cfg.AllowedTools, velxioDesignTools) || cfg.AllowDestructive {
		t.Fatalf("read-only config = %#v", cfg)
	}
}

func TestVelxioSettleLoopStopsWhenAppIsNotRunning(t *testing.T) {
	s, store := storeIntegrationFixture(t, "velxio")
	if err := s.Vault.WriteSecret(desktopstore.VelxioMCPTokenKey(), "velxio-token-stopped-1234567890abcdef"); err != nil {
		t.Fatal(err)
	}
	if err := s.saveStoreIntegration(t.Context(), store, "velxio", storeIntegrationUpdate{Read: true}, false); err != nil {
		t.Fatal(err)
	}
	op, err := store.StartAppOperation(t.Context(), "velxio", desktopstore.OperationStop, desktopstore.OperationRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.RunOperation(t.Context(), op.ID); err != nil {
		t.Fatal(err)
	}
	if app, ok, _ := store.GetInstalled(t.Context(), "velxio"); !ok || app.Status == desktopstore.AppStatusRunning {
		t.Fatalf("app still running: %#v", app.Status)
	}
	overrideSettleVars(t, s)
	var calls atomic.Int32
	storeMCPProbeTools = func(context.Context, tools.MCPServerConfig, *slog.Logger) ([]string, error) {
		calls.Add(1)
		return nil, errors.New("connection refused")
	}
	s.settleStoreIntegration("velxio")
	deadline := time.Now().Add(3 * time.Second)
	for {
		s.storeIntegrations.Lock()
		running := s.storeIntegrations.settling["velxio"]
		s.storeIntegrations.Unlock()
		if !running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("settle loop still running for a stopped app")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if calls.Load() != 0 {
		t.Fatalf("probe calls = %d, want 0", calls.Load())
	}
}

func TestVelxioDisconnectSucceedsWhenURLCannotBeDerived(t *testing.T) {
	s, store := storeIntegrationFixture(t, "velxio")
	if err := s.Vault.WriteSecret(desktopstore.VelxioMCPTokenKey(), "velxio-token-disconnect-1234567890abcdef"); err != nil {
		t.Fatal(err)
	}
	if err := s.saveStoreIntegration(t.Context(), store, "velxio", storeIntegrationUpdate{Read: true}, false); err != nil {
		t.Fatal(err)
	}
	before, err := desktopstore.ReadIntegration(s.Vault, "velxio")
	if err != nil || before.URL == "" {
		t.Fatalf("saved settings = %#v err=%v", before, err)
	}
	s.CfgMu.Lock()
	s.Cfg.Docker.Host = "tcp://docker.remote.example:2375"
	s.CfgMu.Unlock()
	app, _, _ := store.GetInstalled(t.Context(), "velxio")
	if _, err = s.storeIntegrationURL(t.Context(), app, before); err == nil {
		t.Fatal("test setup: URL derivation should fail for a remote Docker host")
	}
	if err = s.saveStoreIntegration(t.Context(), store, "velxio", storeIntegrationUpdate{}, true); err != nil {
		t.Fatalf("disconnect failed: %v", err)
	}
	after, err := desktopstore.ReadIntegration(s.Vault, "velxio")
	if err != nil {
		t.Fatal(err)
	}
	if after.Read || after.Execute || after.URL != before.URL || after.Credential != "" {
		t.Fatalf("settings after disconnect = %#v", after)
	}
}
