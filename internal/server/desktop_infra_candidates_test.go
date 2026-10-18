package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/infra"
	"aurago/internal/inventory"
)

func TestDesktopInfraCandidateNamed(t *testing.T) {
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"", false}, {"  ", false}, {"Unnamed device", false}, {"Fritz!Box host", false},
		{"02F3C3998E5A", false}, {"  075570b4da7eac420a914e059aad5a60  ", false},
		{"67bf621b-1540-4aae-8e70-a11234567890", false},
		{"uuid:67bf621b-1540-4aae-8e70-a11234567890", false},
		{"urn:uuid:67bf621b-1540-4aae-8e70-a11234567890", false},
		{"sha256:0123456789abcdef0123456789abcdef", false},
		{"02:F3:C3:99:8E:5A", false}, {"02-F3-C3-99-8E-5A", false},
		{"[LG] webOS TV", true}, {"router-02F3C3998E5A", true},
		{"Office printer", true}, {"192.0.2.4", true}, {"NAS-01", true},
	} {
		for _, kind := range []string{"registry", "fritzbox"} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				source := infra.Source{SourceIdentity: infra.SourceIdentity{Kind: kind}, Name: tc.name}
				if got := desktopInfraCandidateNamed(source); got != tc.want {
					t.Fatalf("candidate named = %v, want %v", got, tc.want)
				}
			})
		}
	}
	source := infra.Source{SourceIdentity: infra.SourceIdentity{Kind: "registry"}, Name: "02F3C3998E5A", Observed: infra.SourceObserved{Name: "Office TV"}}
	if !desktopInfraCandidateNamed(source) {
		t.Fatal("a resolved device name was hidden by its old source name")
	}
	for _, kind := range []string{"docker", "proxmox"} {
		source.Kind, source.Observed.Name = kind, "02F3C3998E5A"
		if !desktopInfraCandidateNamed(source) {
			t.Fatalf("%s resource name was treated as a scanned device", kind)
		}
	}
}

func TestDesktopInfraCandidatesHideOpaqueNamesAndPreserveSavedDevices(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := &Server{InventoryDB: db}
	cfg := &config.Config{}
	runtime, err := s.desktopInfraRuntime()
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"02F3C3998E5A", "67bf621b-1540-4aae-8e70-a11234567890", "Office TV", "aabbccddeeff"}
	var linkedID string
	for _, name := range names {
		id, err := inventory.CreateDevice(db, name, "device", inventory.ProtocolNone, "192.0.2.4", 0, "", "", "", "", nil, "")
		if err != nil {
			t.Fatal(err)
		}
		linkedID = id
	}
	desktopInfraRefreshRegistry(t.Context(), s, cfg, runtime.store, runtime, false)
	identity := infra.SourceIdentity{Kind: "registry", Instance: "inventory", Key: linkedID}
	candidate, err := runtime.store.GetCandidate(t.Context(), infra.CandidateID(identity))
	if err != nil {
		t.Fatal(err)
	}
	candidate.Node.RegistryID = ""
	node, err := runtime.store.CreateImportedNode(t.Context(), candidate.Node, identity)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	desktopInfraCandidates(response, httptest.NewRequest(http.MethodGet, desktopInfraBasePath+"/candidates", nil), s, cfg, runtime.store, runtime)
	if response.Code != http.StatusOK {
		t.Fatalf("candidates status=%d body=%s", response.Code, response.Body.String())
	}
	var result struct {
		Candidates []infra.Candidate `json:"candidates"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Node.Name != "Office TV" {
		t.Fatalf("visible candidates = %+v", result.Candidates)
	}
	snapshot, err := runtime.store.Snapshot(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Nodes) != 1 || snapshot.Nodes[0].ID != node.ID || len(snapshot.Sources) != len(names) {
		t.Fatalf("filter changed saved nodes or source observations: %+v", snapshot)
	}
	desktopInfraDecorateSnapshot(t.Context(), s, cfg, &snapshot)
	if len(snapshot.Nodes) != 0 {
		t.Fatal("untouched opaque-name import is still visible in the snapshot")
	}
	if _, err := runtime.store.GetNode(t.Context(), node.ID); err != nil {
		t.Fatalf("hidden node was not retained: %v", err)
	}
	devices, err := inventory.ListAllDevices(db)
	if err != nil || len(devices) != len(names) {
		t.Fatalf("filter changed registry devices: count=%d err=%v", len(devices), err)
	}
}

func TestDesktopInfraVisibleNodesPreservesEditedAndConnectedDevices(t *testing.T) {
	opaque := infra.Source{SourceIdentity: infra.SourceIdentity{Kind: "registry"}, Name: "02F3C3998E5A"}
	for _, tc := range []struct {
		name string
		edit func(*infra.Node, *infra.Snapshot)
	}{
		{"renamed", func(n *infra.Node, _ *infra.Snapshot) { n.Name = "Office TV" }},
		{"edited", func(n *infra.Node, _ *infra.Snapshot) { n.Version = 2 }},
		{"placed", func(n *infra.Node, _ *infra.Snapshot) {
			n.Placements = infra.Placements{infra.ViewTopology: {X: 1, Y: 2}}
		}},
		{"grouped", func(n *infra.Node, _ *infra.Snapshot) { n.ParentID = "room" }},
		{"outgoing edge", func(_ *infra.Node, s *infra.Snapshot) { s.Edges = []infra.Edge{{From: "device", To: "peer"}} }},
		{"incoming edge", func(_ *infra.Node, s *infra.Snapshot) { s.Edges = []infra.Edge{{From: "peer", To: "device"}} }},
		{"parent", func(_ *infra.Node, s *infra.Snapshot) { s.Nodes = []infra.Node{{ID: "child", ParentID: "device"}} }},
		{"local", func(n *infra.Node, _ *infra.Snapshot) { n.Sources = nil }},
		{"container", func(n *infra.Node, _ *infra.Snapshot) { n.Kind = infra.NodeContainer }},
		{"resolved source", func(n *infra.Node, _ *infra.Snapshot) {
			n.Sources = []infra.Source{{SourceIdentity: opaque.SourceIdentity, Name: "Office TV"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			node := infra.Node{ID: "device", Version: 1, Kind: infra.NodeDevice, Name: opaque.Name, Sources: []infra.Source{opaque}}
			snapshot := infra.Snapshot{}
			tc.edit(&node, &snapshot)
			snapshot.Nodes = append(snapshot.Nodes, node)
			got := desktopInfraVisibleNodes(&snapshot)
			if len(got) != len(snapshot.Nodes) {
				t.Fatalf("explicitly used device hidden: %+v", got)
			}
			if tc.name == "resolved source" && (got[0].Name != "Office TV" || snapshot.Nodes[0].Name != opaque.Name) {
				t.Fatal("resolved source name must display without changing the stored node")
			}
		})
	}
}
