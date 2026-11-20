package virtualcomputers

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func toolResultMap(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("tool result is not JSON: %v: %s", err, raw)
	}
	return out
}

type reviewUpstream struct {
	mu       sync.Mutex
	requests []string
	bodies   []string
}

func (u *reviewUpstream) count(prefix string) int {
	u.mu.Lock()
	defer u.mu.Unlock()
	n := 0
	for _, r := range u.requests {
		if strings.HasPrefix(r, prefix) {
			n++
		}
	}
	return n
}

func (u *reviewUpstream) bodyOf(request string) string {
	u.mu.Lock()
	defer u.mu.Unlock()
	body := ""
	for i, r := range u.requests {
		if r == request {
			body = u.bodies[i]
		}
	}
	return body
}

func newReviewUpstream(t *testing.T) (*reviewUpstream, *httptest.Server) {
	t.Helper()
	u := &reviewUpstream{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		u.mu.Lock()
		u.requests = append(u.requests, r.Method+" "+r.URL.Path)
		u.bodies = append(u.bodies, string(data))
		u.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspace/capabilities":
			_ = json.NewEncoder(w).Encode(WorkspaceControlPlaneStatus{ProtocolVersion: WorkspaceProtocolVersion, AssetFingerprint: WorkspaceAssetFingerprint()})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/volumes":
			_, _ = io.WriteString(w, `{"id":"vol-new"}`)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/volumes/"):
			_, _ = io.WriteString(w, `{"id":"`+strings.TrimPrefix(r.URL.Path, "/v1/volumes/")+`"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines":
			_, _ = io.WriteString(w, `{"machines":[{"id":"vm-live"}]}`)
		default:
			_, _ = io.WriteString(w, `{}`)
		}
	}))
	t.Cleanup(server.Close)
	return u, server
}

// 1: an unreadable ledger must refuse machine operations instead of skipping the guard.
func TestToolWorkspaceGuardFailsClosedOnLedgerError(t *testing.T) {
	prev := DefaultWorkspaceManager()
	SetDefaultWorkspaceManager(nil)
	t.Cleanup(func() { SetDefaultWorkspaceManager(prev) })
	upstream, server := newReviewUpstream(t)
	cfg := ToolConfig{Enabled: true, ToolGate: true, AllowInternet: true, BoringdURL: server.URL, LedgerPath: t.TempDir()}
	for _, op := range []string{"destroy", "exec"} {
		out := toolResultMap(t, ExecuteTool(context.Background(), cfg, map[string]interface{}{"operation": op, "machine_id": "vm-1", "command": "id"}))
		if out["code"] != "storage_unavailable" {
			t.Fatalf("%s result = %v", op, out)
		}
	}
	if len(upstream.requests) != 0 {
		t.Fatalf("refused operations reached boringd: %v", upstream.requests)
	}
}

type checkpointFixture struct {
	manager  *WorkspaceManager
	ledger   *Ledger
	upstream *reviewUpstream
	cfg      ToolConfig
	owner    WorkspaceIdentity
}

func newCheckpointFixture(t *testing.T) *checkpointFixture {
	t.Helper()
	upstream, server := newReviewUpstream(t)
	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "vc.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ledger.Close() })
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
	t.Cleanup(func() { _ = manager.Close() })
	now := time.Now().UTC()
	if err := ledger.UpsertWorkspace(context.Background(), Workspace{ID: "ws-1", OwnerSessionID: "session-a", MachineID: "vm-1", State: WorkspaceStateReady,
		Template: "desktop", LeaseExpiresAt: now.Add(time.Hour), MaxExpiresAt: now.Add(2 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	return &checkpointFixture{manager: manager, ledger: ledger, upstream: upstream, owner: WorkspaceIdentity{SessionID: "session-a", Actor: "agent"},
		cfg: ToolConfig{Enabled: true, ToolGate: true, AllowVolumes: true, VolumeMaxTTLSeconds: 7 * 86400,
			AgentControl: AgentControlConfig{Enabled: true, DefaultTemplate: "desktop", MaxActiveWorkspaces: 2, IdleTTLSeconds: 600, MaxWorkspaceSeconds: 7200}}}
}

// 3 + 4: the agent cannot overwrite a locked volume and its TTL is clamped; the human path may overwrite.
func TestCheckpointLockedVolumeAndTTLClamp(t *testing.T) {
	f := newCheckpointFixture(t)
	ctx := context.Background()
	if err := f.ledger.UpsertVolume(ctx, Volume{ID: "vol-lock", Format: WorkspaceVolumeFormat, Availability: "available", Locked: true}); err != nil {
		t.Fatal(err)
	}
	_, err := f.manager.CheckpointWithOptions(ctx, f.cfg, f.owner, "ws-1", CheckpointOptions{VolumeID: "vol-lock"})
	if rpc, ok := err.(WorkspaceRPCError); !ok || rpc.Code != "volume_locked" {
		t.Fatalf("agent checkpoint into locked volume err = %v", err)
	}
	if f.upstream.count("POST /v1/machines/vm-1/save") != 0 {
		t.Fatal("refused checkpoint reached the upstream save")
	}
	volume, err := f.manager.CheckpointWithOptions(ctx, f.cfg, f.owner, "ws-1", CheckpointOptions{VolumeID: "vol-lock", AllowLocked: true})
	if err != nil || !volume.Locked {
		t.Fatalf("human checkpoint into locked volume = %+v, %v", volume, err)
	}
	if f.upstream.count("POST /v1/machines/vm-1/save") != 1 {
		t.Fatalf("human checkpoint did not save: %v", f.upstream.requests)
	}
	// Unbind, then request an absurd TTL: boringd must see the configured maximum.
	if err := f.ledger.UpsertWorkspace(ctx, Workspace{ID: "ws-1", OwnerSessionID: "session-a", MachineID: "vm-1", State: WorkspaceStateReady, Template: "desktop",
		LeaseExpiresAt: time.Now().Add(time.Hour), MaxExpiresAt: time.Now().Add(2 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.manager.CheckpointWithOptions(ctx, f.cfg, f.owner, "ws-1", CheckpointOptions{TTLSeconds: 100 * 86400}); err != nil {
		t.Fatal(err)
	}
	if create := f.upstream.bodyOf("POST /v1/volumes"); create != `{"ttl_seconds":604800}` {
		t.Fatalf("checkpoint create body = %s, want clamped 7 days", create)
	}
}

// 3 + 4 legacy tool: save_machine refuses locked volumes, create_volume clamps ttl_seconds.
func TestLegacyToolLockedSaveAndCreateVolumeClamp(t *testing.T) {
	prev := DefaultWorkspaceManager()
	SetDefaultWorkspaceManager(nil)
	t.Cleanup(func() { SetDefaultWorkspaceManager(prev) })
	upstream, server := newReviewUpstream(t)
	path := filepath.Join(t.TempDir(), "vc.db")
	ledger, err := OpenLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := ledger.UpsertVolume(ctx, Volume{ID: "vol-lock", Locked: true}); err != nil {
		t.Fatal(err)
	}
	_ = ledger.Close()
	cfg := ToolConfig{Enabled: true, ToolGate: true, AllowVolumes: true, BoringdURL: server.URL, LedgerPath: path, VolumeMaxTTLSeconds: 3600}
	out := toolResultMap(t, ExecuteTool(ctx, cfg, map[string]interface{}{"operation": "save_machine", "machine_id": "vm-1", "volume_id": "vol-lock"}))
	if out["code"] != "volume_locked" || upstream.count("POST /v1/machines/vm-1/save") != 0 {
		t.Fatalf("save_machine into locked = %v requests=%v", out, upstream.requests)
	}
	out = toolResultMap(t, ExecuteTool(ctx, cfg, map[string]interface{}{"operation": "create_volume", "ttl_seconds": 99999999}))
	if out["status"] != "ok" {
		t.Fatalf("create_volume = %v", out)
	}
	if body := upstream.bodyOf("POST /v1/volumes"); body != `{"ttl_seconds":3600}` {
		t.Fatalf("create_volume body = %s, want clamped 3600", body)
	}
}

// 5: an upstream view without retention fields must not reset local locked/persistent.
func TestUpstreamViewWithoutRetentionKeepsLocalState(t *testing.T) {
	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "vc.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	ctx := context.Background()
	if err := ledger.UpsertVolume(ctx, Volume{ID: "vol-a", Format: WorkspaceVolumeFormat, Availability: "available", Persistent: true, Locked: true, UsedBytes: 42, FileCount: 3}); err != nil {
		t.Fatal(err)
	}
	var fresh Volume
	if err := json.Unmarshal([]byte(`{"id":"vol-a","size_bytes":10,"quota_mb":2048}`), &fresh); err != nil {
		t.Fatal(err)
	}
	if fresh.RetentionReported || fresh.UsageReported {
		t.Fatalf("absent fields flagged as reported: %+v", fresh)
	}
	tracked, err := TrackUpstreamVolume(ctx, ledger, fresh)
	if err != nil || !tracked.Persistent || !tracked.Locked || tracked.UsedBytes != 42 {
		t.Fatalf("tracked = %+v, %v", tracked, err)
	}
	stored, _, _ := ledger.GetVolume(ctx, "vol-a")
	if !stored.Persistent || !stored.Locked || stored.FileCount != 3 {
		t.Fatalf("ledger lost retention: %+v", stored)
	}
	// An explicit upstream false still wins.
	if err := json.Unmarshal([]byte(`{"id":"vol-a","persistent":false,"locked":false}`), &fresh); err != nil {
		t.Fatal(err)
	}
	if tracked, _ = TrackUpstreamVolume(ctx, ledger, fresh); tracked.Persistent || tracked.Locked {
		t.Fatalf("reported false ignored: %+v", tracked)
	}
}

// 6: a transient GetMachine failure must not mark an adopted workspace lost.
func TestReconcileKeepsAdoptedWorkspaceOnTransientMachineError(t *testing.T) {
	f := newAdoptFixture(t, adoptMachineJSON(true, time.Time{}))
	ctx := context.Background()
	workspace, _, err := f.manager.AdoptMachine(ctx, f.cfg, f.admin, "vm-9")
	if err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.machineErr = true
	f.mu.Unlock()
	if err := f.manager.Reconcile(ctx, f.cfg); err != nil {
		t.Fatal(err)
	}
	stored, _, _ := f.ledger.GetWorkspace(ctx, workspace.ID)
	if stored.State != WorkspaceStateReady {
		t.Fatalf("state = %s, want ready", stored.State)
	}
}

// 7: a release whose input revoke fails must not close the workspace.
func TestReleaseAdoptedWorkspaceRefusesWhenGUIRevokeFails(t *testing.T) {
	f := newAdoptFixture(t, adoptMachineJSON(true, time.Time{}))
	ctx := context.Background()
	workspace, _, err := f.manager.AdoptMachine(ctx, f.cfg, f.admin, "vm-9")
	if err != nil {
		t.Fatal(err)
	}
	workspace.Capabilities = append(workspace.Capabilities, "desktop.control.v1")
	if err := f.ledger.UpsertWorkspace(ctx, workspace); err != nil {
		t.Fatal(err)
	}
	f.transport.mu.Lock()
	f.transport.failMethod = "desktop.control"
	f.transport.mu.Unlock()
	err = f.manager.CloseWorkspace(ctx, f.cfg, f.admin, workspace.ID)
	if rpc, ok := err.(WorkspaceRPCError); !ok || rpc.Code != "desktop_control_unconfirmed" {
		t.Fatalf("release err = %v", err)
	}
	stored, _, _ := f.ledger.GetWorkspace(ctx, workspace.ID)
	if stored.State != WorkspaceStateReady || stored.LastError == "" {
		t.Fatalf("workspace = state %s lastError %q", stored.State, stored.LastError)
	}
	if f.seen("DELETE") != 0 {
		t.Fatalf("machine destroyed: %v", f.requests)
	}
}

// 10: the legacy delete_volume refuses a volume an active workspace still uses.
func TestLegacyDeleteVolumeRefusesInUse(t *testing.T) {
	prev := DefaultWorkspaceManager()
	SetDefaultWorkspaceManager(nil)
	t.Cleanup(func() { SetDefaultWorkspaceManager(prev) })
	upstream, server := newReviewUpstream(t)
	path := filepath.Join(t.TempDir(), "vc.db")
	ledger, err := OpenLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	if err := ledger.UpsertVolume(ctx, Volume{ID: "vol-busy", Format: WorkspaceVolumeFormat, Availability: "available"}); err != nil {
		t.Fatal(err)
	}
	if err := ledger.UpsertWorkspace(ctx, Workspace{ID: "ws-busy", OwnerSessionID: "s", MachineID: "vm-1", State: WorkspaceStateReady, Template: "desktop",
		VolumeID: "vol-busy", LeaseExpiresAt: now.Add(time.Hour), MaxExpiresAt: now.Add(2 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	_ = ledger.Close()
	cfg := ToolConfig{Enabled: true, ToolGate: true, AllowVolumes: true, BoringdURL: server.URL, LedgerPath: path}
	out := toolResultMap(t, ExecuteTool(ctx, cfg, map[string]interface{}{"operation": "delete_volume", "volume_id": "vol-busy"}))
	if out["code"] != "volume_in_use" || upstream.count("DELETE") != 0 {
		t.Fatalf("delete_volume = %v requests=%v", out, upstream.requests)
	}
}
