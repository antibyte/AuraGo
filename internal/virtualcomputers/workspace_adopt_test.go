package virtualcomputers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

type adoptRecordingTransport struct {
	mu          sync.Mutex
	methods     []string
	handshakeOK bool
}

func (t *adoptRecordingTransport) Call(_ context.Context, machineID, method string, _ interface{}, result interface{}) error {
	t.mu.Lock()
	t.methods = append(t.methods, method)
	ok := t.handshakeOK
	t.mu.Unlock()
	if method == "system.capabilities" {
		if !ok {
			return errors.New("guest agent missing")
		}
		*(result.(*WorkspaceCapabilities)) = WorkspaceCapabilities{
			ProtocolVersion: WorkspaceProtocolVersion, MachineID: machineID, InstanceNonce: "nonce-adopt",
			Capabilities: []string{"shell.exec", "browser.headful"}, MaxMessageBytes: workspaceMaxWireMessageBytes,
		}
	}
	return nil
}

func (t *adoptRecordingTransport) called(method string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for _, m := range t.methods {
		if m == method {
			n++
		}
	}
	return n
}

type adoptFixture struct {
	manager   *WorkspaceManager
	ledger    *Ledger
	transport *adoptRecordingTransport
	cfg       ToolConfig
	admin     WorkspaceIdentity

	mu         sync.Mutex
	requests   []string
	machine    string // JSON returned for GET /v1/machines/vm-9; empty means 404 not found
	machineMux int
}

func (f *adoptFixture) seen(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.requests {
		if strings.HasPrefix(r, prefix) {
			n++
		}
	}
	return n
}

func (f *adoptFixture) setMachine(js string) {
	f.mu.Lock()
	f.machine = js
	f.mu.Unlock()
}

func newAdoptFixture(t *testing.T, machineJSON string) *adoptFixture {
	t.Helper()
	f := &adoptFixture{machine: machineJSON, transport: &adoptRecordingTransport{handshakeOK: true}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, r.Method+" "+r.URL.Path)
		machine := f.machine
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspace/capabilities":
			_ = json.NewEncoder(w).Encode(WorkspaceControlPlaneStatus{
				ProtocolVersion: WorkspaceProtocolVersion, AssetFingerprint: WorkspaceAssetFingerprint(),
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/machines/vm-9":
			if machine == "" {
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `{"error":"not found"}`)
				return
			}
			_, _ = io.WriteString(w, machine)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines":
			_, _ = io.WriteString(w, machine)
		default:
			// DELETE and extend land here; the tests assert they were never seen.
			_, _ = io.WriteString(w, `{"id":"vm-9"}`)
		}
	}))
	t.Cleanup(server.Close)
	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "vc.db"))
	if err != nil {
		t.Fatalf("OpenLedger: %v", err)
	}
	t.Cleanup(func() { _ = ledger.Close() })
	manager, err := NewWorkspaceManager(ledger, slog.Default(), WorkspaceManagerOptions{
		ClientFactory: func(ToolConfig) (*Client, error) {
			return NewClient(ClientConfig{BaseURL: server.URL, Timeout: 2 * time.Second})
		},
		TransportFactory:  func(*Client) WorkspaceTransport { return f.transport },
		ReconcileInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("NewWorkspaceManager: %v", err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	f.manager, f.ledger = manager, ledger
	f.cfg = ToolConfig{
		Enabled: true, ToolGate: true, AllowInternet: true, LedgerPath: "",
		AgentControl: AgentControlConfig{
			Enabled: true, DefaultTemplate: "desktop", MaxActiveWorkspaces: 2,
			IdleTTLSeconds: 600, MaxWorkspaceSeconds: 7200, MaxJobSeconds: 3600,
			MaxJobOutputBytes: 4 << 20, JobsPerWorkspace: 2, BrowserSessionsPerWorkspace: 1,
			NetworkProfile: "internet_lan",
		},
	}
	f.admin = WorkspaceIdentity{SessionID: "desktop-admin", Actor: "user", Admin: true}
	manager.rememberConfig(f.cfg)
	return f
}

func adoptMachineJSON(persistent bool, expires time.Time) string {
	exp := ""
	if !persistent {
		exp = fmt.Sprintf(`,"expires_at":%q`, expires.UTC().Format(time.RFC3339))
	}
	return fmt.Sprintf(`{"id":"vm-9","template":"desktop","display":true,"status":"running","persistent":%v%s}`, persistent, exp)
}

func TestAdoptMachineCreatesAdoptedWorkspace(t *testing.T) {
	t.Parallel()
	expires := time.Now().Add(40 * time.Minute).UTC().Truncate(time.Second)
	f := newAdoptFixture(t, adoptMachineJSON(false, expires))
	ctx := context.Background()

	if _, _, err := f.manager.AdoptMachine(ctx, f.cfg, WorkspaceIdentity{SessionID: "x", Actor: "user"}, "vm-9"); err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("non-admin adopt error = %v, want forbidden", err)
	}
	workspace, reused, err := f.manager.AdoptMachine(ctx, f.cfg, f.admin, "vm-9")
	if err != nil || reused {
		t.Fatalf("AdoptMachine reused=%v err=%v", reused, err)
	}
	if workspace.Origin != WorkspaceOriginAdopted || workspace.OwnerSessionID != AdoptedWorkspaceOwnerSessionID ||
		workspace.MissionID != "" || workspace.NetworkProfile != WorkspaceNetworkProfileMachine || workspace.State != WorkspaceStateReady {
		t.Fatalf("workspace = %+v", workspace)
	}
	if !workspace.MaxExpiresAt.Equal(expires) {
		t.Fatalf("MaxExpiresAt = %v, want machine expiry %v", workspace.MaxExpiresAt, expires)
	}
	if workspace.InstanceNonce != "nonce-adopt" || workspace.Template != "desktop" {
		t.Fatalf("handshake data missing: %+v", workspace)
	}
	stored, ok, _ := f.ledger.GetWorkspace(ctx, workspace.ID)
	if !ok || stored.Origin != WorkspaceOriginAdopted {
		t.Fatalf("stored workspace = %+v ok=%v", stored, ok)
	}

	// Persistent machine: far-future hard expiry, not the zero time.
	f2 := newAdoptFixture(t, adoptMachineJSON(true, time.Time{}))
	persistent, _, err := f2.manager.AdoptMachine(ctx, f2.cfg, f2.admin, "vm-9")
	if err != nil {
		t.Fatalf("adopt persistent: %v", err)
	}
	if persistent.MaxExpiresAt.Before(time.Now().AddDate(50, 0, 0)) {
		t.Fatalf("persistent MaxExpiresAt = %v", persistent.MaxExpiresAt)
	}

	// Limit.
	f3 := newAdoptFixture(t, adoptMachineJSON(true, time.Time{}))
	f3.cfg.AgentControl.MaxActiveWorkspaces = 0
	if _, _, err := f3.manager.AdoptMachine(ctx, f3.cfg, f3.admin, "vm-9"); err == nil || !strings.Contains(err.Error(), "workspace_limit_reached") {
		t.Fatalf("limit error = %v", err)
	}
}

func TestAdoptMachineReusesActiveAndReopensClosedRow(t *testing.T) {
	t.Parallel()
	f := newAdoptFixture(t, adoptMachineJSON(true, time.Time{}))
	ctx := context.Background()
	first, _, err := f.manager.AdoptMachine(ctx, f.cfg, f.admin, "vm-9")
	if err != nil {
		t.Fatalf("first adopt: %v", err)
	}
	again, reused, err := f.manager.AdoptMachine(ctx, f.cfg, f.admin, "vm-9")
	if err != nil || !reused || again.ID != first.ID {
		t.Fatalf("second adopt id=%s reused=%v err=%v", again.ID, reused, err)
	}
	// A GUI gate left behind by the closed row must not survive the reopen.
	f.manager.guiFor(first.ID).mu.Lock()
	f.manager.guiFor(first.ID).closed = true
	f.manager.guiFor(first.ID).mu.Unlock()
	if err := f.manager.CloseWorkspace(ctx, f.cfg, f.admin, first.ID); err != nil {
		t.Fatalf("release: %v", err)
	}
	reopened, reused, err := f.manager.AdoptMachine(ctx, f.cfg, f.admin, "vm-9")
	if err != nil || reused || reopened.ID != first.ID || reopened.State != WorkspaceStateReady {
		t.Fatalf("reopen id=%s state=%s reused=%v err=%v", reopened.ID, reopened.State, reused, err)
	}
	f.manager.guiMu.Lock()
	_, kept := f.manager.gui[first.ID]
	f.manager.guiMu.Unlock()
	if kept {
		t.Fatal("reopen kept the stale GUI gate")
	}

	// A launched workspace owns another machine: refuse.
	now := time.Now().UTC()
	if err := f.ledger.UpsertWorkspace(ctx, Workspace{
		ID: "ws-launched", OwnerSessionID: "s1", MachineID: "vm-launched", State: WorkspaceStateReady, Template: "desktop",
		CreatedAt: now, UpdatedAt: now, LastActivityAt: now, LeaseExpiresAt: now.Add(time.Hour), MaxExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	f.setMachine(`{"id":"vm-9","template":"desktop","display":true,"status":"running","persistent":true}`)
	if _, _, err := f.manager.AdoptMachine(ctx, f.cfg, f.admin, "vm-launched"); err == nil || !strings.Contains(err.Error(), "machine_owned_by_workspace") {
		t.Fatalf("owned machine error = %v", err)
	}
}

func TestAdoptMachineHandshakeFailureMapsToUpgradeRequired(t *testing.T) {
	t.Parallel()
	f := newAdoptFixture(t, adoptMachineJSON(true, time.Time{}))
	f.transport.handshakeOK = false
	_, _, err := f.manager.AdoptMachine(context.Background(), f.cfg, f.admin, "vm-9")
	var rpc WorkspaceRPCError
	if !errors.As(err, &rpc) || rpc.Code != "workspace_agent_upgrade_required" {
		t.Fatalf("error = %v", err)
	}
	if _, ok, _ := f.ledger.WorkspaceByMachine(context.Background(), "vm-9"); ok {
		t.Fatal("handshake failure left a ledger row")
	}
	if f.seen("DELETE") != 0 {
		t.Fatal("handshake failure destroyed the machine")
	}
	f.setMachine(`{"id":"vm-9","status":"stopped"}`)
	f.transport.handshakeOK = true
	if _, _, err := f.manager.AdoptMachine(context.Background(), f.cfg, f.admin, "vm-9"); err == nil || !strings.Contains(err.Error(), "machine_not_running") {
		t.Fatalf("stopped machine error = %v", err)
	}
}

func TestReleaseAdoptedWorkspaceKeepsMachine(t *testing.T) {
	t.Parallel()
	f := newAdoptFixture(t, adoptMachineJSON(true, time.Time{}))
	ctx := context.Background()
	workspace, _, err := f.manager.AdoptMachine(ctx, f.cfg, f.admin, "vm-9")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := f.ledger.UpsertWorkspaceJob(ctx, WorkspaceJob{ID: "job-r", WorkspaceID: workspace.ID, Mode: JobModeSync, State: JobStateRunning, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := f.ledger.UpsertCredentialGrant(ctx, CredentialGrant{
		ID: "grant-r", WorkspaceID: workspace.ID, CredentialID: "c", UsageType: GrantUsageShell, JobID: "job-r",
		Status: GrantActive, CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.manager.CloseWorkspace(ctx, f.cfg, f.admin, workspace.ID); err != nil {
		t.Fatalf("release: %v", err)
	}
	if f.transport.called("job.cancel") != 1 || f.transport.called("credential.revoke") != 1 {
		t.Fatalf("rpc calls = %v", f.transport.methods)
	}
	if f.transport.called("workspace.close") != 0 {
		t.Fatal("release called the guest workspace.close")
	}
	if f.seen("DELETE") != 0 || f.seen("POST /v1/machines/vm-9/extend") != 0 {
		t.Fatalf("release touched the machine: %v", f.requests)
	}
	stored, _, _ := f.ledger.GetWorkspace(ctx, workspace.ID)
	if stored.State != WorkspaceStateClosed {
		t.Fatalf("state = %s", stored.State)
	}
	job, _, _ := f.ledger.GetWorkspaceJob(ctx, "job-r")
	if job.State == JobStateRunning {
		t.Fatalf("job state = %s", job.State)
	}
	grants, _ := f.ledger.ListCredentialGrants(ctx, workspace.ID)
	if len(grants) != 1 || grants[0].Status != GrantRevoked {
		t.Fatalf("grants = %+v", grants)
	}
	events, _ := f.ledger.ListWorkspaceEvents(ctx, workspace.ID, 0, 50)
	var released bool
	for _, e := range events {
		if e.Type == "workspace_released" {
			released = true
		}
	}
	if !released {
		t.Fatalf("events = %+v", events)
	}
	// Idempotent.
	if err := f.manager.CloseWorkspace(ctx, f.cfg, f.admin, workspace.ID); err != nil {
		t.Fatalf("second release: %v", err)
	}
}

func TestLeaseReaperReleasesAdoptedWorkspace(t *testing.T) {
	t.Parallel()
	f := newAdoptFixture(t, adoptMachineJSON(true, time.Time{}))
	ctx := context.Background()
	workspace, _, err := f.manager.AdoptMachine(ctx, f.cfg, f.admin, "vm-9")
	if err != nil {
		t.Fatal(err)
	}
	workspace.LeaseExpiresAt = time.Now().Add(-time.Second)
	if err := f.ledger.UpsertWorkspace(ctx, workspace); err != nil {
		t.Fatal(err)
	}
	f.manager.reconcileLeases(ctx)
	stored, _, _ := f.ledger.GetWorkspace(ctx, workspace.ID)
	if stored.State != WorkspaceStateClosed {
		t.Fatalf("state = %s, want closed", stored.State)
	}
	if f.seen("DELETE") != 0 || f.transport.called("workspace.close") != 0 {
		t.Fatalf("reaper destroyed the user's machine: %v %v", f.requests, f.transport.methods)
	}
}

func TestReconcileLeasesMarksAdoptedLostWhenMachineGone(t *testing.T) {
	t.Parallel()
	f := newAdoptFixture(t, adoptMachineJSON(true, time.Time{}))
	ctx := context.Background()
	workspace, _, err := f.manager.AdoptMachine(ctx, f.cfg, f.admin, "vm-9")
	if err != nil {
		t.Fatal(err)
	}
	f.setMachine("")
	f.manager.reconcileLeases(ctx)
	stored, _, _ := f.ledger.GetWorkspace(ctx, workspace.ID)
	if stored.State != WorkspaceStateLost {
		t.Fatalf("state = %s, want lost", stored.State)
	}

	// While the machine lives its new expiry is mirrored into the workspace.
	f2 := newAdoptFixture(t, adoptMachineJSON(false, time.Now().Add(20*time.Minute)))
	w2, _, err := f2.manager.AdoptMachine(ctx, f2.cfg, f2.admin, "vm-9")
	if err != nil {
		t.Fatal(err)
	}
	extended := time.Now().Add(50 * time.Minute).UTC().Truncate(time.Second)
	f2.setMachine(adoptMachineJSON(false, extended))
	f2.manager.reconcileLeases(ctx)
	stored, _, _ = f2.ledger.GetWorkspace(ctx, w2.ID)
	if !stored.MaxExpiresAt.Equal(extended) {
		t.Fatalf("MaxExpiresAt = %v, want %v", stored.MaxExpiresAt, extended)
	}
}

func TestTouchDoesNotExtendAdoptedMachine(t *testing.T) {
	t.Parallel()
	f := newAdoptFixture(t, adoptMachineJSON(true, time.Time{}))
	ctx := context.Background()
	workspace, _, err := f.manager.AdoptMachine(ctx, f.cfg, f.admin, "vm-9")
	if err != nil {
		t.Fatal(err)
	}
	client, _ := f.manager.clientFactory(f.cfg)
	if err := f.manager.touch(ctx, f.cfg, client, workspace); err != nil {
		t.Fatalf("touch: %v", err)
	}
	if f.seen("POST /v1/machines/vm-9/extend") != 0 {
		t.Fatalf("touch extended the adopted machine: %v", f.requests)
	}
}

func TestOpenClampsLeaseToMaxWorkspaceAndMachineExpiry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	owner := WorkspaceIdentity{SessionID: "s", Actor: "agent"}

	f := newAdoptFixture(t, `{"id":"vm-9","template":"desktop","display":true,"status":"running"}`)
	f.cfg.AgentControl.MaxWorkspaceSeconds = 120
	w, err := f.manager.Open(ctx, f.cfg, owner, WorkspaceOpenRequest{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if w.LeaseExpiresAt.After(w.MaxExpiresAt) {
		t.Fatalf("lease %v beyond max %v", w.LeaseExpiresAt, w.MaxExpiresAt)
	}

	soon := time.Now().Add(90 * time.Second).UTC().Truncate(time.Second)
	f2 := newAdoptFixture(t, adoptMachineJSON(false, soon))
	w2, err := f2.manager.Open(ctx, f2.cfg, owner, WorkspaceOpenRequest{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if w2.LeaseExpiresAt.After(soon) {
		t.Fatalf("lease %v beyond machine expiry %v", w2.LeaseExpiresAt, soon)
	}
}

func TestExecuteToolRefusesWorkspaceOwnedMachine(t *testing.T) {
	f := newAdoptFixture(t, adoptMachineJSON(true, time.Time{}))
	ctx := context.Background()
	prev := DefaultWorkspaceManager()
	SetDefaultWorkspaceManager(nil)
	t.Cleanup(func() { SetDefaultWorkspaceManager(prev) })
	workspace, _, err := f.manager.AdoptMachine(ctx, f.cfg, f.admin, "vm-9")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, "TOOL "+r.Method+" "+r.URL.Path)
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"vm-9","status":"running"}`)
	}))
	defer server.Close()
	cfg := ToolConfig{Enabled: true, ToolGate: true, BoringdURL: server.URL, LedgerPath: f.ledgerPath(t), AllowAgentTasks: true, AllowVolumes: true}
	for _, op := range []string{"exec", "destroy", "extend", "fork", "save_machine", "screenshot"} {
		out := ExecuteTool(ctx, cfg, map[string]interface{}{"operation": op, "machine_id": "vm-9", "command": "id"})
		if !strings.Contains(out, "machine_owned_by_workspace") || !strings.Contains(out, workspace.ID) {
			t.Fatalf("%s result = %s", op, out)
		}
	}
	if f.seen("TOOL") != 0 {
		t.Fatalf("refused operations reached boringd: %v", f.requests)
	}
	if out := ExecuteTool(ctx, cfg, map[string]interface{}{"operation": "get", "machine_id": "vm-9"}); strings.Contains(out, "machine_owned_by_workspace") || f.seen("TOOL GET /v1/machines/vm-9") != 1 {
		t.Fatalf("get result = %s requests=%v", out, f.requests)
	}
}

func (f *adoptFixture) ledgerPath(t *testing.T) string {
	t.Helper()
	var path string
	if err := f.ledger.db.QueryRow(`SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&path); err != nil {
		t.Fatalf("ledger path: %v", err)
	}
	return path
}

func TestExecuteToolForkDefaultsMaxWhenUnset(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	}))
	defer server.Close()
	cfg := ToolConfig{Enabled: true, ToolGate: true, BoringdURL: server.URL, MaxForks: 0}
	out := ExecuteTool(context.Background(), cfg, map[string]interface{}{"operation": "fork", "machine_id": "vm-1", "count": 4})
	if !strings.Contains(out, "invalid_argument") || calls != 0 {
		t.Fatalf("fork with unset limit = %s calls=%d", out, calls)
	}
}
