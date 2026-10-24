package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/infra"
)

func TestDesktopInfraSourceHostRouteUsesVersionCAS(t *testing.T) {
	_, cfg, store, runtime := newDesktopInfraRefreshFixture(t)
	ctx := context.Background()
	instance := desktopInfraDockerInstance(cfg)
	if err := store.ObserveSources(ctx, "docker", instance, []infra.SourceObservation{{
		Identity: infra.SourceIdentity{Kind: "docker", Instance: instance, Key: strings.Repeat("c3", 32)}, Name: "web",
		Observed: infra.SourceObserved{Kind: infra.NodeContainer, Name: "web", Icon: "container"},
	}}, true); err != nil {
		t.Fatal(err)
	}
	host, err := store.CreateNode(ctx, infra.Node{Kind: infra.NodeDevice, Name: "TheBrain", Icon: "tower-server"})
	if err != nil {
		t.Fatal(err)
	}
	call := func(body string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		desktopInfraSetSourceHost(response, httptest.NewRequest(http.MethodPut, desktopInfraBasePath+"/source-hosts", strings.NewReader(body)), store)
		return response
	}
	response := call(`{"kind":"docker","instance":"` + instance + `","node_id":"` + host.ID + `","version":0}`)
	if response.Code != http.StatusOK {
		t.Fatalf("assign host = %d %s", response.Code, response.Body.String())
	}
	var payload struct {
		SourceHost infra.SourceHost `json:"source_host"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.SourceHost.NodeID != host.ID || payload.SourceHost.Version != 1 {
		t.Fatalf("assigned source host = %#v", payload.SourceHost)
	}
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"stale version", `{"kind":"docker","instance":"` + instance + `","node_id":"","version":0}`, http.StatusConflict},
		{"unknown engine", `{"kind":"docker","instance":"docker_missing","node_id":"","version":0}`, http.StatusNotFound},
		{"runtime field", `{"kind":"docker","instance":"` + instance + `","node_id":"","version":1,"inferred_node_id":"x"}`, http.StatusBadRequest},
	} {
		if got := call(tc.body); got.Code != tc.status {
			t.Fatalf("%s = %d %s; want %d", tc.name, got.Code, got.Body.String(), tc.status)
		}
	}
	snapshot, err := store.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(desktopInfraSnapshotResponse(snapshot, runtime))
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		SourceHosts []infra.SourceHost `json:"source_hosts"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.SourceHosts) != 1 || decoded.SourceHosts[0].NodeID != host.ID || decoded.SourceHosts[0].Version != 1 {
		t.Fatalf("snapshot source_hosts = %#v", decoded.SourceHosts)
	}
}

func TestDesktopInfraDockerEngineAddresses(t *testing.T) {
	local := []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("fe80::1"), netip.MustParseAddr("192.0.2.10")}
	for _, tc := range []struct {
		host string
		want []string
	}{
		{"", []string{"192.0.2.10"}},
		{"unix:///var/run/docker.sock", []string{"192.0.2.10"}},
		{"npipe:////./pipe/docker_engine", []string{"192.0.2.10"}},
		{"tcp://localhost:2375", []string{"192.0.2.10"}},
		{"tcp://127.0.0.1:2375", []string{"192.0.2.10"}},
		{"tcp://192.0.2.50:2375", []string{"192.0.2.50"}},
		{"tcp://docker-proxy:2375", nil},
		{"mock://docker", nil},
	} {
		var got []string
		for _, addr := range desktopInfraDockerEngineAddresses(tc.host, local) {
			got = append(got, addr.String())
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("engine addresses for %q = %v; want %v", tc.host, got, tc.want)
		}
	}
	nodes := []infra.Node{
		{ID: "a", Kind: infra.NodeDevice, Profile: infra.Profile{IPAddress: "192.0.2.50"}},
		{ID: "b", Kind: infra.NodeDevice, Profile: infra.Profile{Interfaces: []infra.NetworkInterface{{Name: "eth0", IPAddress: "192.0.2.50"}}}},
	}
	engine := []netip.Addr{netip.MustParseAddr("192.0.2.50")}
	if got := desktopInfraNodeWithAddress(nodes, engine); got != "" {
		t.Fatalf("ambiguous address inferred %q", got)
	}
	if got := desktopInfraNodeWithAddress(nodes[:1], engine); got != "a" {
		t.Fatalf("unique address inferred %q; want a", got)
	}
}

func TestDesktopInfraHostProjectionUsesProxmoxEdgesDockerMappingAndInference(t *testing.T) {
	s, cfg, store, _ := newDesktopInfraRefreshFixture(t)
	ctx := context.Background()
	original := desktopInfraLocalAddresses
	desktopInfraLocalAddresses = func() []netip.Addr {
		return []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("192.0.2.10")}
	}
	t.Cleanup(func() { desktopInfraLocalAddresses = original })
	cfg.Docker.Host = "unix:///var/run/docker.sock"
	dockerInstance := desktopInfraDockerInstance(cfg)

	importSource := func(observation infra.SourceObservation) infra.Node {
		t.Helper()
		if err := store.UpsertObservation(ctx, observation); err != nil {
			t.Fatal(err)
		}
		candidate, err := store.GetCandidate(ctx, infra.CandidateID(observation.Identity))
		if err != nil {
			t.Fatal(err)
		}
		node, err := store.CreateImportedNode(ctx, candidate.Node, observation.Identity)
		if err != nil {
			t.Fatal(err)
		}
		return node
	}
	observed := func(kind infra.NodeKind, name, icon string) infra.SourceObserved {
		return infra.SourceObserved{Kind: kind, Name: name, Icon: icon, Tags: []string{}, Interfaces: []infra.NetworkInterface{}}
	}
	guestObserved := observed(infra.NodeVM, "web-vm", "vm")
	guestObserved.Host = "pve-a"
	guest := importSource(infra.SourceObservation{Identity: infra.SourceIdentity{Kind: "proxmox", Instance: "pve-test", Key: "qemu:101"}, Name: "web-vm", Observed: guestObserved})
	hypervisor := importSource(infra.SourceObservation{Identity: infra.SourceIdentity{Kind: "proxmox", Instance: "pve-test", Key: "node:pve-a"}, Name: "pve-a", Observed: observed(infra.NodeDevice, "pve-a", "hypervisor")})
	if err := desktopInfraReconcileProxmoxEdges(ctx, store, "pve-test"); err != nil {
		t.Fatal(err)
	}
	container := importSource(infra.SourceObservation{Identity: infra.SourceIdentity{Kind: "docker", Instance: dockerInstance, Key: "c0ffee"}, Name: "radarr", Observed: observed(infra.NodeContainer, "radarr", "container")})
	brain, err := store.CreateNode(ctx, infra.Node{Kind: infra.NodeDevice, Name: "TheBrain", Icon: "tower-server", Profile: infra.Profile{IPAddress: "192.0.2.10"}})
	if err != nil {
		t.Fatal(err)
	}
	nas, err := store.CreateNode(ctx, infra.Node{Kind: infra.NodeDevice, Name: "NAS", Icon: "nas"})
	if err != nil {
		t.Fatal(err)
	}

	decorated := func() infra.Snapshot {
		t.Helper()
		snapshot, err := store.Snapshot(ctx)
		if err != nil {
			t.Fatal(err)
		}
		desktopInfraDecorateSnapshot(ctx, s, cfg, &snapshot)
		return snapshot
	}
	nodeByID := func(snapshot infra.Snapshot, id string) infra.Node {
		t.Helper()
		for _, node := range snapshot.Nodes {
			if node.ID == id {
				return node
			}
		}
		t.Fatalf("node %s missing from the snapshot", id)
		return infra.Node{}
	}
	snapshot := decorated()
	if got := nodeByID(snapshot, guest.ID); got.HostID != hypervisor.ID || got.HostInferred {
		t.Fatalf("Proxmox guest host = %q inferred=%v; want %q from runs_on", got.HostID, got.HostInferred, hypervisor.ID)
	}
	if got := nodeByID(snapshot, container.ID); got.HostID != brain.ID || !got.HostInferred {
		t.Fatalf("Docker guest host = %q inferred=%v; want inferred %q", got.HostID, got.HostInferred, brain.ID)
	}
	if got := nodeByID(snapshot, brain.ID); got.HostID != "" {
		t.Fatalf("a device received a host: %q", got.HostID)
	}
	if len(snapshot.SourceHosts) != 1 || snapshot.SourceHosts[0].Instance != dockerInstance ||
		snapshot.SourceHosts[0].InferredNodeID != brain.ID || snapshot.SourceHosts[0].Version != 0 {
		t.Fatalf("source hosts = %#v", snapshot.SourceHosts)
	}
	if _, err := store.SetSourceHost(ctx, infra.SourceHost{Kind: "docker", Instance: dockerInstance, NodeID: nas.ID}); err != nil {
		t.Fatal(err)
	}
	if got := nodeByID(decorated(), container.ID); got.HostID != nas.ID || got.HostInferred {
		t.Fatalf("mapped Docker guest host = %q inferred=%v; want %q", got.HostID, got.HostInferred, nas.ID)
	}
	written := desktopInfraApplyHosts(ctx, store, cfg, []infra.Node{container})
	if written[0].HostID != nas.ID || written[0].HostInferred {
		t.Fatalf("write response host = %q inferred=%v; want %q", written[0].HostID, written[0].HostInferred, nas.ID)
	}
}
