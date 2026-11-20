package virtualcomputers

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLedgerMigratesToV5WithBackup(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "virtual_computers.db")
	ledger, err := OpenLedger(path)
	if err != nil {
		t.Fatalf("OpenLedger: %v", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`ALTER TABLE volumes DROP COLUMN persistent`,
		`ALTER TABLE volumes DROP COLUMN locked`,
		`ALTER TABLE volumes DROP COLUMN used_bytes`,
		`ALTER TABLE volumes DROP COLUMN file_count`,
		`ALTER TABLE machines DROP COLUMN label`,
		`ALTER TABLE machines DROP COLUMN volume_id`,
		`ALTER TABLE workspaces DROP COLUMN origin`,
		`UPDATE schema_meta SET value = '4' WHERE key = 'schema_version'`,
	} {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	_ = raw.Close()

	ledger, err = OpenLedger(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer ledger.Close()
	version, err := ledger.schemaVersion(context.Background())
	if err != nil || version != 5 {
		t.Fatalf("schema version = %d, %v; want 5", version, err)
	}
	if _, err := os.Stat(path + ".v4.bak"); err != nil {
		t.Fatalf("v4 backup missing: %v", err)
	}
	ctx := context.Background()
	if err := ledger.UpsertVolume(ctx, Volume{ID: "vol-1", Name: "keep", Persistent: true, Locked: true, UsedBytes: 12, FileCount: 3}); err != nil {
		t.Fatalf("UpsertVolume: %v", err)
	}
	// An empty name from boringd must not wipe the user-chosen one.
	if err := ledger.UpsertVolume(ctx, Volume{ID: "vol-1", Persistent: true, Locked: false, UsedBytes: 20, FileCount: 4}); err != nil {
		t.Fatalf("UpsertVolume refresh: %v", err)
	}
	got, ok, err := ledger.GetVolume(ctx, "vol-1")
	if err != nil || !ok {
		t.Fatalf("GetVolume = %v, %v", ok, err)
	}
	if got.Name != "keep" || !got.Persistent || got.Locked || got.UsedBytes != 20 || got.FileCount != 4 {
		t.Fatalf("volume = %+v", got)
	}
	if renamed, err := ledger.SetVolumeName(ctx, "vol-1", ""); err != nil || !renamed {
		t.Fatalf("SetVolumeName clear = %v, %v", renamed, err)
	}
	if got, _, _ = ledger.GetVolume(ctx, "vol-1"); got.Name != "" {
		t.Fatalf("name not cleared: %q", got.Name)
	}
	if err := ledger.UpsertWorkspace(ctx, Workspace{ID: "ws-1", OwnerSessionID: "s", MachineID: "vm-1", State: WorkspaceStateReady, VolumeID: "vol-1"}); err != nil {
		t.Fatalf("UpsertWorkspace: %v", err)
	}
	if workspace, _, _ := ledger.GetWorkspace(ctx, "ws-1"); workspace.Origin != WorkspaceOriginLaunched {
		t.Fatalf("origin = %q, want launched", workspace.Origin)
	}
	if active, err := ledger.ActiveWorkspacesForVolume(ctx, "vol-1"); err != nil || len(active) != 1 {
		t.Fatalf("ActiveWorkspacesForVolume = %v, %v", active, err)
	}
}

func TestVolumeUnmarshalParsesRetentionView(t *testing.T) {
	t.Parallel()
	var patched Volume
	if err := json.Unmarshal([]byte(`{"id":"vol-1","created_at":"2026-01-02T03:04:05Z","expires_at":"","quota_mb":2048,"used_bytes":4096,"files":7,"persistent":true,"locked":true}`), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.ExpiresAt != nil || !patched.Persistent || !patched.Locked || patched.UsedBytes != 4096 || patched.FileCount != 7 || patched.QuotaMB != 2048 || patched.CreatedAt == nil {
		t.Fatalf("patched view = %+v", patched)
	}
	var legacy Volume
	if err := json.Unmarshal([]byte(`{"id":"vol-2","created_at":"2026-01-02T03:04:05Z","expires_at":"2026-02-02T03:04:05Z","quota_mb":256}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.ExpiresAt == nil || legacy.Persistent || legacy.Locked || legacy.QuotaMB != 256 {
		t.Fatalf("legacy meta = %+v", legacy)
	}
}

func TestClientVolumeRetentionContracts(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var bodies []string
	probeStatus, probeBody := http.StatusNotFound, `{"error":"no such volume"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, r.Method+" "+r.URL.Path+" "+string(data))
		status, body := probeStatus, probeBody
		mu.Unlock()
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/volumes":
			_, _ = io.WriteString(w, `{"id":"vol-new"}`)
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/volumes/vol-0000000000":
			w.WriteHeader(status)
			_, _ = io.WriteString(w, body)
		case r.Method == http.MethodPatch:
			_, _ = io.WriteString(w, `{"id":"vol-1","locked":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewClient(ClientConfig{BaseURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := client.CreateVolume(ctx, 60); err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateVolumeWithOptions(ctx, VolumeCreateOptions{Persistent: true, Locked: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateVolumeWithOptions(ctx, VolumeCreateOptions{}); err == nil {
		t.Fatal("a volume without ttl must be persistent")
	}
	locked := true
	if _, err := client.UpdateVolume(ctx, "vol-1", VolumeUpdate{Locked: &locked}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	want := []string{`POST /v1/volumes {"ttl_seconds":60}`, `POST /v1/volumes {"locked":true,"persistent":true}`, `PATCH /v1/volumes/vol-1 {"locked":true}`}
	if len(bodies) != len(want) {
		t.Fatalf("requests = %q", bodies)
	}
	for i := range want {
		if bodies[i] != want[i] {
			t.Fatalf("request %d = %q, want %q", i, bodies[i], want[i])
		}
	}
	mu.Unlock()

	for _, tc := range []struct {
		status int
		body   string
		want   bool
	}{
		{http.StatusNotFound, `{"error":"no such volume"}`, true},
		{http.StatusBadRequest, `{"error":"invalid JSON"}`, true},
		{http.StatusMethodNotAllowed, "method not allowed", false},
		{http.StatusNotFound, "404 page not found", false},
	} {
		mu.Lock()
		probeStatus, probeBody = tc.status, tc.body
		mu.Unlock()
		got, err := client.ProbeVolumeRetention(ctx)
		if err != nil || got != tc.want {
			t.Fatalf("probe %d %q = %v, %v; want %v", tc.status, tc.body, got, err, tc.want)
		}
	}
	mu.Lock()
	probeStatus, probeBody = http.StatusInternalServerError, `{"error":"boom"}`
	mu.Unlock()
	if got, err := client.ProbeVolumeRetention(ctx); err == nil || got {
		t.Fatalf("probe 500 = %v, %v; want an error", got, err)
	}
}

func TestListTrackedVolumesPreservesNameAndTakesUpstreamRetention(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/volumes/vol-a" {
			_, _ = io.WriteString(w, `{"id":"vol-a","expires_at":"","quota_mb":2048,"used_bytes":99,"files":5,"persistent":true,"locked":true}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	client, _ := NewClient(ClientConfig{BaseURL: server.URL, Timeout: time.Second})
	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "vc.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	ctx := context.Background()
	if err := ledger.UpsertVolume(ctx, Volume{ID: "vol-a", Name: "Projects", Format: WorkspaceVolumeFormat, Availability: "available", StorageEpochID: 3}); err != nil {
		t.Fatal(err)
	}
	volumes, err := ListTrackedVolumes(ctx, ledger, client)
	if err != nil || len(volumes) != 1 {
		t.Fatalf("ListTrackedVolumes = %v, %v", volumes, err)
	}
	got := volumes[0]
	if got.Name != "Projects" || got.Format != WorkspaceVolumeFormat || got.StorageEpochID != 3 {
		t.Fatalf("local fields lost: %+v", got)
	}
	if !got.Persistent || !got.Locked || got.UsedBytes != 99 || got.FileCount != 5 || got.ExpiresAt != nil || got.VerificationStatus != "verified" {
		t.Fatalf("upstream retention not applied: %+v", got)
	}
	// An untracked volume becomes legacy_root / available.
	tracked, err := TrackUpstreamVolume(ctx, ledger, Volume{ID: "vol-b", Locked: true})
	if err != nil || tracked.Format != "legacy_root" || tracked.Availability != "available" || !tracked.Locked {
		t.Fatalf("TrackUpstreamVolume = %+v, %v", tracked, err)
	}
}

func TestClassifyErrorMapsVolumeLockAndQuota(t *testing.T) {
	t.Parallel()
	locked := ClassifyError(RESTError{StatusCode: http.StatusLocked, Path: "/v1/volumes/vol-1", Body: `{"error":"volume is locked"}`})
	if locked.Code != "volume_locked" || locked.HTTPStatus != http.StatusConflict {
		t.Fatalf("locked = %+v", locked)
	}
	quota := ClassifyError(RESTError{StatusCode: http.StatusRequestEntityTooLarge, Path: "/v1/machines/vm-1/save", Body: `{"error":"too big"}`})
	if quota.Code != "volume_quota_exceeded" || quota.HTTPStatus != http.StatusRequestEntityTooLarge || !strings.Contains(quota.Message, "volume_quota_mb") {
		t.Fatalf("quota = %+v", quota)
	}
}

func TestSetupInstallWritesVolumeRetentionLimits(t *testing.T) {
	t.Parallel()
	script := (SetupManager{InstallOptions: SetupInstallOptions{VolumeQuotaMB: 4096, VolumeMaxTTLSeconds: 7 * 86400}}).installScript()
	for _, want := range []string{"BORING_VOLUME_QUOTA_MB_VALUE=4096", "BORING_VOLUME_TTL_MAX_VALUE=604800", "BORING_VOLUME_QUOTA_MB=${BORING_VOLUME_QUOTA_MB_VALUE}", "BORING_VOLUME_TTL_MAX=${BORING_VOLUME_TTL_MAX_VALUE}"} {
		if !strings.Contains(script, want) {
			t.Fatalf("script is missing %q", want)
		}
	}
	defaults := (SetupManager{}).installScript()
	for _, want := range []string{"BORING_VOLUME_QUOTA_MB_VALUE=2048", "BORING_VOLUME_TTL_MAX_VALUE=2592000"} {
		if !strings.Contains(defaults, want) {
			t.Fatalf("default script is missing %q", want)
		}
	}
}

func TestCheckpointUsesConfiguredVolumeTTLAndOptions(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var createBodies, saves []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspace/capabilities":
			_ = json.NewEncoder(w).Encode(WorkspaceControlPlaneStatus{ProtocolVersion: WorkspaceProtocolVersion, AssetFingerprint: WorkspaceAssetFingerprint()})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/volumes":
			data, _ := io.ReadAll(r.Body)
			mu.Lock()
			createBodies = append(createBodies, string(data))
			n := len(createBodies)
			mu.Unlock()
			_, _ = io.WriteString(w, `{"id":"vol-`+string(rune('0'+n))+`","expires_at":"","persistent":true,"used_bytes":10,"files":1}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines/vm-1/save":
			mu.Lock()
			saves = append(saves, r.URL.RawQuery)
			mu.Unlock()
			_, _ = io.WriteString(w, `{}`)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/volumes/vol-"):
			_, _ = io.WriteString(w, `{"id":"`+strings.TrimPrefix(r.URL.Path, "/v1/volumes/")+`","expires_at":"","persistent":true,"used_bytes":2048,"files":9}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "vc.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	manager, err := NewWorkspaceManager(ledger, slog.Default(), WorkspaceManagerOptions{
		ClientFactory: func(ToolConfig) (*Client, error) {
			return NewClient(ClientConfig{BaseURL: server.URL, Timeout: time.Second})
		},
		TransportFactory:  func(*Client) WorkspaceTransport { return &workspaceTestTransport{} },
		ReconcileInterval: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	cfg := ToolConfig{Enabled: true, ToolGate: true, AllowVolumes: true, VolumeMaxTTLSeconds: 7 * 86400,
		AgentControl: AgentControlConfig{Enabled: true, DefaultTemplate: "desktop", MaxActiveWorkspaces: 2, IdleTTLSeconds: 600, MaxWorkspaceSeconds: 7200}}
	owner := WorkspaceIdentity{SessionID: "session-a", Actor: "agent"}
	ctx := context.Background()
	now := time.Now().UTC()
	if err := ledger.UpsertWorkspace(ctx, Workspace{ID: "ws-1", OwnerSessionID: "session-a", MachineID: "vm-1", State: WorkspaceStateReady,
		Template: "desktop", LeaseExpiresAt: now.Add(time.Hour), MaxExpiresAt: now.Add(2 * time.Hour)}); err != nil {
		t.Fatal(err)
	}

	volume, err := manager.Checkpoint(ctx, cfg, owner, "ws-1", 0)
	if err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
	if volume.ID != "vol-1" || volume.Format != WorkspaceVolumeFormat {
		t.Fatalf("volume = %+v", volume)
	}
	mu.Lock()
	first := createBodies[0]
	mu.Unlock()
	if first != `{"ttl_seconds":604800}` {
		t.Fatalf("default checkpoint ttl body = %s, want the configured 7-day maximum", first)
	}

	// An explicit existing workspace_v2 volume is overwritten, not recreated.
	volume, err = manager.CheckpointWithOptions(ctx, cfg, owner, "ws-1", CheckpointOptions{Name: "Snapshot", Persistent: true, VolumeID: "vol-1"})
	if err != nil {
		t.Fatalf("CheckpointWithOptions existing: %v", err)
	}
	if volume.ID != "vol-1" || volume.UsedBytes != 2048 || volume.FileCount != 9 || !volume.Persistent {
		t.Fatalf("refreshed volume = %+v", volume)
	}
	// Unbind the volume so the next checkpoint creates a fresh, named, permanent one.
	if err := ledger.UpsertWorkspace(ctx, Workspace{ID: "ws-1", OwnerSessionID: "session-a", MachineID: "vm-1", State: WorkspaceStateReady, Template: "desktop",
		LeaseExpiresAt: now.Add(time.Hour), MaxExpiresAt: now.Add(2 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	named, err := manager.CheckpointWithOptions(ctx, cfg, owner, "ws-1", CheckpointOptions{Name: "Snapshot", Persistent: true})
	if err != nil {
		t.Fatalf("CheckpointWithOptions new: %v", err)
	}
	mu.Lock()
	second := createBodies[len(createBodies)-1]
	mu.Unlock()
	if second != `{"persistent":true}` || named.Name != "Snapshot" {
		t.Fatalf("persistent create body = %s, name = %q", second, named.Name)
	}
	if tracked, ok, _ := ledger.GetVolume(ctx, named.ID); !ok || tracked.Name != "Snapshot" || tracked.Format != WorkspaceVolumeFormat {
		t.Fatalf("tracked = %+v ok=%v", tracked, ok)
	}
	if _, err := manager.CheckpointWithOptions(ctx, cfg, owner, "ws-1", CheckpointOptions{VolumeID: "vol-unknown"}); err == nil {
		t.Fatal("an unknown explicit volume must be rejected")
	}
}

func TestExecuteToolRefusesDeletingLockedVolume(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	deletes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			mu.Lock()
			deletes++
			mu.Unlock()
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "vc.db")
	ledger, err := OpenLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := ledger.UpsertVolume(ctx, Volume{ID: "vol-locked", Locked: true}); err != nil {
		t.Fatal(err)
	}
	if err := ledger.UpsertVolume(ctx, Volume{ID: "vol-free"}); err != nil {
		t.Fatal(err)
	}
	_ = ledger.Close()
	cfg := ToolConfig{Enabled: true, ToolGate: true, AllowVolumes: true, BoringdURL: server.URL, LedgerPath: path}

	var refused map[string]interface{}
	if err := json.Unmarshal([]byte(ExecuteTool(ctx, cfg, map[string]interface{}{"operation": "delete_volume", "volume_id": "vol-locked"})), &refused); err != nil {
		t.Fatal(err)
	}
	if refused["status"] != "error" || refused["code"] != "volume_locked" {
		t.Fatalf("locked delete = %v", refused)
	}
	mu.Lock()
	if deletes != 0 {
		t.Fatalf("upstream DELETE reached boringd %d times for a locked volume", deletes)
	}
	mu.Unlock()
	var ok map[string]interface{}
	if err := json.Unmarshal([]byte(ExecuteTool(ctx, cfg, map[string]interface{}{"operation": "delete_volume", "volume_id": "vol-free"})), &ok); err != nil {
		t.Fatal(err)
	}
	if ok["status"] != "ok" {
		t.Fatalf("unlocked delete = %v", ok)
	}
	mu.Lock()
	defer mu.Unlock()
	if deletes != 1 {
		t.Fatalf("DELETE count = %d, want 1", deletes)
	}
}
