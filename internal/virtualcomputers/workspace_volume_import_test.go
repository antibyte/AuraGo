package virtualcomputers

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type legacyImportTestTransport struct {
	mu             sync.Mutex
	scanned        bool
	imported       bool
	expectedDigest string
}

func (transport *legacyImportTestTransport) Call(_ context.Context, machineID, method string, params interface{}, result interface{}) error {
	switch method {
	case "system.capabilities":
		capabilities := result.(*WorkspaceCapabilities)
		*capabilities = WorkspaceCapabilities{
			ProtocolVersion: WorkspaceProtocolVersion, MachineID: machineID, InstanceNonce: "import-nonce",
			Capabilities: []string{"volume.legacy_import"}, MaxMessageBytes: workspaceMaxWireMessageBytes,
		}
	case "volume.legacy_scan":
		scan := result.(*legacyVolumeScanRPCResult)
		*scan = legacyVolumeScanRPCResult{
			Paths: []string{"project"}, Entries: []LegacyVolumeImportEntry{{Path: "project", Kind: "directory", FileCount: 2, DirectoryCount: 1, SizeBytes: 42}},
			Digest: "scan-digest", TotalBytes: 42, FileCount: 2,
		}
		transport.mu.Lock()
		transport.scanned = true
		transport.mu.Unlock()
	case "volume.legacy_import":
		encoded, _ := json.Marshal(params)
		var request struct {
			ExpectedDigest string `json:"expected_digest"`
		}
		_ = json.Unmarshal(encoded, &request)
		transport.mu.Lock()
		transport.imported = true
		transport.expectedDigest = request.ExpectedDigest
		transport.mu.Unlock()
	}
	return nil
}

func TestLegacyVolumeImportRequiresSeparateAdminConfirmation(t *testing.T) {
	t.Parallel()
	var launch launchMachineRequest
	var createCalls, saveCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspace/capabilities":
			_ = json.NewEncoder(w).Encode(WorkspaceControlPlaneStatus{ProtocolVersion: WorkspaceProtocolVersion, AssetFingerprint: WorkspaceAssetFingerprint()})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/volumes/legacy-volume":
			_, _ = io.WriteString(w, `{"id":"legacy-volume"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines":
			_ = json.NewDecoder(r.Body).Decode(&launch)
			_, _ = io.WriteString(w, `{"id":"migration-vm","template":"python"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/volumes":
			createCalls++
			_, _ = io.WriteString(w, `{"id":"workspace-volume"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines/migration-vm/save":
			saveCalls++
			if r.URL.Query().Get("format") != WorkspaceVolumeFormat {
				t.Errorf("save format = %q", r.URL.Query().Get("format"))
			}
			_, _ = io.WriteString(w, `{}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/machines/migration-vm":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "virtual-computers.db"))
	if err != nil {
		t.Fatalf("OpenLedger: %v", err)
	}
	defer ledger.Close()
	if err := ledger.UpsertVolume(context.Background(), Volume{ID: "legacy-volume", Format: "legacy_root"}); err != nil {
		t.Fatalf("track legacy volume: %v", err)
	}
	transport := &legacyImportTestTransport{}
	manager, err := NewWorkspaceManager(ledger, slog.Default(), WorkspaceManagerOptions{
		ClientFactory: func(ToolConfig) (*Client, error) {
			return NewClient(ClientConfig{BaseURL: server.URL, Timeout: time.Second})
		},
		TransportFactory: func(*Client) WorkspaceTransport { return transport }, ReconcileInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("NewWorkspaceManager: %v", err)
	}
	defer manager.Close()
	cfg := ToolConfig{
		Enabled: true, ToolGate: true, AllowInternet: true, AllowVolumes: true,
		AgentControl: AgentControlConfig{Enabled: true, MaxActiveWorkspaces: 2, NetworkProfile: "internet_lan"},
	}
	admin := WorkspaceIdentity{SessionID: "desktop-admin", Actor: "user", Admin: true}
	preview, err := manager.BeginLegacyVolumeImport(context.Background(), cfg, admin, "legacy-volume", []string{"project"})
	if err != nil {
		t.Fatalf("BeginLegacyVolumeImport: %v", err)
	}
	if createCalls != 0 || saveCalls != 0 {
		t.Fatalf("scan mutated volumes: create=%d save=%d", createCalls, saveCalls)
	}
	if launch.VolumeFormat != "legacy_root" || launch.Volume != "legacy-volume" {
		t.Fatalf("migration launch = %+v", launch)
	}
	if preview.FileCount != 2 || preview.TotalBytes != 42 || len(preview.Entries) != 1 {
		t.Fatalf("preview = %+v", preview)
	}
	if _, err := manager.ConfirmLegacyVolumeImport(context.Background(), cfg, WorkspaceIdentity{SessionID: "agent"}, preview.ID, 3600); err == nil {
		t.Fatal("non-admin unexpectedly confirmed legacy import")
	}
	volume, err := manager.ConfirmLegacyVolumeImport(context.Background(), cfg, admin, preview.ID, 3600)
	if err != nil {
		t.Fatalf("ConfirmLegacyVolumeImport: %v", err)
	}
	if volume.ID != "workspace-volume" || volume.Format != WorkspaceVolumeFormat || createCalls != 1 || saveCalls != 1 {
		t.Fatalf("imported volume = %+v, create=%d save=%d", volume, createCalls, saveCalls)
	}
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if !transport.scanned || !transport.imported || transport.expectedDigest != "scan-digest" {
		t.Fatalf("transport state = %+v", transport)
	}
}
