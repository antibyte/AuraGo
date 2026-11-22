package server

import (
	"bytes"
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

	"aurago/internal/virtualcomputers"
)

// fakeMachinesBoringd records every request and answers the machine routes.
type fakeMachinesBoringd struct {
	mu        sync.Mutex
	requests  []string
	bodies    map[string]string
	machines  string // JSON array returned by GET /v1/machines; "" means HTTP 500
	templates string // JSON returned by GET /v1/templates
}

func newFakeMachinesBoringd(t *testing.T) (*fakeMachinesBoringd, *httptest.Server) {
	t.Helper()
	fake := &fakeMachinesBoringd{bodies: map[string]string{}, machines: `[]`, templates: `[]`}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		fake.mu.Lock()
		key := r.Method + " " + r.URL.Path
		fake.requests = append(fake.requests, key)
		fake.bodies[key] = string(data)
		machines, templates := fake.machines, fake.templates
		fake.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v1/workspace/capabilities":
			_ = json.NewEncoder(w).Encode(virtualcomputers.WorkspaceControlPlaneStatus{
				ProtocolVersion: virtualcomputers.WorkspaceProtocolVersion, AssetFingerprint: virtualcomputers.WorkspaceAssetFingerprint(),
			})
		case key == "GET /v1/machines":
			if machines == "" {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = io.WriteString(w, `{"error":"boom"}`)
				return
			}
			_, _ = io.WriteString(w, machines)
		case key == "GET /v1/templates":
			_, _ = io.WriteString(w, templates)
		case key == "POST /v1/machines":
			_, _ = io.WriteString(w, `{"id":"vm-new","template":"desktop","display":true,"status":"running"}`)
		case key == "GET /v1/machines/vm-1":
			_, _ = io.WriteString(w, `{"id":"vm-1","template":"desktop","display":true,"status":"running","persistent":true}`)
		case r.URL.Path == "/v1/machines/vm-1/extend":
			_, _ = io.WriteString(w, `{"id":"vm-1","template":"desktop","status":"running","expires_at":"`+time.Now().Add(30*time.Minute).UTC().Format(time.RFC3339)+`"}`)
		case r.URL.Path == "/v1/machines/vm-1/download":
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, `<script>alert(1)</script>`)
		default:
			_, _ = io.WriteString(w, `{"id":"vm-1"}`)
		}
	}))
	t.Cleanup(server.Close)
	return fake, server
}

func (f *fakeMachinesBoringd) count(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, request := range f.requests {
		if strings.HasPrefix(request, prefix) {
			n++
		}
	}
	return n
}

func (f *fakeMachinesBoringd) body(key string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bodies[key]
}

func newMachineTestServer(t *testing.T, upstreamURL string) (*Server, *virtualcomputers.Ledger, *http.ServeMux) {
	t.Helper()
	s, ledger, mux := newVolumeTestServer(t, upstreamURL)
	s.Cfg.VirtualComputers.AllowPersistent = false
	s.Cfg.VirtualComputers.MaxTTLSeconds = 900
	s.Cfg.VirtualComputers.DefaultTTLSeconds = 600
	return s, ledger, mux
}

func machineTestRequest(mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, reader))
	return rec
}

func TestVirtualComputersMachineLaunchValidation(t *testing.T) {
	fake, upstream := newFakeMachinesBoringd(t)
	s, ledger, mux := newMachineTestServer(t, upstream.URL)
	ctx := context.Background()
	for _, volume := range []virtualcomputers.Volume{
		{ID: "vol-ws", Format: virtualcomputers.WorkspaceVolumeFormat, Availability: "available"},
		{ID: "vol-root", Format: "legacy_root", Availability: "available"},
		{ID: "vol-old", Format: "legacy_root", Availability: "previous_store"},
	} {
		if err := ledger.UpsertVolume(ctx, volume); err != nil {
			t.Fatal(err)
		}
	}
	fake.templates = `[{"id":"desktop"},{"id":"python"}]`
	cases := []struct {
		name, body, code string
		status           int
	}{
		{"persistent disabled", `{"template":"desktop","persistent":true}`, "persistent_disabled", http.StatusForbidden},
		{"ttl too long", `{"template":"desktop","ttl_seconds":901}`, "ttl_too_long", http.StatusBadRequest},
		{"unknown template", `{"template":"nope"}`, "invalid_template", http.StatusBadRequest},
		{"bad label", "{\"template\":\"desktop\",\"name\":\"a\\u0007b\"}", "invalid_name", http.StatusBadRequest},
		{"unknown volume", `{"template":"desktop","volume_id":"vol-missing"}`, "volume_not_found", http.StatusNotFound},
		{"unavailable volume", `{"template":"desktop","volume_id":"vol-old"}`, "volume_unavailable", http.StatusConflict},
	}
	s.Cfg.VirtualComputers.AllowVolumes = true
	for _, tc := range cases {
		rec := machineTestRequest(mux, http.MethodPost, "/api/virtual-computers/machines", tc.body)
		if payload := decodeVolumeTestBody(t, rec); rec.Code != tc.status || payload["code"] != tc.code {
			t.Fatalf("%s: status=%d body=%s", tc.name, rec.Code, rec.Body.String())
		}
	}
	if fake.count("POST /v1/machines") != 0 {
		t.Fatal("rejected launches reached boringd")
	}

	// The ledger decides volume_format; the client-sent value is ignored.
	rec := machineTestRequest(mux, http.MethodPost, "/api/virtual-computers/machines",
		`{"template":"desktop","name":"  Build box ","volume_id":"vol-ws","volume_format":"legacy_root"}`)
	payload := decodeVolumeTestBody(t, rec)
	machine, _ := payload["machine"].(map[string]interface{})
	if rec.Code != http.StatusOK || machine["label"] != "Build box" || machine["volume_id"] != "vol-ws" {
		t.Fatalf("launch status=%d body=%s", rec.Code, rec.Body.String())
	}
	if body := fake.body("POST /v1/machines"); !strings.Contains(body, `"volume_format":"workspace_v2"`) {
		t.Fatalf("upstream launch body = %s", body)
	}
	meta, _ := ledger.MachineMetadata(ctx)
	if meta["vm-new"].Label != "Build box" || meta["vm-new"].VolumeID != "vol-ws" {
		t.Fatalf("machine metadata = %+v", meta["vm-new"])
	}
	rec = machineTestRequest(mux, http.MethodPost, "/api/virtual-computers/machines", `{"template":"desktop","volume_id":"vol-root","volume_format":"workspace_v2"}`)
	if rec.Code != http.StatusOK || strings.Contains(fake.body("POST /v1/machines"), "volume_format") {
		t.Fatalf("legacy volume launch status=%d body=%s upstream=%s", rec.Code, rec.Body.String(), fake.body("POST /v1/machines"))
	}
}

func TestVirtualComputersMachinesListMergesLabelsAndPrunes(t *testing.T) {
	fake, upstream := newFakeMachinesBoringd(t)
	_, ledger, mux := newMachineTestServer(t, upstream.URL)
	ctx := context.Background()
	fake.machines = `[{"id":"vm-1","template":"desktop","status":"running"},{"id":"vm-2","template":"python","status":"running"}]`
	_ = ledger.SetMachineLabel(ctx, "vm-1", "Web")
	_ = ledger.SetMachineVolume(ctx, "vm-1", "vol-a")
	_ = ledger.SetMachineLabel(ctx, "vm-gone", "Old")
	now := time.Now().UTC()
	if err := ledger.UpsertWorkspace(ctx, virtualcomputers.Workspace{
		ID: "ws-1", OwnerSessionID: virtualcomputers.AdoptedWorkspaceOwnerSessionID, MachineID: "vm-1", State: virtualcomputers.WorkspaceStateReady,
		Origin: virtualcomputers.WorkspaceOriginAdopted, LeaseExpiresAt: now.Add(time.Hour), MaxExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	rec := machineTestRequest(mux, http.MethodGet, "/api/virtual-computers/machines", "")
	var body struct {
		Machines []map[string]interface{} `json:"machines"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || len(body.Machines) != 2 {
		t.Fatalf("list status=%d body=%s", rec.Code, rec.Body.String())
	}
	first := body.Machines[0]
	if first["label"] != "Web" || first["volume_id"] != "vol-a" || first["workspace_id"] != "ws-1" || first["workspace_origin"] != "adopted" {
		t.Fatalf("merged machine = %v", first)
	}
	if _, ok := body.Machines[1]["workspace_id"]; ok {
		t.Fatalf("unowned machine carries a workspace: %v", body.Machines[1])
	}
	meta, _ := ledger.MachineMetadata(ctx)
	if _, stale := meta["vm-gone"]; stale || meta["vm-1"].Label != "Web" {
		t.Fatalf("metadata after prune = %+v", meta)
	}

	// A failed list must not prune anything.
	_ = ledger.SetMachineLabel(ctx, "vm-keep", "Keep")
	fake.mu.Lock()
	fake.machines = ""
	fake.mu.Unlock()
	if rec := machineTestRequest(mux, http.MethodGet, "/api/virtual-computers/machines", ""); rec.Code < 500 {
		t.Fatalf("failed list status = %d", rec.Code)
	}
	if meta, _ := ledger.MachineMetadata(ctx); meta["vm-keep"].Label != "Keep" {
		t.Fatal("a failed list pruned the ledger")
	}
}

func TestVirtualComputersMachinePatchRenames(t *testing.T) {
	_, upstream := newFakeMachinesBoringd(t)
	_, ledger, mux := newMachineTestServer(t, upstream.URL)
	rec := machineTestRequest(mux, http.MethodPatch, "/api/virtual-computers/machines/vm-1", `{"name":" Alpha "}`)
	payload := decodeVolumeTestBody(t, rec)
	if rec.Code != http.StatusOK || payload["label"] != "Alpha" || payload["machine_id"] != "vm-1" {
		t.Fatalf("rename status=%d body=%s", rec.Code, rec.Body.String())
	}
	if meta, _ := ledger.MachineMetadata(context.Background()); meta["vm-1"].Label != "Alpha" {
		t.Fatalf("metadata = %+v", meta)
	}
	rec = machineTestRequest(mux, http.MethodPatch, "/api/virtual-computers/machines/vm-1", `{"name":"`+strings.Repeat("x", 65)+`"}`)
	if payload := decodeVolumeTestBody(t, rec); rec.Code != http.StatusBadRequest || payload["code"] != "invalid_name" {
		t.Fatalf("long name status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestVirtualComputersMachineDeleteRefusesWorkspaceOwned(t *testing.T) {
	fake, upstream := newFakeMachinesBoringd(t)
	_, ledger, mux := newMachineTestServer(t, upstream.URL)
	now := time.Now().UTC()
	if err := ledger.UpsertWorkspace(context.Background(), virtualcomputers.Workspace{
		ID: "ws-9", OwnerSessionID: "s", MachineID: "vm-1", State: virtualcomputers.WorkspaceStateReady,
		LeaseExpiresAt: now.Add(time.Hour), MaxExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	rec := machineTestRequest(mux, http.MethodDelete, "/api/virtual-computers/machines/vm-1", "")
	payload := decodeVolumeTestBody(t, rec)
	if rec.Code != http.StatusConflict || payload["code"] != "machine_owned_by_workspace" || payload["workspace_id"] != "ws-9" || payload["origin"] != "launched" {
		t.Fatalf("owned delete status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.count("DELETE") != 0 {
		t.Fatal("refused destroy reached boringd")
	}
	if rec := machineTestRequest(mux, http.MethodDelete, "/api/virtual-computers/machines/vm-free", ""); rec.Code != http.StatusOK || fake.count("DELETE /v1/machines/vm-free") != 1 {
		t.Fatalf("free delete status=%d", rec.Code)
	}
}

func TestVirtualComputersMachineDeleteFailsClosedWithoutLedger(t *testing.T) {
	for _, failure := range []string{"missing path", "missing file", "cannot open", "cannot read"} {
		t.Run(failure, func(t *testing.T) {
			fake, upstream := newFakeMachinesBoringd(t)
			s, ledger, mux := newMachineTestServer(t, upstream.URL)
			switch failure {
			case "missing path":
				s.VirtualComputersDB = nil
				s.Cfg.SQLite.VirtualComputersPath = ""
			case "missing file":
				s.VirtualComputersDB = nil
				s.Cfg.SQLite.VirtualComputersPath = filepath.Join(t.TempDir(), "missing.db")
			case "cannot open":
				s.VirtualComputersDB = nil
				s.Cfg.SQLite.VirtualComputersPath = t.TempDir()
			case "cannot read":
				if err := ledger.Close(); err != nil {
					t.Fatal(err)
				}
			}
			rec := machineTestRequest(mux, http.MethodDelete, "/api/virtual-computers/machines/vm-1", "")
			if payload := decodeVolumeTestBody(t, rec); rec.Code != http.StatusServiceUnavailable || payload["code"] != "storage_unavailable" {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if fake.count("DELETE") != 0 {
				t.Fatal("destroy reached boringd without a verified owner lookup")
			}
		})
	}
}

func TestVirtualComputersMachineDownloadForcesAttachment(t *testing.T) {
	_, upstream := newFakeMachinesBoringd(t)
	_, _, mux := newMachineTestServer(t, upstream.URL)
	rec := machineTestRequest(mux, http.MethodGet, "/api/virtual-computers/machines/vm-1/download?path=/tmp/evil%22%3Bx.html", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	header := rec.Header()
	if header.Get("Content-Type") != "application/octet-stream" || header.Get("X-Content-Type-Options") != "nosniff" ||
		header.Get("Content-Disposition") != `attachment; filename="evil__x.html"` {
		t.Fatalf("headers = %v", header)
	}
	if virtualComputersSafeDownloadName("/") != "download" || virtualComputersSafeDownloadName("..") != "download" {
		t.Fatal("degenerate paths must fall back to download")
	}
}

func TestVirtualComputersForkCapsWhenMaxForksUnset(t *testing.T) {
	fake, upstream := newFakeMachinesBoringd(t)
	s, _, mux := newMachineTestServer(t, upstream.URL)
	s.Cfg.VirtualComputers.MaxForks = 0
	rec := machineTestRequest(mux, http.MethodPost, "/api/virtual-computers/machines/vm-1/fork", `{"count":4}`)
	if payload := decodeVolumeTestBody(t, rec); rec.Code != http.StatusBadRequest || payload["code"] != "invalid_argument" {
		t.Fatalf("fork status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.count("POST /v1/machines/vm-1/branch") != 0 {
		t.Fatal("over-limit fork reached boringd")
	}
}

func TestVirtualComputersExtendRejectsMalformedJSONAndFollowsAdoption(t *testing.T) {
	fake, upstream := newFakeMachinesBoringd(t)
	_, ledger, mux := newMachineTestServer(t, upstream.URL)
	ctx := context.Background()
	if rec := machineTestRequest(mux, http.MethodPost, "/api/virtual-computers/machines/vm-1/extend", `{bad`); rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed status = %d", rec.Code)
	}
	rec := machineTestRequest(mux, http.MethodPost, "/api/virtual-computers/machines/vm-1/extend", `{"ttl_seconds":5000}`)
	if payload := decodeVolumeTestBody(t, rec); rec.Code != http.StatusBadRequest || payload["code"] != "ttl_too_long" {
		t.Fatalf("too long status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.count("POST /v1/machines/vm-1/extend") != 0 {
		t.Fatal("rejected extends reached boringd")
	}
	now := time.Now().UTC()
	if err := ledger.UpsertWorkspace(ctx, virtualcomputers.Workspace{
		ID: "ws-a", OwnerSessionID: virtualcomputers.AdoptedWorkspaceOwnerSessionID, MachineID: "vm-1", State: virtualcomputers.WorkspaceStateReady,
		Origin: virtualcomputers.WorkspaceOriginAdopted, LeaseExpiresAt: now.Add(time.Hour), MaxExpiresAt: now.Add(5 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if rec := machineTestRequest(mux, http.MethodPost, "/api/virtual-computers/machines/vm-1/extend", `{"ttl_seconds":600}`); rec.Code != http.StatusOK {
		t.Fatalf("extend status=%d body=%s", rec.Code, rec.Body.String())
	}
	workspace, _, _ := ledger.GetWorkspace(ctx, "ws-a")
	if workspace.MaxExpiresAt.Before(now.Add(20 * time.Minute)) {
		t.Fatalf("adopted workspace expiry not moved: %v", workspace.MaxExpiresAt)
	}
}

type serverAdoptTransport struct {
	mu      sync.Mutex
	methods []string
}

func (t *serverAdoptTransport) Call(_ context.Context, machineID, method string, _ interface{}, result interface{}) error {
	t.mu.Lock()
	t.methods = append(t.methods, method)
	t.mu.Unlock()
	if method == "system.capabilities" {
		*(result.(*virtualcomputers.WorkspaceCapabilities)) = virtualcomputers.WorkspaceCapabilities{
			ProtocolVersion: virtualcomputers.WorkspaceProtocolVersion, MachineID: machineID, InstanceNonce: "n-1",
			Capabilities: []string{"shell.exec"}, MaxMessageBytes: 1 << 20,
		}
	}
	return nil
}

func TestVirtualComputersAssignAgentRouteAndRelease(t *testing.T) {
	fake, upstream := newFakeMachinesBoringd(t)
	s, ledger, mux := newMachineTestServer(t, upstream.URL)
	s.Cfg.VirtualComputers.AgentControl.Enabled = true
	s.Cfg.VirtualComputers.AgentControl.MaxActiveWorkspaces = 2
	s.Cfg.VirtualComputers.AgentControl.IdleTTLSeconds = 600
	s.Cfg.VirtualComputers.AgentControl.MaxWorkspaceSeconds = 3600
	transport := &serverAdoptTransport{}
	manager, err := virtualcomputers.NewWorkspaceManager(ledger, slog.Default(), virtualcomputers.WorkspaceManagerOptions{
		ClientFactory: func(virtualcomputers.ToolConfig) (*virtualcomputers.Client, error) {
			return virtualcomputers.NewClient(virtualcomputers.ClientConfig{BaseURL: upstream.URL, Timeout: 2 * time.Second})
		},
		TransportFactory:  func(*virtualcomputers.Client) virtualcomputers.WorkspaceTransport { return transport },
		ReconcileInterval: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	s.VirtualWorkspaceManager = manager

	if rec := machineTestRequest(mux, http.MethodGet, "/api/virtual-computers/machines/vm-1/assign-agent", ""); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d", rec.Code)
	}
	rec := machineTestRequest(mux, http.MethodPost, "/api/virtual-computers/machines/vm-1/assign-agent", "")
	payload := decodeVolumeTestBody(t, rec)
	workspace, _ := payload["workspace"].(map[string]interface{})
	if rec.Code != http.StatusOK || payload["reused"] != false || workspace["origin"] != "adopted" ||
		workspace["owner_session_id"] != "virtual-desktop" || workspace["network_profile"] != "machine" {
		t.Fatalf("assign status=%d body=%s", rec.Code, rec.Body.String())
	}
	workspaceID, _ := workspace["id"].(string)
	rec = machineTestRequest(mux, http.MethodPost, "/api/virtual-computers/machines/vm-1/assign-agent", "")
	if payload := decodeVolumeTestBody(t, rec); rec.Code != http.StatusOK || payload["reused"] != true {
		t.Fatalf("second assign status=%d body=%s", rec.Code, rec.Body.String())
	}
	// The destroy guard now covers the adopted machine.
	rec = machineTestRequest(mux, http.MethodDelete, "/api/virtual-computers/machines/vm-1", "")
	if payload := decodeVolumeTestBody(t, rec); rec.Code != http.StatusConflict || payload["origin"] != "adopted" {
		t.Fatalf("destroy status=%d body=%s", rec.Code, rec.Body.String())
	}
	// Closing the workspace releases: released=true and no DELETE upstream.
	rec = machineTestRequest(mux, http.MethodDelete, "/api/virtual-computers/workspaces/"+workspaceID, "")
	if payload := decodeVolumeTestBody(t, rec); rec.Code != http.StatusOK || payload["released"] != true {
		t.Fatalf("release status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.count("DELETE") != 0 {
		t.Fatalf("release destroyed the machine: %v", fake.requests)
	}
	transport.mu.Lock()
	for _, method := range transport.methods {
		if method == "workspace.close" {
			t.Fatal("release called workspace.close")
		}
	}
	transport.mu.Unlock()
	// Without a manager the route answers 503.
	s.VirtualWorkspaceManager = nil
	virtualcomputers.SetDefaultWorkspaceManager(nil)
	if rec := machineTestRequest(mux, http.MethodPost, "/api/virtual-computers/machines/vm-1/assign-agent", ""); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("no manager status = %d", rec.Code)
	}
}

func TestVirtualComputersSetupInstallSingleFlight(t *testing.T) {
	_, upstream := newFakeMachinesBoringd(t)
	s, _, mux := newMachineTestServer(t, upstream.URL)
	s.Cfg.VirtualComputers.AutoSetup = true
	virtualComputersManualSetup.Lock()
	defer virtualComputersManualSetup.Unlock()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/virtual-computers/setup/install", bytes.NewReader([]byte(`{}`))))
	if payload := decodeVolumeTestBody(t, rec); rec.Code != http.StatusConflict || payload["code"] != "setup_in_progress" {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
