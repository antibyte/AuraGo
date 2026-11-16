package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktopstore"
	"aurago/internal/security"
)

func storeIntegrationFixture(t *testing.T, app string) (*Server, *desktopstore.Service) {
	t.Helper()
	s, _ := newProviderTestServer(t, "virtual_desktop:\n  enabled: true\n  allow_agent_control: true\ntools:\n  virtual_desktop:\n    enabled: true\nagent:\n  allow_network_requests: true\ndocker:\n  enabled: true\n")
	store, _, _ := testInstalledStoreApp(t, app, 18000, 18080)
	s.DesktopStore = store
	t.Cleanup(s.stopStoreIntegrations)
	return s, store
}

func TestStoreIntegrationLegacySaveUsesExistingVaultKeyAndMasksResponse(t *testing.T) {
	s, store := storeIntegrationFixture(t, "adguard-home")
	const secret = "store-test-password-unique"
	err := s.saveStoreIntegration(t.Context(), store, "adguard-home", storeIntegrationUpdate{URL: "http://127.0.0.1:18000", Username: "admin", Credential: secret, Read: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !s.ConfigSnapshot().AdGuard.StoreManaged || !s.ConfigSnapshot().AdGuard.ReadOnly {
		t.Fatal("legacy policy not preserved")
	}
	value, err := s.Vault.ReadSecret("adguard_password")
	if err != nil || value != secret {
		t.Fatal("password not in legacy key")
	}
	settings, err := desktopstore.ReadIntegration(s.Vault, "adguard-home")
	if err != nil || settings.Credential != "" {
		t.Fatal("duplicate credential")
	}
	status, err := s.storeIntegrationStatus(t.Context(), store, "adguard-home")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(status)
	if strings.Contains(string(raw), secret) || status["credential_stored"] != true {
		t.Fatal("unsafe credential status")
	}
	yaml, err := os.ReadFile(s.Cfg.ConfigPath)
	if err != nil || strings.Contains(string(yaml), secret) {
		t.Fatal("secret in config")
	}
	if _, _, err = s.acquireStoreAccess(t.Context(), "adguard-home", settings.URL, false); err == nil {
		t.Fatal("save enabled unverified access")
	}
}

func TestStoreIntegrationManualConflictAndRevision(t *testing.T) {
	s, store := storeIntegrationFixture(t, "adguard-home")
	s.Cfg.AdGuard.URL = "http://manual.test"
	_, _ = s.Vault.ReadSecret("unused")
	req := storeIntegrationUpdate{URL: "http://127.0.0.1:18000", Username: "user", Read: true}
	if err := s.saveStoreIntegration(t.Context(), store, "adguard-home", req, false); err == nil {
		t.Fatal("overwrote manual binding")
	}
	req.SelectStoreInstance = true
	if err := s.saveStoreIntegration(t.Context(), store, "adguard-home", req, false); err != nil {
		t.Fatal(err)
	}
	if err := s.saveStoreIntegration(t.Context(), store, "adguard-home", req, false); err == nil {
		t.Fatal("accepted stale revision")
	}
}

func TestStoreIntegrationReadOnlyProbeAndRevocation(t *testing.T) {
	var calls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		user, password, ok := r.BasicAuth()
		if r.Method != "GET" || r.URL.Path != "/control/status" || !ok || user != "admin" || password != "test-store-secret" {
			t.Error("invalid read-only probe")
			w.WriteHeader(401)
			return
		}
		_, _ = w.Write([]byte(`{"version":"v0.107","running":true}`))
	}))
	defer upstream.Close()
	s, store := storeIntegrationFixture(t, "adguard-home")
	if err := s.saveStoreIntegration(t.Context(), store, "adguard-home", storeIntegrationUpdate{URL: upstream.URL, Username: "admin", Credential: "test-store-secret", Read: true}, false); err != nil {
		t.Fatal(err)
	}
	settings, _ := desktopstore.ReadIntegration(s.Vault, "adguard-home")
	if err := s.testStoreIntegration(t.Context(), store, "adguard-home", settings); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("probe calls: %d", calls)
	}
	ctx, release, err := s.acquireStoreAccess(context.Background(), "adguard-home", upstream.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, _, err = s.acquireStoreAccess(t.Context(), "adguard-home", upstream.URL, true); err == nil {
		t.Fatal("read-only write allowed")
	}
	s.revokeStoreIntegration("adguard-home")
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("running request not cancelled")
	}
	if _, _, err = s.acquireStoreAccess(t.Context(), "adguard-home", upstream.URL, false); err == nil {
		t.Fatal("revoked access accepted")
	}
}

func TestStoreIntegrationFailedSavePreservesConfigAndVault(t *testing.T) {
	s, _ := storeIntegrationFixture(t, "adguard-home")
	before, _ := os.ReadFile(s.Cfg.ConfigPath)
	if err := s.Vault.WriteSecret("adguard_password", "old-password"); err != nil {
		t.Fatal(err)
	}
	// A directory as the Vault path makes the second phase fail after YAML write.
	previousVault := s.Vault
	s.Vault, _ = security.NewVault(strings.Repeat("b", 64), t.TempDir())
	err := s.persistStoreIntegrationPatch(s.ConfigSnapshot(), map[string]interface{}{"adguard": map[string]interface{}{"enabled": true}}, []vaultMutation{{key: "adguard_password", value: "new-password"}})
	if err == nil {
		t.Fatal("failed Vault accepted")
	}
	after, _ := os.ReadFile(s.Cfg.ConfigPath)
	if string(after) != string(before) {
		t.Fatal("YAML not restored")
	}
	if value, err := previousVault.ReadSecret("adguard_password"); err != nil || value != "old-password" {
		t.Fatal("old credential lost")
	}
}

func TestStoreIntegrationDeleteDataPreservesOtherManualCredentials(t *testing.T) {
	s, store := storeIntegrationFixture(t, "adguard-home")
	if err := s.saveStoreIntegration(t.Context(), store, "adguard-home", storeIntegrationUpdate{URL: "http://127.0.0.1:18000", Username: "admin", Credential: "store-password"}, false); err != nil {
		t.Fatal(err)
	}
	cfg := s.ConfigSnapshot().Clone()
	cfg.AdGuard.StoreManaged = false
	cfg.AdGuard.URL = "http://manual.test"
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(cfg)
	s.CfgMu.Unlock()
	if err := s.Vault.WriteSecret("adguard_password", "manual-password"); err != nil {
		t.Fatal(err)
	}
	if err := s.deleteStoreIntegrationData(t.Context(), "adguard-home"); err != nil {
		t.Fatal(err)
	}
	value, err := s.Vault.ReadSecret("adguard_password")
	if err != nil || value != "manual-password" {
		t.Fatal("foreign credential deleted")
	}
	if _, err = s.Vault.ReadSecret(desktopstore.IntegrationKey("adguard-home")); err != security.ErrSecretNotFound {
		t.Fatal("owned settings retained")
	}
}

func TestStoreIntegrationDNSNeverBecomesTailscaleHTTPProxy(t *testing.T) {
	app := desktopstore.InstalledApp{AppID: "adguard-home", Status: desktopstore.AppStatusRunning, TailscaleEnabled: true, Ports: []desktopstore.PortBinding{{ID: "main", HostPort: 13000, Protocol: "tcp"}, {ID: "dns-tcp", HostPort: 53, Protocol: "tcp"}, {ID: "dns-udp", HostPort: 53, Protocol: "udp"}}}
	specs, _ := desktopStoreTailscaleProxySpecs([]desktopstore.InstalledApp{app})
	if len(specs) != 1 || specs[0].Port != 13000 {
		t.Fatalf("DNS exposed to HTTP proxy: %+v", specs)
	}
}
