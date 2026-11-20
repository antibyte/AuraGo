package server

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/security"
	"aurago/internal/virtualcomputers"
)

func reviewFixesTokenManager(t *testing.T, s *Server) (writeToken, adminToken string) {
	t.Helper()
	dir := t.TempDir()
	vault, err := security.NewVault(strings.Repeat("d", 64), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := security.NewTokenManager(vault, filepath.Join(dir, "tokens.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if writeToken, _, err = tokens.Create("vc write", []string{desktopScopeWrite}, nil); err != nil {
		t.Fatal(err)
	}
	if adminToken, _, err = tokens.Create("vc admin", []string{desktopScopeAdmin}, nil); err != nil {
		t.Fatal(err)
	}
	s.TokenManager = tokens
	s.Vault = vault
	return writeToken, adminToken
}

func reviewFixesBearerRequest(mux *http.ServeMux, method, path, token string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	mux.ServeHTTP(rec, req)
	return rec
}

// 2: assigning a computer to the agent needs the admin scope, not just write.
func TestVirtualComputersAssignAgentRequiresAdminScope(t *testing.T) {
	_, upstream := newFakeMachinesBoringd(t)
	s, ledger, mux := newMachineTestServer(t, upstream.URL)
	s.Cfg.VirtualComputers.AgentControl.Enabled = true
	s.Cfg.VirtualComputers.AgentControl.MaxActiveWorkspaces = 2
	s.Cfg.VirtualComputers.AgentControl.IdleTTLSeconds = 600
	s.Cfg.VirtualComputers.AgentControl.MaxWorkspaceSeconds = 3600
	writeToken, adminToken := reviewFixesTokenManager(t, s)
	manager, err := virtualcomputers.NewWorkspaceManager(ledger, slog.Default(), virtualcomputers.WorkspaceManagerOptions{
		ClientFactory: func(virtualcomputers.ToolConfig) (*virtualcomputers.Client, error) {
			return virtualcomputers.NewClient(virtualcomputers.ClientConfig{BaseURL: upstream.URL, Timeout: 2 * time.Second})
		},
		TransportFactory:  func(*virtualcomputers.Client) virtualcomputers.WorkspaceTransport { return &serverAdoptTransport{} },
		ReconcileInterval: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	s.VirtualWorkspaceManager = manager

	path := "/api/virtual-computers/machines/vm-1/assign-agent"
	if rec := reviewFixesBearerRequest(mux, http.MethodPost, path, writeToken); rec.Code != http.StatusForbidden {
		t.Fatalf("write-scope status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := reviewFixesBearerRequest(mux, http.MethodPost, path, adminToken); rec.Code != http.StatusOK {
		t.Fatalf("admin status = %d body=%s", rec.Code, rec.Body.String())
	}
}

// 8: manual install/repair and the auto-setup trigger exclude each other.
func TestVirtualComputersSetupSingleFlightBothWays(t *testing.T) {
	_, upstream := newFakeMachinesBoringd(t)
	s, _, mux := newMachineTestServer(t, upstream.URL)
	s.Cfg.VirtualComputers.AutoSetup = true

	// Auto-setup running: the manual handler answers 409 without blocking.
	virtualComputersAutoSetupState.Lock()
	virtualComputersAutoSetupState.running = true
	virtualComputersAutoSetupState.Unlock()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/virtual-computers/setup/install", bytes.NewReader([]byte(`{}`))))
	if payload := decodeVolumeTestBody(t, rec); rec.Code != http.StatusConflict || payload["code"] != "setup_in_progress" {
		t.Fatalf("manual during auto status=%d body=%s", rec.Code, rec.Body.String())
	}
	virtualComputersAutoSetupState.Lock()
	virtualComputersAutoSetupState.running = false
	virtualComputersAutoSetupState.Unlock()

	// Manual run holding the shared mutex: the auto trigger must not start a second install.
	var runs atomic.Int32
	prevNeeded, prevRunner := virtualComputersAutoSetupNeeded, virtualComputersAutoSetupRunner
	virtualComputersAutoSetupNeeded = func(*Server, virtualcomputers.ToolConfig) bool { return true }
	virtualComputersAutoSetupRunner = func(_ context.Context, _ *Server, _ virtualcomputers.ToolConfig) error {
		runs.Add(1)
		return nil
	}
	t.Cleanup(func() {
		virtualComputersAutoSetupNeeded, virtualComputersAutoSetupRunner = prevNeeded, prevRunner
		virtualComputersAutoSetupState.Lock()
		virtualComputersAutoSetupState.running = false
		virtualComputersAutoSetupState.retryAfter = time.Time{}
		virtualComputersAutoSetupState.generation++
		virtualComputersAutoSetupState.Unlock()
	})
	virtualComputersManualSetup.Lock()
	cfg := virtualcomputers.ToolConfig{Enabled: true, AutoSetup: true}
	started := virtualComputersTriggerAutoSetupWithForce(s, cfg, true)
	virtualComputersManualSetup.Unlock()
	time.Sleep(50 * time.Millisecond)
	if started || runs.Load() != 0 || virtualComputersAutoSetupRunning() {
		t.Fatalf("auto setup started next to a manual run: started=%v runs=%d running=%v", started, runs.Load(), virtualComputersAutoSetupRunning())
	}
}

// 9: deleting a volume fails closed when the ledger is unavailable.
func TestVirtualComputersVolumeDeleteFailsClosedWithoutLedger(t *testing.T) {
	fake, upstream := newFakeRetentionBoringd(t, true)
	s, _, mux := newVolumeTestServer(t, upstream.URL)
	fake.volumes["vol-x"] = map[string]interface{}{"id": "vol-x"}
	s.VirtualComputersDB = nil
	s.Cfg.SQLite.VirtualComputersPath = t.TempDir() // a directory cannot be opened as a ledger
	rec := volumeTestRequest(mux, http.MethodDelete, "/api/virtual-computers/volumes/vol-x", nil)
	if payload := decodeVolumeTestBody(t, rec); rec.Code != http.StatusServiceUnavailable || payload["code"] != "storage_unavailable" {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.count("DELETE /v1/volumes/") != 0 {
		t.Fatal("delete reached boringd without a ledger")
	}
}
