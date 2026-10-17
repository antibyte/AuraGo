package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/infra"
	"aurago/internal/inventory"
	"aurago/internal/tools"
)

func TestDesktopInfraRouteRequiresAdminAndAppliesInfraReadOnly(t *testing.T) {
	s, readToken, writeToken := testDesktopPermissionServer(t)
	adminToken, _, err := s.TokenManager.Create("infra admin", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s.InventoryDB = db
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.VirtualDesktop.Infra.Enabled = true

	mux := http.NewServeMux()
	registerDesktopInfraRoutes(mux, s)
	call := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}

	for _, token := range []string{"", readToken, writeToken} {
		response := call(http.MethodGet, desktopInfraBasePath+"/snapshot", token, "")
		if response.Code != http.StatusForbidden && response.Code != http.StatusUnauthorized {
			t.Fatalf("non-admin token status=%d body=%s", response.Code, response.Body.String())
		}
	}
	if s.infraRuntime != nil {
		t.Fatal("unauthorized requests initialized the Infra store")
	}

	response := call(http.MethodGet, desktopInfraBasePath+"/snapshot", adminToken, "")
	if response.Code != http.StatusOK {
		t.Fatalf("admin snapshot status=%d body=%s", response.Code, response.Body.String())
	}
	var snapshot infra.Snapshot
	if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Nodes == nil || snapshot.Edges == nil || snapshot.Sources == nil || !snapshot.Capabilities.Write {
		t.Fatalf("snapshot did not expose initialized arrays/capabilities: %+v", snapshot)
	}

	s.Cfg.VirtualDesktop.Infra.ReadOnly = true
	response = call(http.MethodPost, desktopInfraBasePath+"/nodes", adminToken, `{"kind":"service","name":"blocked"}`)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "infra_readonly") {
		t.Fatalf("Infra readonly did not block admin mutation: %d %s", response.Code, response.Body.String())
	}
}

func TestDesktopInfraRegistryProjectionOmitsCredentialReferences(t *testing.T) {
	s, _, _ := testDesktopPermissionServer(t)
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s.InventoryDB = db
	id, err := inventory.CreateDevice(db, "router", "router", inventory.ProtocolNone, "192.0.2.4", 0,
		"operator", "vault-reference-never-expose", "", "edge", []string{"lab"}, "02:00:00:00:00:04")
	if err != nil {
		t.Fatal(err)
	}
	device, err := inventory.GetDeviceByID(db, id)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := json.Marshal(desktopInfraRegistryObserved(device))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(observed), "vault-reference-never-expose") || strings.Contains(string(observed), "operator") || strings.Contains(string(observed), "credential_id") {
		t.Fatalf("registry observation leaked editable/secret data: %s", observed)
	}

	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.Infra.Enabled = true
	cfg.VirtualDesktop.Infra.AllowSourceEdit = true
	edit, err := desktopInfraReadSourceEdit(context.Background(), s, cfg, nil, infra.Source{
		SourceIdentity: infra.SourceIdentity{Kind: "registry", Instance: "inventory", Key: id},
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(edit)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "vault-reference-never-expose") || strings.Contains(string(encoded), "vault_secret_id") {
		t.Fatalf("registry source-edit projection leaked the vault reference: %s", encoded)
	}
}

func TestDesktopInfraDockerActionPersistsUncertainOutcomeWithoutRetry(t *testing.T) {
	var reads, writes int
	cfg := &config.Config{}
	configureDesktopInfraTestWrite(cfg)
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "mock://docker"
	runtime := &desktopInfraRuntime{dockerRequest: func(_ context.Context, _ tools.DockerConfig, method, endpoint, _ string) ([]byte, int, error) {
		if method == http.MethodGet && endpoint == "/containers/deadbeef/json" {
			reads++
			return []byte(`{"Id":"deadbeef","Name":"/web","Config":{"Image":"nginx","Labels":{}},"State":{"Status":"running"}}`), http.StatusOK, nil
		}
		if method == http.MethodPost && endpoint == "/containers/deadbeef/restart" {
			writes++
			return nil, 0, errors.New("mock transport failed after send")
		}
		t.Fatalf("unexpected Docker request %s %s", method, endpoint)
		return nil, 0, errors.New("unexpected request")
	}}
	source := infra.Source{SourceIdentity: infra.SourceIdentity{Kind: "docker", Instance: desktopInfraDockerInstance(cfg), Key: "deadbeef"}}
	request := desktopInfraActionRequest{Action: "restart", RequestID: "action-1", Confirmed: true}
	response := desktopInfraRunAction(context.Background(), nil, cfg, nil, runtime, "", source, request)
	if response.Status != infra.ActionUnknown || reads != 1 || writes != 1 {
		t.Fatalf("uncertain action = %+v, inspect requests=%d, mutation requests=%d", response, reads, writes)
	}
	if !desktopInfraActionNeedsConfirm("docker", "restart") || desktopInfraActionNeedsConfirm("docker", "start") {
		t.Fatal("Docker confirmation policy does not match the action contract")
	}
}

func TestDesktopInfraDockerActionCompletesOnlyAfterStateReadback(t *testing.T) {
	for _, tc := range []struct {
		name, action, before, after string
		beforeRestart, afterRestart int
		want                        string
	}{
		{name: "start confirmed", action: "start", before: "exited", after: "running", want: infra.ActionCompleted},
		{name: "stop confirmed", action: "stop", before: "running", after: "exited", want: infra.ActionCompleted},
		{name: "stop restarting is not confirmed", action: "stop", before: "running", after: "restarting", want: infra.ActionUnknown},
		{name: "stop missing state is not confirmed", action: "stop", before: "running", after: "", want: infra.ActionUnknown},
		{name: "stop paused is not confirmed", action: "stop", before: "running", after: "paused", want: infra.ActionUnknown},
		{name: "restart confirmed", action: "restart", before: "running", after: "running", beforeRestart: 2, afterRestart: 3, want: infra.ActionCompleted},
		{name: "restart state alone is insufficient", action: "restart", before: "running", after: "running", beforeRestart: 2, afterRestart: 2, want: infra.ActionUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{}
			configureDesktopInfraTestWrite(cfg)
			cfg.Docker.Enabled = true
			cfg.Docker.Host = "mock://docker"
			reads, writes := 0, 0
			runtime := &desktopInfraRuntime{dockerRequest: func(_ context.Context, _ tools.DockerConfig, method, endpoint, _ string) ([]byte, int, error) {
				switch {
				case method == http.MethodGet && endpoint == "/containers/deadbeef/json":
					reads++
					state, restartCount := tc.before, tc.beforeRestart
					if reads > 1 {
						state, restartCount = tc.after, tc.afterRestart
					}
					payload, _ := json.Marshal(map[string]any{
						"Id": "deadbeef", "Name": "/web", "RestartCount": restartCount,
						"Config": map[string]any{"Image": "nginx", "Labels": map[string]string{}},
						"State":  map[string]string{"Status": state, "StartedAt": "2026-10-16T10:00:00Z"},
					})
					return payload, http.StatusOK, nil
				case method == http.MethodPost && endpoint == "/containers/deadbeef/"+tc.action:
					writes++
					return nil, http.StatusNoContent, nil
				default:
					t.Fatalf("unexpected Docker request %s %s", method, endpoint)
					return nil, 0, errors.New("unexpected request")
				}
			}}
			source := infra.Source{SourceIdentity: infra.SourceIdentity{Kind: "docker", Instance: desktopInfraDockerInstance(cfg), Key: "deadbeef"}}
			response := desktopInfraRunAction(context.Background(), nil, cfg, nil, runtime, "", source,
				desktopInfraActionRequest{Action: tc.action, RequestID: "action-readback", Confirmed: true})
			if response.Status != tc.want || reads != 2 || writes != 1 {
				t.Fatalf("action = %+v, reads=%d writes=%d, want status=%s", response, reads, writes, tc.want)
			}
		})
	}
}

func TestDesktopInfraDockerRenameReportsUnknownAfterTransportFailure(t *testing.T) {
	var reads, writes int
	cfg := &config.Config{}
	configureDesktopInfraTestWrite(cfg)
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "mock://docker"
	runtime := &desktopInfraRuntime{dockerRequest: func(_ context.Context, _ tools.DockerConfig, method, endpoint, _ string) ([]byte, int, error) {
		if method == http.MethodGet && endpoint == "/containers/deadbeef/json" {
			reads++
			return []byte(`{"Id":"deadbeef","Name":"/web","Config":{"Image":"nginx","Labels":{}},"State":{"Status":"running"}}`), http.StatusOK, nil
		}
		if method == http.MethodPost && endpoint == "/containers/deadbeef/rename?name=web-new" {
			writes++
			return nil, 0, errors.New("mock transport failed after send")
		}
		t.Fatalf("unexpected Docker request %s %s", method, endpoint)
		return nil, 0, errors.New("unexpected request")
	}}
	source := infra.Source{SourceIdentity: infra.SourceIdentity{Kind: "docker", Instance: desktopInfraDockerInstance(cfg), Key: "deadbeef"}}
	before := desktopInfraDockerInspect{ID: "deadbeef", Name: "/web"}
	before.Config.Image = "nginx"
	before.State.Status = "running"
	result, status, err := desktopInfraWriteSourceEdit(context.Background(), nil, cfg, nil, runtime, "", source, desktopInfraSourceEditRequest{
		Source: "docker", Version: desktopInfraDockerVersion(before), Fields: json.RawMessage(`{"name":"web-new"}`),
	})
	if err != nil || status != http.StatusOK || result.Status != infra.ActionUnknown || writes != 1 || reads != 1 {
		t.Fatalf("rename uncertainty = result=%+v status=%d err=%v reads=%d writes=%d", result, status, err, reads, writes)
	}
}

func configureDesktopInfraTestWrite(cfg *config.Config) {
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.Infra.Enabled = true
	cfg.VirtualDesktop.Infra.AllowControl = true
	cfg.VirtualDesktop.Infra.AllowSourceEdit = true
}

func TestDesktopInfraActionPoliciesHonorProviderReadOnlyAndDestructiveGates(t *testing.T) {
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Proxmox.Enabled = true
	cfg.Proxmox.AllowDestructive = true
	cfg.FritzBox.Enabled = true
	cfg.FritzBox.Host = "fritz.lan"
	cfg.FritzBox.Network.Enabled = true
	cfg.FritzBox.Network.SubFeatures.Hosts = true
	cfg.FritzBox.Network.SubFeatures.WakeOnLAN = true
	for _, tc := range []struct {
		name, source, action string
		readonly             func()
		want                 bool
	}{
		{name: "docker start", source: "docker", action: "start", want: true},
		{name: "docker stop readonly", source: "docker", action: "stop", readonly: func() { cfg.Docker.ReadOnly = true }, want: false},
		{name: "proxmox start", source: "proxmox", action: "start", want: true},
		{name: "proxmox shutdown gated", source: "proxmox", action: "shutdown", want: true},
		{name: "proxmox reboot readonly", source: "proxmox", action: "reboot", readonly: func() { cfg.Proxmox.ReadOnly = true }, want: false},
		{name: "fritz wake", source: "fritzbox", action: "wake", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg.Docker.ReadOnly = false
			cfg.Proxmox.ReadOnly = false
			if tc.readonly != nil {
				tc.readonly()
			}
			if got := desktopInfraActionAllowed(cfg, tc.source, tc.action); got != tc.want {
				t.Fatalf("allowed=%v, want %v", got, tc.want)
			}
		})
	}
	cfg.Proxmox.AllowDestructive = false
	if desktopInfraActionAllowed(cfg, "proxmox", "shutdown") {
		t.Fatal("Proxmox shutdown ignored the destructive-action gate")
	}
}

func TestDesktopInfraProxmoxObservationsKeepHostIdentityNarrowAndGuestIdentityStable(t *testing.T) {
	host, err := desktopInfraProxmoxObservation("pve-cluster", tools.InfraProxmoxResource{
		Type: "node", Node: "pve-a", Name: "pve-a", Status: "online", CPU: 2, MaxCPU: 8,
		MaxMemory: 16 << 30, MaxDisk: 512 << 30, Tags: "production",
	})
	if err != nil {
		t.Fatal(err)
	}
	if host.Identity.Key != "node:pve-a" || host.Observed.Kind != infra.NodeDevice || host.Observed.Icon != "hypervisor" {
		t.Fatalf("host projection identity/type = %#v / %#v", host.Identity, host.Observed)
	}
	if host.Observed.Role != "" || host.Observed.Vendor != "" || host.Observed.Model != "" || host.Observed.CPU != "" || host.Observed.RAM != "" || host.Observed.Storage != "" || len(host.Observed.Tags) != 0 {
		t.Fatalf("host metadata should remain hidden: %#v", host.Observed)
	}
	guestA, err := desktopInfraProxmoxObservation("pve-cluster", tools.InfraProxmoxResource{Type: "qemu", VMID: "101", Node: "pve-a", Name: "web"})
	if err != nil {
		t.Fatal(err)
	}
	guestB, err := desktopInfraProxmoxObservation("pve-cluster", tools.InfraProxmoxResource{Type: "qemu", VMID: "101", Node: "pve-b", Name: "web"})
	if err != nil {
		t.Fatal(err)
	}
	if guestA.Identity.Key != "qemu:101" || guestB.Identity.Key != guestA.Identity.Key || guestA.Observed.Host != "pve-a" || guestB.Observed.Host != "pve-b" {
		t.Fatalf("guest identity/placement must survive migration: A=%#v B=%#v", guestA, guestB)
	}
	lxc, err := desktopInfraProxmoxObservation("pve-cluster", tools.InfraProxmoxResource{Type: "lxc", VMID: "102", Node: "pve-a"})
	if err != nil {
		t.Fatal(err)
	}
	if lxc.Observed.Kind != infra.NodeContainer || lxc.Observed.Icon != "container" {
		t.Fatalf("LXC source projection = %#v", lxc.Observed)
	}
	stopped, err := desktopInfraProxmoxObservation("pve-cluster", tools.InfraProxmoxResource{Type: "qemu", VMID: "103", Node: "pve-a", Name: "db", Status: "stopped"})
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Observed.Power != infra.PowerStopped || stopped.Observed.Role != "" || stopped.Observed.Model != "qemu" {
		t.Fatalf("guest power projection = %#v", stopped.Observed)
	}
	if desktopInfraPower("docker", "exited") != infra.PowerStopped || desktopInfraPower("docker", "restarting") != infra.PowerRunning || desktopInfraPower("docker", "paused") != infra.PowerPaused || desktopInfraPower("docker", "weird") != infra.PowerUnknown || desktopInfraPower("docker", "") != "" {
		t.Fatal("Docker power vocabulary drifted")
	}
	if desktopInfraPower("proxmox", "suspended") != infra.PowerPaused || desktopInfraFritzPower(true) != infra.PowerRunning || desktopInfraFritzPower(false) != infra.PowerStopped {
		t.Fatal("Proxmox or Fritz!Box power vocabulary drifted")
	}
}

func TestDesktopInfraProxmoxHostHasNoMetadataEditorOrPowerActions(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := infra.New(db)
	if err != nil {
		t.Fatal(err)
	}
	s, _, _ := testDesktopPermissionServer(t)
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.Infra.Enabled = true
	cfg.VirtualDesktop.Infra.AllowSourceEdit = true
	cfg.VirtualDesktop.Infra.AllowControl = true
	cfg.Proxmox.Enabled = true
	cfg.Proxmox.URL = "https://pve.example:8006"
	cfg.Proxmox.TokenID = "infra-test@pve!desktop"
	cfg.Proxmox.Secret = "test-secret"
	instance := desktopInfraProxmoxInstance(cfg)
	identity := infra.SourceIdentity{Kind: "proxmox", Instance: instance, Key: "node:pve-a"}
	observation, err := desktopInfraProxmoxObservation(instance, tools.InfraProxmoxResource{Type: "node", Node: "pve-a", Name: "pve-a"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ObserveSources(context.Background(), "proxmox", instance, []infra.SourceObservation{observation}, true); err != nil {
		t.Fatal(err)
	}
	candidate, err := store.GetCandidate(context.Background(), infra.CandidateID(identity))
	if err != nil {
		t.Fatal(err)
	}
	node, err := store.CreateImportedNode(context.Background(), candidate.Node, identity)
	if err != nil {
		t.Fatal(err)
	}
	projected := desktopInfraProjectNode(node, cfg, nil, nil)
	if len(projected.AvailableActions) != 0 || len(projected.ActionReasons) != 0 {
		t.Fatalf("hypervisor source exposed power controls: actions=%#v reasons=%#v", projected.AvailableActions, projected.ActionReasons)
	}
	request := httptest.NewRequest(http.MethodGet, desktopInfraBasePath+"/nodes/"+node.ID+"/source-edit?source=proxmox", nil)
	response := httptest.NewRecorder()
	desktopInfraSourceEdit(response, request, s, cfg, store, nil, node.ID)
	if response.Code != http.StatusOK {
		t.Fatalf("host metadata response status=%d body=%s", response.Code, response.Body.String())
	}
	var edit desktopInfraSourceEditResponse
	if err := json.Unmarshal(response.Body.Bytes(), &edit); err != nil {
		t.Fatal(err)
	}
	if edit.Editable || edit.Reason != "source_readonly" || edit.Fields != nil {
		t.Fatalf("hypervisor metadata editor should be absent: %+v", edit)
	}
}

func TestDesktopInfraProxmoxEditFieldsUseKindSpecificNamesAndPendingStatus(t *testing.T) {
	for _, tc := range []struct {
		kind, key string
	}{
		{kind: "qemu", key: "name"},
		{kind: "lxc", key: "hostname"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			fields, err := desktopInfraProxmoxFields(tc.kind, tools.InfraProxmoxConfig{
				Name: "guest-name", Description: "guest description", Tags: "lab", Pending: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			var projected map[string]any
			if err := json.Unmarshal(encoded, &projected); err != nil {
				t.Fatal(err)
			}
			if projected[tc.key] != "guest-name" || projected["description"] != "guest description" || projected["tags"] != "lab" {
				t.Fatalf("%s projection = %s", tc.kind, encoded)
			}
			other := "name"
			if tc.key == "name" {
				other = "hostname"
			}
			if _, exists := projected[other]; exists {
				t.Fatalf("%s projection exposed the wrong name field: %s", tc.kind, encoded)
			}
			name, err := desktopInfraProxmoxNameField(tc.kind, fields)
			if err != nil || name != "guest-name" {
				t.Fatalf("%s input name = %q, err=%v", tc.kind, name, err)
			}
		})
	}
	if _, err := desktopInfraProxmoxNameField("lxc", desktopInfraProxmoxEditFields{Name: "wrong-field"}); err == nil {
		t.Fatal("LXC name field was accepted in place of hostname")
	}
	if _, err := desktopInfraProxmoxNameField("qemu", desktopInfraProxmoxEditFields{Hostname: "wrong-field"}); err == nil {
		t.Fatal("QEMU hostname field was accepted in place of name")
	}
	response := desktopInfraSourceEditResponse{Source: "proxmox", Fields: desktopInfraProxmoxEditFields{Hostname: "new-host"}, Status: "pending", Pending: true}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"status":"pending"`) || !strings.Contains(string(encoded), `"pending":true`) || !strings.Contains(string(encoded), `"hostname":"new-host"`) {
		t.Fatalf("pending source response = %s", encoded)
	}
}

func TestDesktopInfraProjectionMarksDisabledOrReconfiguredSourcesStale(t *testing.T) {
	fetchedAt := time.Date(2026, time.October, 16, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name     string
		setup    func(*config.Config)
		instance string
	}{
		{name: "disabled provider", setup: func(cfg *config.Config) {}, instance: "docker_old"},
		{name: "reconfigured provider", setup: func(cfg *config.Config) { cfg.Docker.Enabled = true; cfg.Docker.Host = "mock://new" }, instance: "docker_old"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{}
			tc.setup(cfg)
			source := infra.Source{
				SourceIdentity: infra.SourceIdentity{Kind: "docker", Instance: tc.instance, Key: "container-id"},
				State:          infra.SourceCurrent,
				FetchedAt:      &fetchedAt,
				Observed:       infra.SourceObserved{Kind: infra.NodeContainer, Name: "last known", Icon: "container"},
			}
			snapshot := infra.Snapshot{
				Nodes:   []infra.Node{{ID: "node-1", Sources: []infra.Source{source}}},
				Sources: []infra.Source{source},
			}
			desktopInfraDecorateSnapshot(context.Background(), nil, cfg, &snapshot)
			if snapshot.Sources[0].State != infra.SourceStale || snapshot.Nodes[0].Sources[0].State != infra.SourceStale {
				t.Fatalf("nonconfigured source still appears current: snapshot=%#v node=%#v", snapshot.Sources[0], snapshot.Nodes[0].Sources[0])
			}
			if snapshot.Sources[0].FetchedAt == nil || !snapshot.Sources[0].FetchedAt.Equal(fetchedAt) || snapshot.Sources[0].Observed.Name != "last known" {
				t.Fatalf("staling discarded last-known facts: %#v", snapshot.Sources[0])
			}
		})
	}
}

func TestDesktopInfraRefreshesRunInParallelWithOneDeadline(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan time.Time, 2)
	release := make(chan struct{})
	finished := make(chan struct{})
	refresh := func(ctx context.Context) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Error("refresh context has no bounded deadline")
			return
		}
		started <- deadline
		<-release
		<-ctx.Done()
	}
	go func() {
		desktopInfraRunRefreshes(parent, time.Second, refresh, refresh)
		close(finished)
	}()
	var deadlines [2]time.Time
	for i := range deadlines {
		select {
		case deadlines[i] = <-started:
		case <-time.After(300 * time.Millisecond):
			close(release)
			cancel()
			<-finished
			t.Fatal("configured provider refreshes did not start concurrently")
		}
	}
	if !deadlines[0].Equal(deadlines[1]) {
		close(release)
		cancel()
		<-finished
		t.Fatalf("provider refreshes have separate deadlines: %v and %v", deadlines[0], deadlines[1])
	}
	close(release)
	cancel()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("provider refreshes did not stop after shared cancellation")
	}
}

func TestDesktopInfraTargetsAreConfiguredAndStoreBacked(t *testing.T) {
	dataDir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "desktop_store.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE desktop_store_apps (app_id TEXT, container_id TEXT, status TEXT);
		INSERT INTO desktop_store_apps VALUES ('radarr', 'container-radarr-full', 'running'), ('installing', 'container-pending', 'installing');`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Directories.DataDir = dataDir
	cfg.VirtualDesktop.Enabled = true
	cfg.Go2RTC.Enabled = true
	cfg.Go2RTC.Streams = []config.Go2RTCStreamConfig{
		{ID: "front-door", Name: "Front Door", Enabled: true, SourceConfigured: true},
		{ID: "disabled-cam", Name: "Disabled", Enabled: false},
		{ID: "invalid/cam", Name: "Invalid", Enabled: true},
	}
	targets, byContainer, err := desktopInfraTargets(context.Background(), &Server{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	wantTargets := []desktopInfraAppTarget{
		{App: "network-cameras", TargetID: "front-door", Label: "Front Door"},
		{App: "software-store", TargetID: "radarr", Label: "radarr"},
	}
	if !reflect.DeepEqual(targets, wantTargets) {
		t.Fatalf("targets = %#v, want %#v", targets, wantTargets)
	}
	if byContainer["container-radarr-full"] != "radarr" || len(byContainer) != 1 {
		t.Fatalf("store container associations = %#v", byContainer)
	}
	if err := desktopInfraValidateAppBindings(context.Background(), &Server{}, cfg, []infra.AppLink{{App: "network-cameras", TargetID: "front-door"}}, nil); err != nil {
		t.Fatalf("configured camera binding rejected: %v", err)
	}
	if err := desktopInfraValidateAppBindings(context.Background(), &Server{}, cfg, []infra.AppLink{{App: "network-cameras", TargetID: "not-configured"}}, nil); !errors.Is(err, infra.ErrValidation) {
		t.Fatalf("unconfigured camera binding error = %v", err)
	}
}

func TestDesktopInfraProfileSavesPreserveOfflineBindingsWithoutAddingUnverifiedTargets(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDir, "desktop_store.db"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Directories.DataDir = dataDir
	cfg.Go2RTC.Enabled = true
	cfg.Go2RTC.Streams = []config.Go2RTCStreamConfig{{ID: "front-door", Enabled: true, SourceConfigured: true}}
	previous := []infra.AppLink{{App: "network-cameras", TargetID: "front-door"}}
	if err := desktopInfraValidateAppBindings(context.Background(), &Server{}, cfg, previous, previous); err != nil {
		t.Fatalf("offline target blocked unchanged local profile binding: %v", err)
	}
	if err := desktopInfraValidateAppBindings(context.Background(), &Server{}, cfg, nil, previous); err != nil {
		t.Fatalf("offline target blocked removing a local binding: %v", err)
	}
	added := append(append([]infra.AppLink(nil), previous...), infra.AppLink{App: "meshcore", TargetID: "unverified-conversation"})
	if err := desktopInfraValidateAppBindings(context.Background(), &Server{}, cfg, added, previous); err == nil || errors.Is(err, infra.ErrValidation) {
		t.Fatalf("offline target did not block a newly added unverified binding: %v", err)
	}
}

func TestDesktopInfraProjectionValidatesAppLinksAndUsesOnlyCurrentSources(t *testing.T) {
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.Infra.Enabled = true
	cfg.VirtualDesktop.Infra.AllowControl = true
	cfg.Docker.Enabled = true
	cfg.Proxmox.Enabled = true
	cfg.Proxmox.AllowDestructive = true
	cfg.FritzBox.Enabled = true
	cfg.FritzBox.Host = "router.lan"
	cfg.FritzBox.Network.Enabled = true
	cfg.FritzBox.Network.SubFeatures.Hosts = true
	cfg.FritzBox.Network.SubFeatures.WakeOnLAN = true
	node := infra.Node{
		RegistryID: "registry-1",
		Sources: []infra.Source{
			{SourceIdentity: infra.SourceIdentity{Kind: "registry", Instance: "inventory", Key: "registry-1"}, State: infra.SourceCurrent},
			{SourceIdentity: infra.SourceIdentity{Kind: "docker", Instance: "old-docker-instance", Key: "docker-full-id"}, State: infra.SourceCurrent},
			{SourceIdentity: infra.SourceIdentity{Kind: "docker", Instance: desktopInfraDockerInstance(cfg), Key: "docker-full-id"}, State: infra.SourceCurrent},
			{SourceIdentity: infra.SourceIdentity{Kind: "proxmox", Instance: desktopInfraProxmoxInstance(cfg), Key: "qemu:100"}, State: infra.SourceCurrent},
			{SourceIdentity: infra.SourceIdentity{Kind: "fritzbox", Instance: desktopInfraFritzInstance(cfg), Key: "host-ref"}, State: infra.SourceCurrent},
		},
		Profile: infra.Profile{AppBindings: []infra.AppLink{
			{App: "network-cameras", TargetID: "front-door"},
			{App: "network-cameras", TargetID: "deleted-camera"},
			{App: "meshcore", TargetID: "conversation-1"},
		}},
	}
	links := map[string]struct{}{
		"network-cameras\x00front-door": {},
		"software-store\x00radarr":      {},
		"meshcore\x00conversation-1":    {},
	}
	projected := desktopInfraProjectNode(node, cfg, links, map[string]string{"docker-full-id": "radarr"})
	wantLinks := []infra.AppLink{
		{App: "quick-connect", TargetID: "registry-1"},
		{App: "network-cameras", TargetID: "front-door"},
		{App: "meshcore", TargetID: "conversation-1"},
		{App: "software-store", TargetID: "radarr"},
	}
	if !reflect.DeepEqual(projected.AppLinks, wantLinks) {
		t.Fatalf("app links = %#v, want %#v", projected.AppLinks, wantLinks)
	}
	oldDockerOnly := node
	oldDockerOnly.Sources = []infra.Source{node.Sources[1]}
	projectedOldDocker := desktopInfraProjectNode(oldDockerOnly, cfg, links, map[string]string{"docker-full-id": "radarr"})
	wantOldDockerLinks := []infra.AppLink{
		{App: "network-cameras", TargetID: "front-door"},
		{App: "meshcore", TargetID: "conversation-1"},
	}
	if !reflect.DeepEqual(projectedOldDocker.AppLinks, wantOldDockerLinks) {
		t.Fatalf("old Docker source changed explicit app bindings or inferred a Store link: got %#v want %#v", projectedOldDocker.AppLinks, wantOldDockerLinks)
	}
	wantActions := []string{"start", "stop", "restart", "shutdown", "reboot", "wake"}
	if !reflect.DeepEqual(projected.AvailableActions, wantActions) {
		t.Fatalf("available actions = %#v, want %#v", projected.AvailableActions, wantActions)
	}
	wantSourceActions := map[string][]string{
		infra.CandidateID(node.Sources[2].SourceIdentity): {"start", "stop", "restart"},
		infra.CandidateID(node.Sources[3].SourceIdentity): {"start", "shutdown", "reboot"},
		infra.CandidateID(node.Sources[4].SourceIdentity): {"wake"},
	}
	if !reflect.DeepEqual(projected.SourceActions, wantSourceActions) {
		t.Fatalf("exact source action projection = %#v, want %#v", projected.SourceActions, wantSourceActions)
	}

	stale := node
	for i := range stale.Sources {
		stale.Sources[i].State = infra.SourceStale
	}
	stale.Profile.Custom = map[string]string{"action": "restart", "target": "network-cameras/front-door"}
	projected = desktopInfraProjectNode(stale, cfg, links, map[string]string{"docker-full-id": "radarr"})
	wantStaleLinks := []infra.AppLink{
		{App: "network-cameras", TargetID: "front-door"},
		{App: "meshcore", TargetID: "conversation-1"},
	}
	if !reflect.DeepEqual(projected.AppLinks, wantStaleLinks) || len(projected.AvailableActions) != 0 {
		t.Fatalf("stale provider associations remained actionable: links=%#v actions=%#v", projected.AppLinks, projected.AvailableActions)
	}
	if len(projected.SourceActions) != 0 || len(projected.SourceActionReasons) == 0 {
		t.Fatalf("stale source action map = actions %#v reasons %#v", projected.SourceActions, projected.SourceActionReasons)
	}
}

func TestDesktopInfraActionProjectionBlocksStaleAndProtectedSources(t *testing.T) {
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.Infra.Enabled = true
	cfg.VirtualDesktop.Infra.AllowControl = true
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "mock://docker"
	protected := infra.Node{Sources: []infra.Source{{
		SourceIdentity: infra.SourceIdentity{Kind: "docker", Instance: desktopInfraDockerInstance(cfg), Key: "managed"},
		State:          infra.SourceCurrent, Observed: infra.SourceObserved{Protected: true},
	}}}
	projected := desktopInfraProjectNode(protected, cfg, nil, nil)
	if len(projected.AvailableActions) != 0 || projected.ActionReasons["start"] != "managed_container" {
		t.Fatalf("protected Docker projection = actions %#v reasons %#v", projected.AvailableActions, projected.ActionReasons)
	}
	protectedKey := infra.CandidateID(protected.Sources[0].SourceIdentity)
	if len(projected.SourceActions[protectedKey]) != 0 || projected.SourceActionReasons[protectedKey]["start"] != "managed_container" {
		t.Fatalf("protected Docker exact-source projection = actions %#v reasons %#v", projected.SourceActions, projected.SourceActionReasons)
	}
	oldSource := infra.Node{Sources: []infra.Source{{
		SourceIdentity: infra.SourceIdentity{Kind: "docker", Instance: "old-docker-instance", Key: "ordinary"}, State: infra.SourceCurrent,
	}}}
	projected = desktopInfraProjectNode(oldSource, cfg, nil, nil)
	if len(projected.AvailableActions) != 0 || projected.ActionReasons["start"] != "source_disabled" {
		t.Fatalf("old-instance Docker projection = actions %#v reasons %#v", projected.AvailableActions, projected.ActionReasons)
	}
	oldKey := infra.CandidateID(oldSource.Sources[0].SourceIdentity)
	if len(projected.SourceActions[oldKey]) != 0 || projected.SourceActionReasons[oldKey]["start"] != "source_disabled" {
		t.Fatalf("old-instance exact-source projection = actions %#v reasons %#v", projected.SourceActions, projected.SourceActionReasons)
	}
	combined := infra.Node{Sources: []infra.Source{
		oldSource.Sources[0],
		{SourceIdentity: infra.SourceIdentity{Kind: "docker", Instance: desktopInfraDockerInstance(cfg), Key: "current"}, State: infra.SourceCurrent},
	}}
	if source, ok := desktopInfraFindConfiguredNodeSource(combined, cfg, "docker"); !ok || source.Key != "current" {
		t.Fatalf("configured source resolver chose %#v, ok=%v", source.SourceIdentity, ok)
	}
	ambiguous := infra.Node{Sources: []infra.Source{
		{SourceIdentity: infra.SourceIdentity{Kind: "docker", Instance: desktopInfraDockerInstance(cfg), Key: "container-a"}, State: infra.SourceCurrent},
		{SourceIdentity: infra.SourceIdentity{Kind: "docker", Instance: desktopInfraDockerInstance(cfg), Key: "container-b"}, State: infra.SourceCurrent},
	}}
	projected = desktopInfraProjectNode(ambiguous, cfg, nil, nil)
	if len(projected.AvailableActions) != 0 || len(projected.SourceActions) != 0 {
		t.Fatalf("ambiguous source identities exposed actions: %#v / %#v", projected.AvailableActions, projected.SourceActions)
	}
	for _, source := range ambiguous.Sources {
		key := infra.CandidateID(source.SourceIdentity)
		if projected.SourceActionReasons[key]["start"] != "source_policy" {
			t.Fatalf("ambiguous source reason for %s = %#v", key, projected.SourceActionReasons[key])
		}
	}
}

func TestDesktopInfraProxmoxPlacementUsesImportedHostAndTracksMigration(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := infra.New(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	instance := "pve-test-instance"
	guestID := infra.SourceIdentity{Kind: "proxmox", Instance: instance, Key: "qemu:101"}
	hostAID := infra.SourceIdentity{Kind: "proxmox", Instance: instance, Key: "node:pve-a"}
	hostBID := infra.SourceIdentity{Kind: "proxmox", Instance: instance, Key: "node:pve-b"}
	observations := []infra.SourceObservation{
		{Identity: guestID, Name: "web", Observed: infra.SourceObserved{Kind: infra.NodeVM, Name: "web", Icon: "vm", Host: "pve-a", Tags: []string{}, Interfaces: []infra.NetworkInterface{}}},
		{Identity: hostAID, Name: "pve-a", Observed: infra.SourceObserved{Kind: infra.NodeDevice, Name: "pve-a", Icon: "hypervisor", Role: "hypervisor", Tags: []string{}, Interfaces: []infra.NetworkInterface{}}},
		{Identity: hostBID, Name: "pve-b", Observed: infra.SourceObserved{Kind: infra.NodeDevice, Name: "pve-b", Icon: "hypervisor", Role: "hypervisor", Tags: []string{}, Interfaces: []infra.NetworkInterface{}}},
	}
	if err := store.ObserveSources(ctx, "proxmox", instance, observations, true); err != nil {
		t.Fatal(err)
	}
	guestCandidate, err := store.GetCandidate(ctx, infra.CandidateID(guestID))
	if err != nil {
		t.Fatal(err)
	}
	guest, err := store.CreateImportedNode(ctx, guestCandidate.Node, guestID)
	if err != nil {
		t.Fatal(err)
	}
	if err := desktopInfraReconcileProxmoxEdges(ctx, store, instance); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Edges) != 0 {
		t.Fatalf("placement inferred without an imported host: %#v", snapshot.Edges)
	}
	hostCandidate, err := store.GetCandidate(ctx, infra.CandidateID(hostAID))
	if err != nil {
		t.Fatal(err)
	}
	hostA, err := store.CreateImportedNode(ctx, hostCandidate.Node, hostAID)
	if err != nil {
		t.Fatal(err)
	}
	if err := desktopInfraReconcileProxmoxEdges(ctx, store, instance); err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Edges) != 1 || snapshot.Edges[0].Kind != infra.EdgeRunsOn || snapshot.Edges[0].From != guest.ID || snapshot.Edges[0].To != hostA.ID || snapshot.Edges[0].Source == nil || *snapshot.Edges[0].Source != guestID {
		t.Fatalf("provider-confirmed placement edge = %#v", snapshot.Edges)
	}
	observations[0].Observed.Host = "pve-b"
	if err := store.ObserveSources(ctx, "proxmox", instance, observations, true); err != nil {
		t.Fatal(err)
	}
	hostCandidate, err = store.GetCandidate(ctx, infra.CandidateID(hostBID))
	if err != nil {
		t.Fatal(err)
	}
	hostB, err := store.CreateImportedNode(ctx, hostCandidate.Node, hostBID)
	if err != nil {
		t.Fatal(err)
	}
	if err := desktopInfraReconcileProxmoxEdges(ctx, store, instance); err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Edges) != 1 || snapshot.Edges[0].From != guest.ID || snapshot.Edges[0].To != hostB.ID {
		t.Fatalf("migration did not replace the provider placement: %#v", snapshot.Edges)
	}
}

func TestDesktopInfraProjectPowerUsesOnlyCurrentAgreement(t *testing.T) {
	cfg := &config.Config{}
	conflict := infra.Node{Sources: []infra.Source{
		{SourceIdentity: infra.SourceIdentity{Kind: "registry", Instance: "inventory", Key: "a"}, State: infra.SourceCurrent, Observed: infra.SourceObserved{Power: infra.PowerRunning}},
		{SourceIdentity: infra.SourceIdentity{Kind: "registry", Instance: "inventory", Key: "b"}, State: infra.SourceCurrent, Observed: infra.SourceObserved{Power: infra.PowerStopped}},
	}}
	projected := desktopInfraProjectNode(conflict, cfg, nil, nil)
	if projected.Power != infra.PowerUnknown {
		t.Fatalf("conflicting current power = %q", projected.Power)
	}
	stale := infra.Node{Sources: []infra.Source{
		{SourceIdentity: infra.SourceIdentity{Kind: "registry", Instance: "inventory", Key: "a"}, State: infra.SourceStale, Observed: infra.SourceObserved{Power: infra.PowerRunning}},
	}}
	projected = desktopInfraProjectNode(stale, cfg, nil, nil)
	if projected.Power != "" {
		t.Fatalf("stale power = %q", projected.Power)
	}
	stopped := infra.Node{Sources: []infra.Source{
		{SourceIdentity: infra.SourceIdentity{Kind: "registry", Instance: "inventory", Key: "a"}, State: infra.SourceCurrent, Observed: infra.SourceObserved{Power: infra.PowerStopped}},
	}}
	projected = desktopInfraProjectNode(stopped, cfg, nil, nil)
	if projected.Power != infra.PowerStopped {
		t.Fatalf("current stopped power = %q", projected.Power)
	}
}
