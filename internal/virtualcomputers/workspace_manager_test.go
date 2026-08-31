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

	"aurago/internal/security"
)

type workspaceTestTransport struct {
	mu              sync.Mutex
	activatedValues map[string]string
	maxOutputBytes  int64
	jobState        string
}

func (t *workspaceTestTransport) Call(_ context.Context, machineID, method string, params interface{}, result interface{}) error {
	switch method {
	case "system.capabilities":
		capabilities := result.(*WorkspaceCapabilities)
		*capabilities = WorkspaceCapabilities{
			ProtocolVersion: WorkspaceProtocolVersion, MachineID: machineID, InstanceNonce: "nonce-1",
			Capabilities: []string{"shell.exec", "job.pty", "browser.headful"}, MaxMessageBytes: workspaceMaxWireMessageBytes,
		}
	case "job.start":
		encoded, _ := json.Marshal(params)
		var request WorkspaceStartJobRequest
		_ = json.Unmarshal(encoded, &request)
		t.mu.Lock()
		t.maxOutputBytes = request.MaxOutputBytes
		t.mu.Unlock()
		job := result.(*WorkspaceJob)
		t.mu.Lock()
		state := t.jobState
		t.mu.Unlock()
		if state == "" {
			state = JobStateQueued
		}
		*job = WorkspaceJob{ID: "job-1", State: state, CreatedAt: time.Now().UTC()}
	case "job.status":
		t.mu.Lock()
		state := t.jobState
		t.mu.Unlock()
		if state == "" {
			state = JobStateQueued
		}
		job := result.(*WorkspaceJob)
		*job = WorkspaceJob{ID: "job-1", State: state, CreatedAt: time.Now().UTC()}
	case "credential.activate":
		encoded, _ := json.Marshal(params)
		var request struct {
			Values map[string]string `json:"values"`
		}
		_ = json.Unmarshal(encoded, &request)
		t.mu.Lock()
		t.activatedValues = request.Values
		t.mu.Unlock()
	case "job.cancel", "credential.revoke", "workspace.close":
		return nil
	}
	return nil
}

type workspaceTestCredentials struct{ resolveCalls int }

func (p *workspaceTestCredentials) ListGrantable(context.Context) ([]GrantableCredential, error) {
	return []GrantableCredential{{ID: "cred-1", Name: "registry", Type: "login", AvailableFields: []string{"username", "password"}}}, nil
}

func (p *workspaceTestCredentials) Resolve(context.Context, string, []string) (map[string]string, error) {
	p.resolveCalls++
	return map[string]string{"username": "agent", "password": "secret-value"}, nil
}

func TestWorkspaceManagerBindsOwnershipAndDefersCredentialResolution(t *testing.T) {
	t.Parallel()
	var launch launchMachineRequest
	var extendCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspace/capabilities":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(WorkspaceControlPlaneStatus{
				ProtocolVersion: WorkspaceProtocolVersion, AssetFingerprint: WorkspaceAssetFingerprint(),
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines":
			if err := json.NewDecoder(r.Body).Decode(&launch); err != nil {
				t.Errorf("decode launch: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"vm-1","template":"desktop","display":true}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/machines/vm-1":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/machines/vm-1/extend":
			extendCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"vm-1","template":"desktop","display":true}`)
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
	transport := &workspaceTestTransport{}
	credentials := &workspaceTestCredentials{}
	manager, err := NewWorkspaceManager(ledger, slog.Default(), WorkspaceManagerOptions{
		CredentialProvider: credentials,
		ClientFactory: func(ToolConfig) (*Client, error) {
			return NewClient(ClientConfig{BaseURL: server.URL, Timeout: time.Second})
		},
		TransportFactory:  func(*Client) WorkspaceTransport { return transport },
		ReconcileInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("NewWorkspaceManager: %v", err)
	}
	defer manager.Close()

	cfg := ToolConfig{
		Enabled: true, ToolGate: true, AllowInternet: true,
		AgentControl: AgentControlConfig{
			Enabled: true, DefaultTemplate: "desktop", MaxActiveWorkspaces: 2,
			IdleTTLSeconds: 600, MaxWorkspaceSeconds: 7200, MaxJobSeconds: 3600,
			MaxJobOutputBytes: 4 << 20, JobsPerWorkspace: 2, BrowserSessionsPerWorkspace: 1,
			NetworkProfile: "internet_lan", AllowedPrivateCIDRs: []string{"192.168.50.0/24"},
			CredentialsEnabled: true, CredentialGrantTTLSeconds: 900,
		},
	}
	owner := WorkspaceIdentity{SessionID: "session-a", MissionID: "mission-a", Actor: "main-agent"}
	workspace, err := manager.Open(context.Background(), cfg, owner, WorkspaceOpenRequest{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if launch.VolumeFormat != WorkspaceVolumeFormat || launch.NetworkProfile != "internet_lan" {
		t.Fatalf("launch policy = %+v", launch)
	}
	if len(launch.ProtectedCIDRs) == 0 {
		t.Fatal("launch omitted AuraGo protected addresses")
	}
	if _, err := manager.Get(context.Background(), WorkspaceIdentity{SessionID: "session-b", MissionID: "mission-a"}, workspace.ID); err == nil {
		t.Fatal("foreign session unexpectedly read workspace")
	}
	if _, err := manager.Get(context.Background(), WorkspaceIdentity{SessionID: "session-a", MissionID: "mission-b"}, workspace.ID); err == nil {
		t.Fatal("foreign mission unexpectedly read workspace")
	}
	admin := WorkspaceIdentity{SessionID: "desktop-admin", Actor: "user", Admin: true}
	if _, err := manager.TakeControl(context.Background(), admin, workspace.ID, true); err != nil {
		t.Fatalf("TakeControl: %v", err)
	}
	if _, err := manager.BrowserAction(context.Background(), cfg, owner, workspace.ID, BrowserActionRequest{Operation: "inspect"}); err != nil {
		t.Fatalf("read-only browser inspection during human takeover: %v", err)
	}
	if _, err := manager.BrowserAction(context.Background(), cfg, owner, workspace.ID, BrowserActionRequest{Operation: "click", Selector: "button"}); err == nil {
		t.Fatal("agent browser input unexpectedly bypassed human takeover")
	}
	if _, err := manager.TakeControl(context.Background(), admin, workspace.ID, false); err != nil {
		t.Fatalf("release control: %v", err)
	}

	job, err := manager.StartJob(context.Background(), cfg, owner, workspace.ID, WorkspaceStartJobRequest{
		Command: "private-command", WaitForCredentialGrant: true,
	})
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	transport.mu.Lock()
	configuredOutputBytes := transport.maxOutputBytes
	transport.mu.Unlock()
	if configuredOutputBytes != cfg.AgentControl.MaxJobOutputBytes {
		t.Fatalf("guest output cap = %d, want %d", configuredOutputBytes, cfg.AgentControl.MaxJobOutputBytes)
	}
	storedWorkspace, ok, err := ledger.GetWorkspace(context.Background(), workspace.ID)
	if err != nil || !ok {
		t.Fatalf("load workspace before lease test: ok=%v err=%v", ok, err)
	}
	storedWorkspace.LeaseExpiresAt = time.Now().Add(-time.Second)
	if err := ledger.UpsertWorkspace(context.Background(), storedWorkspace); err != nil {
		t.Fatalf("expire workspace lease: %v", err)
	}
	extendsBeforeReconcile := extendCalls
	manager.reconcileLeases(context.Background())
	if extendCalls <= extendsBeforeReconcile {
		t.Fatal("active queued job did not automatically extend the workspace lease")
	}
	storedWorkspace, ok, err = ledger.GetWorkspace(context.Background(), workspace.ID)
	if err != nil || !ok || storedWorkspace.State != WorkspaceStateReady || !storedWorkspace.LeaseExpiresAt.After(time.Now()) {
		t.Fatalf("workspace was not retained for active job: %+v ok=%v err=%v", storedWorkspace, ok, err)
	}
	if _, err := manager.RequestCredentialGrant(context.Background(), cfg, owner, workspace.ID, CredentialGrantRequest{
		CredentialID: "cred-1", UsageType: GrantUsageShell, JobID: job.ID, FieldNames: []string{"not-a-field"},
	}); err == nil {
		t.Fatal("invalid credential fields unexpectedly fell back to every available secret field")
	}
	grant, err := manager.RequestCredentialGrant(context.Background(), cfg, owner, workspace.ID, CredentialGrantRequest{
		CredentialID: "cred-1", UsageType: GrantUsageShell, JobID: job.ID, Purpose: "registry login",
	})
	if err != nil {
		t.Fatalf("RequestCredentialGrant: %v", err)
	}
	if credentials.resolveCalls != 0 {
		t.Fatalf("credential resolved before approval: %d calls", credentials.resolveCalls)
	}
	if _, err := manager.ApproveCredentialGrant(context.Background(), cfg, owner, grant.ID); err == nil {
		t.Fatal("agent identity unexpectedly approved its own credential grant")
	}
	approved, err := manager.ApproveCredentialGrant(context.Background(), cfg, WorkspaceIdentity{SessionID: "desktop-admin", Actor: "user", Admin: true}, grant.ID)
	if err != nil {
		t.Fatalf("ApproveCredentialGrant: %v", err)
	}
	if approved.Status != GrantActive || credentials.resolveCalls != 1 {
		t.Fatalf("approved grant = %+v, resolve calls = %d", approved, credentials.resolveCalls)
	}
	transport.mu.Lock()
	password := transport.activatedValues["password"]
	transport.mu.Unlock()
	if password != "secret-value" {
		t.Fatal("approved shell credential was not delivered to the guest job")
	}
	transport.mu.Lock()
	transport.jobState = JobStateCompleted
	transport.mu.Unlock()
	manager.reconcileLeases(context.Background())
	completedGrant, ok, err := ledger.GetCredentialGrant(context.Background(), grant.ID)
	if err != nil || !ok || completedGrant.Status != GrantConsumed {
		t.Fatalf("completed job grant was not consumed: %+v ok=%v err=%v", completedGrant, ok, err)
	}
	completedJob, ok, err := ledger.GetWorkspaceJob(context.Background(), job.ID)
	if err != nil || !ok || completedJob.State != JobStateCompleted {
		t.Fatalf("completed guest job was not reconciled: %+v ok=%v err=%v", completedJob, ok, err)
	}
}

func TestBrowserHumanControlOnlyPausesMutatingActions(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"inspect", "list_tabs", "screenshot", "list_downloads", "wait"} {
		if browserActionRequiresControl(operation) {
			t.Fatalf("read-only browser operation %q unexpectedly requires control", operation)
		}
	}
	for _, operation := range []string{"navigate", "click", "type", "select", "press", "switch_tab", "upload_file", "close"} {
		if !browserActionRequiresControl(operation) {
			t.Fatalf("mutating browser operation %q unexpectedly bypasses human control", operation)
		}
	}
}

func TestWorkspaceAuditCommandRedactsInlineSecrets(t *testing.T) {
	t.Parallel()
	_, summary := auditCommand(`curl -H "Authorization: Bearer top-secret-token" https://example.test`)
	if strings.Contains(summary, "top-secret-token") {
		t.Fatalf("audit summary exposed inline credential: %q", summary)
	}
}

func TestWorkspaceToolResultScrubsRegisteredCredentialValues(t *testing.T) {
	t.Parallel()
	const secret = "workspace-tool-secret-71f914a2"
	security.RegisterSensitive(secret)
	result := workspaceToolResult("done", map[string]string{"output": "value=" + secret}, nil)
	if strings.Contains(result, secret) {
		t.Fatalf("workspace tool output exposed registered credential: %s", result)
	}
}
