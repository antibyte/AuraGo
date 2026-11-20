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
	originalProbe, originalInterval := storeMCPProbeTools, storeIntegrationSettleInterval
	t.Cleanup(func() { storeMCPProbeTools, storeIntegrationSettleInterval = originalProbe, originalInterval })
	storeIntegrationSettleInterval = 20 * time.Millisecond
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
