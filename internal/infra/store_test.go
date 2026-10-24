package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/inventory"
)

func openInfraTestStore(t *testing.T) (*Store, *sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "inventory.db")
	db, err := inventory.InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	return store, db, path
}

func testNode(kind NodeKind, name, icon string) Node {
	return Node{Kind: kind, Name: name, Icon: icon, Profile: Profile{}, Placements: Placements{}}
}

func TestMigrationBacksUpInventoryBeforeAddingInfraSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inventory.db")
	db, err := inventory.InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	registryID, err := inventory.CreateDevice(db, "NAS", "nas", inventory.ProtocolSSH, "192.0.2.10", 22, "operator", "vault", "", "existing", []string{"storage"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(db); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(path + ".infra-v3-*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("migration backups = %v, %v; want one pre-migration backup", backups, err)
	}
	backupDB, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backupDB.Close()
	var count int
	if err := backupDB.QueryRow(`SELECT count(*) FROM devices WHERE id=?`, registryID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("backup inventory rows = %d, %v; want existing row", count, err)
	}
	var infraTables int
	if err := backupDB.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='infra_nodes'`).Scan(&infraTables); err != nil || infraTables != 0 {
		t.Fatalf("backup contains infra schema = %d, %v; want pre-migration database", infraTables, err)
	}
	var userVersion int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&userVersion); err != nil || userVersion != 4 {
		t.Fatalf("inventory user_version = %d, %v; want existing version 4 preserved", userVersion, err)
	}
}

func TestManualNodeCreatesCanonicalNonExecutableRegistryDevice(t *testing.T) {
	store, db, _ := openInfraTestStore(t)
	ctx := context.Background()
	node := testNode(NodeDevice, "Office NAS", "nas")
	node.Profile.IPAddress = "192.0.2.25"
	node.Profile.MACAddress = "02:00:00:00:00:25"
	node.Profile.Tags = []string{"storage", "office"}
	created, err := store.CreateManualNode(ctx, node)
	if err != nil {
		t.Fatal(err)
	}
	var name, protocol, ip, mac string
	if err := db.QueryRow(`SELECT name, protocol, ip_address, mac_address FROM devices WHERE id=?`, created.RegistryID).Scan(&name, &protocol, &ip, &mac); err != nil {
		t.Fatal(err)
	}
	if name != "Office NAS" || protocol != inventory.ProtocolNone || ip != node.Profile.IPAddress || mac != node.Profile.MACAddress {
		t.Fatalf("registry row = name %q protocol %q ip %q mac %q", name, protocol, ip, mac)
	}
	if len(created.Sources) != 1 || created.Sources[0].Kind != "registry" || created.Sources[0].Key != created.RegistryID {
		t.Fatalf("created registry source = %#v", created.Sources)
	}
	registry, err := inventory.GetDeviceByID(db, created.RegistryID)
	if err != nil || registry.Protocol != inventory.ProtocolNone {
		t.Fatalf("canonical device = %#v, %v", registry, err)
	}
	registry.Protocol = inventory.ProtocolSSH
	if err := inventory.UpdateDevice(db, registry); err != nil {
		t.Fatal(err)
	}
	created.Name = "Office NAS Mk II"
	created.Profile.Notes = "Located in the office rack"
	updated, err := store.UpdateNode(ctx, created)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT name, protocol FROM devices WHERE id=?`, created.RegistryID).Scan(&name, &protocol); err != nil {
		t.Fatal(err)
	}
	if name != "Office NAS" || protocol != inventory.ProtocolSSH {
		t.Fatalf("local Infra edit changed canonical row: name %q protocol %q", name, protocol)
	}
	if err := store.DeleteNode(ctx, updated.ID, updated.Version); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM devices WHERE id=?`, updated.RegistryID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("deleting Infra node removed registry row: count=%d err=%v", count, err)
	}
}

func TestNodeValidationHierarchyAndNetworkCycles(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	group, err := store.CreateNode(ctx, testNode(NodeGroup, "Lab", "group"))
	if err != nil {
		t.Fatal(err)
	}
	roomNode := testNode(NodeRoom, "Rack Room", "room")
	roomNode.ParentID = group.ID
	room, err := store.CreateNode(ctx, roomNode)
	if err != nil {
		t.Fatal(err)
	}
	cycle := group
	cycle.ParentID = room.ID
	if _, err := store.UpdateNode(ctx, cycle); !errors.Is(err, ErrValidation) {
		t.Fatalf("hierarchy cycle error = %v; want validation error", err)
	}
	unsafe := testNode(NodeService, "Admin", "web-service")
	unsafe.Profile.Links = []Link{{Label: "admin", URL: "https://user:password@example.test/"}}
	if _, err := store.CreateNode(ctx, unsafe); !errors.Is(err, ErrValidation) {
		t.Fatalf("unsafe link error = %v; want validation error", err)
	}
	left, err := store.CreateNode(ctx, testNode(NodeDevice, "Switch A", "switch"))
	if err != nil {
		t.Fatal(err)
	}
	right, err := store.CreateNode(ctx, testNode(NodeDevice, "Switch B", "switch"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateEdge(ctx, Edge{From: left.ID, To: "missing", Kind: EdgeEthernet}); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid edge endpoint error = %v; want validation error", err)
	}
	for _, pair := range [][2]string{{left.ID, right.ID}, {right.ID, left.ID}} {
		if _, err := store.CreateEdge(ctx, Edge{From: pair[0], To: pair[1], Kind: EdgeEthernet}); err != nil {
			t.Fatalf("network cycle edge %s -> %s: %v", pair[0], pair[1], err)
		}
	}
}

func TestNetworkSegmentProfileFieldsValidateAndPersist(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	network := testNode(NodeNetwork, "Office LAN", "network")
	network.Profile.CIDR = "192.168.40.0/24"
	network.Profile.VLAN = 40
	created, err := store.CreateNode(ctx, network)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := store.GetNode(ctx, created.ID)
	if err != nil || loaded.Profile.CIDR != "192.168.40.0/24" || loaded.Profile.VLAN != 40 {
		t.Fatalf("persisted network segment fields = CIDR %q VLAN %d, err %v", loaded.Profile.CIDR, loaded.Profile.VLAN, err)
	}

	for _, test := range []struct {
		name string
		cidr string
		vlan int
	}{
		{name: "invalid CIDR", cidr: "192.168.40.0/99", vlan: 40},
		{name: "invalid VLAN zero lower bound", cidr: "192.168.40.0/24", vlan: -1},
		{name: "invalid VLAN upper bound", cidr: "192.168.40.0/24", vlan: 4095},
	} {
		t.Run(test.name, func(t *testing.T) {
			node := testNode(NodeNetwork, "Invalid Segment", "network")
			node.Profile.CIDR = test.cidr
			node.Profile.VLAN = test.vlan
			if _, err := store.CreateNode(ctx, node); !errors.Is(err, ErrValidation) {
				t.Fatalf("CreateNode error = %v; want validation error", err)
			}
		})
	}

	identity := SourceIdentity{Kind: "docker", Instance: "network-host", Key: "segment-40"}
	observation := SourceObservation{Identity: identity, Name: "Office LAN", Observed: SourceObserved{
		Kind: NodeNetwork, Name: "Office LAN", Icon: "network", CIDR: "10.20.0.0/16", VLAN: 20,
	}}
	if err := store.ObserveSources(ctx, "docker", "network-host", []SourceObservation{observation}, true); err != nil {
		t.Fatal(err)
	}
	candidate, err := store.GetCandidate(ctx, CandidateID(identity))
	if err != nil || candidate.Node.Profile.CIDR != "10.20.0.0/16" || candidate.Node.Profile.VLAN != 20 {
		t.Fatalf("candidate segment fields = CIDR %q VLAN %d, err %v", candidate.Node.Profile.CIDR, candidate.Node.Profile.VLAN, err)
	}
	observation.Observed.VLAN = 4095
	if err := store.ObserveSources(ctx, "docker", "network-host", []SourceObservation{observation}, true); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid source VLAN error = %v; want validation error", err)
	}
}

func TestNodeRuntimeProjectionFieldsAreNotTrustedOrPersisted(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	node := testNode(NodeDevice, "Observed Device", "generic-device")
	node.AvailableActions = []string{"reboot", "delete-all"}
	node.Power = PowerRunning
	node.AppLinks = []AppLink{{App: "terminal", TargetID: "arbitrary-target"}}
	node.ActionReasons = map[string]string{"restart": "forged"}
	node.SourceActions = map[string][]string{"docker:host": {"restart", "delete"}}
	node.SourceActionReasons = map[string]map[string]string{"docker:host": {"restart": "forged"}}
	node.Profile.AppBindings = []AppLink{{App: "terminal", TargetID: "terminal-1"}}
	created, err := store.CreateNode(ctx, node)
	if err != nil {
		t.Fatal(err)
	}
	if len(created.AvailableActions) != 0 || created.AvailableActions == nil || created.Power != "" || len(created.AppLinks) != 0 || created.AppLinks == nil || created.ActionReasons != nil || created.SourceActions != nil || created.SourceActionReasons != nil {
		t.Fatalf("untrusted node projection fields were returned: %#v", created)
	}
	loaded, err := store.GetNode(ctx, created.ID)
	if err != nil || loaded.AvailableActions == nil || len(loaded.AvailableActions) != 0 || loaded.Power != "" || loaded.AppLinks == nil || len(loaded.AppLinks) != 0 {
		t.Fatalf("stored node projection fields = actions %#v app links %#v, err %v", loaded.AvailableActions, loaded.AppLinks, err)
	}
	if len(loaded.Profile.AppBindings) != 1 || loaded.Profile.AppBindings[0] != (AppLink{App: "terminal", TargetID: "terminal-1"}) {
		t.Fatalf("persisted app bindings = %#v", loaded.Profile.AppBindings)
	}
	loaded.AvailableActions = []string{"shutdown"}
	loaded.Power = PowerStopped
	loaded.AppLinks = []AppLink{{App: "terminal", TargetID: "forged"}}
	loaded.ActionReasons = map[string]string{"shutdown": "forged"}
	loaded.SourceActions = map[string][]string{"inventory:local": {"wake"}}
	loaded.SourceActionReasons = map[string]map[string]string{"inventory:local": {"wake": "forged"}}
	updated, err := store.UpdateNode(ctx, loaded)
	if err != nil || len(updated.AvailableActions) != 0 || updated.Power != "" || len(updated.AppLinks) != 0 || updated.ActionReasons != nil || updated.SourceActions != nil || updated.SourceActionReasons != nil {
		t.Fatalf("updated runtime fields = actions %#v app links %#v reasons %#v source actions %#v source reasons %#v, err %v", updated.AvailableActions, updated.AppLinks, updated.ActionReasons, updated.SourceActions, updated.SourceActionReasons, err)
	}
	loaded, err = store.GetNode(ctx, created.ID)
	if err != nil || len(loaded.Profile.AppBindings) != 1 || loaded.Profile.AppBindings[0].TargetID != "terminal-1" {
		t.Fatalf("updated app bindings = %#v, err %v", loaded.Profile.AppBindings, err)
	}
	invalid := testNode(NodeDevice, "Duplicate Bindings", "generic-device")
	invalid.Profile.AppBindings = []AppLink{{App: "terminal", TargetID: "one"}, {App: "terminal", TargetID: "one"}}
	if _, err := store.CreateNode(ctx, invalid); !errors.Is(err, ErrValidation) {
		t.Fatalf("duplicate app binding error = %v; want validation error", err)
	}
	snapshot, err := store.Snapshot(ctx)
	if err != nil || len(snapshot.Nodes) != 1 || snapshot.Nodes[0].AvailableActions == nil || snapshot.Nodes[0].AppLinks == nil {
		t.Fatalf("snapshot projection arrays = %#v, err %v", snapshot.Nodes, err)
	}
}

func TestObservedPowerStaysOutOfTheImportedProfile(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	identity := SourceIdentity{Kind: "docker", Instance: "engine", Key: "container-1"}
	rejected := SourceObservation{Identity: identity, Name: "web", Observed: SourceObserved{
		Kind: NodeContainer, Name: "web", Icon: "container", Power: "on", Tags: []string{}, Interfaces: []NetworkInterface{},
	}}
	if err := store.ObserveSources(ctx, identity.Kind, identity.Instance, []SourceObservation{rejected}, true); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid power error = %v; want validation error", err)
	}
	observation := SourceObservation{Identity: identity, Name: "web", Observed: SourceObserved{
		Kind: NodeContainer, Name: "web", Icon: "container", Model: "nginx:latest", Power: PowerStopped,
		Tags: []string{}, Interfaces: []NetworkInterface{},
	}}
	if err := store.ObserveSources(ctx, identity.Kind, identity.Instance, []SourceObservation{observation}, true); err != nil {
		t.Fatal(err)
	}
	candidate, err := store.GetCandidate(ctx, CandidateID(identity))
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Node.Profile.Role != "" || candidate.Node.Profile.Model != "nginx:latest" || candidate.Node.Power != "" {
		t.Fatalf("imported profile = role %q model %q node power %q", candidate.Node.Profile.Role, candidate.Node.Profile.Model, candidate.Node.Power)
	}
	if candidate.Source.Observed.Power != PowerStopped {
		t.Fatalf("observed power = %q", candidate.Source.Observed.Power)
	}
}

func TestLayoutCASIsAtomicAcrossNodes(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	a, err := store.CreateNode(ctx, testNode(NodeDevice, "A", "generic-device"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := store.CreateNode(ctx, testNode(NodeDevice, "B", "generic-device"))
	if err != nil {
		t.Fatal(err)
	}
	request := LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{
		{ID: a.ID, Version: a.Version, X: 15, Y: 20},
		{ID: b.ID, Version: b.Version + 1, X: 30, Y: 40},
	}}
	if _, err := store.SaveLayout(ctx, request); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale layout save error = %v; want conflict", err)
	}
	after, err := store.GetNode(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Version != a.Version || len(after.Placements) != 0 {
		t.Fatalf("partial layout commit changed first node: %#v", after)
	}
	request.Nodes[1].Version = b.Version
	request.Nodes[1].Pinned = true
	updated, err := store.SaveLayout(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated) != 2 || updated[0].Version != a.Version+1 || updated[1].Version != b.Version+1 || !updated[1].Placements[ViewAtlas].Pinned {
		t.Fatalf("saved layout result = %#v", updated)
	}
}

func TestSourceObservationsCandidatesAndRetention(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	identity := SourceIdentity{Kind: "docker", Instance: "host-a", Key: "sha256:111"}
	first := SourceObservation{Identity: identity, Name: "web", Observed: SourceObserved{Kind: NodeContainer, Name: "web", Icon: "container", Protected: true, Host: "pve-node-1"}}
	second := SourceObservation{Identity: SourceIdentity{Kind: "docker", Instance: "host-a", Key: "sha256:222"}, Name: "db", Observed: SourceObserved{Kind: NodeContainer, Name: "db", Icon: "container"}}
	if err := store.ObserveSources(ctx, "docker", "host-a", []SourceObservation{first, second}, true); err != nil {
		t.Fatal(err)
	}
	candidateID := CandidateID(identity)
	candidate, err := store.GetCandidate(ctx, candidateID)
	if err != nil || candidate.ID != candidateID || candidate.Node.Name != "web" {
		t.Fatalf("candidate = %#v, %v", candidate, err)
	}
	if !candidate.Source.Observed.Protected || candidate.Source.Observed.Host != "pve-node-1" {
		t.Fatalf("source metadata was not preserved through candidate resolution: %#v", candidate.Source.Observed)
	}
	imported, err := store.CreateImportedNode(ctx, candidate.Node, identity)
	if err != nil {
		t.Fatal(err)
	}
	if len(imported.Sources) != 1 || imported.Sources[0].Key != identity.Key {
		t.Fatalf("imported source link = %#v", imported.Sources)
	}
	if _, err := store.GetCandidate(ctx, candidateID); !errors.Is(err, ErrSourceLinked) {
		t.Fatalf("linked source candidate error = %v; want linked", err)
	}
	if err := store.ObserveSources(ctx, "docker", "host-a", []SourceObservation{first}, false); err != nil {
		t.Fatal(err)
	}
	seen, err := store.GetSource(ctx, identity)
	if err != nil || seen.State != SourceCurrent {
		t.Fatalf("observed source state = %q, %v; want current", seen.State, err)
	}
	missing, err := store.GetSource(ctx, second.Identity)
	if err != nil || missing.State != SourceStale {
		t.Fatalf("source omitted by incomplete enumeration state = %q, %v; want stale", missing.State, err)
	}
	if err := store.ObserveSources(ctx, "docker", "host-a", []SourceObservation{first}, true); err != nil {
		t.Fatal(err)
	}
	missing, err = store.GetSource(ctx, second.Identity)
	if err != nil || missing.State != SourceRemoved || missing.RemovedAt == nil {
		t.Fatalf("source omitted by complete enumeration = %#v, %v; want removed", missing, err)
	}
	if _, err := store.GetCandidate(ctx, CandidateID(second.Identity)); !errors.Is(err, ErrSourceRemoved) {
		t.Fatalf("removed candidate resolution error = %v", err)
	}
	pruned, err := store.PruneSources(ctx, time.Now().UTC().Add(time.Second))
	if err != nil || pruned != 1 {
		t.Fatalf("pruned removed, unlinked sources = %d, %v; want 1", pruned, err)
	}
	if _, err := store.GetSource(ctx, second.Identity); !errors.Is(err, ErrSourceNotFound) {
		t.Fatalf("expired source lookup error = %v", err)
	}
	if _, err := store.GetSource(ctx, identity); err != nil {
		t.Fatalf("retention removed a linked source: %v", err)
	}
	if _, err := store.GetCandidate(ctx, "infra_not-a-valid-candidate"); !errors.Is(err, ErrValidation) {
		t.Fatalf("malformed candidate id error = %v", err)
	}
}

func TestRegistryLinksSupportDevicesVMsAndContainersWithVersionCAS(t *testing.T) {
	store, db, _ := openInfraTestStore(t)
	ctx := context.Background()
	kinds := []NodeKind{NodeDevice, NodeVM, NodeContainer}
	identities := make([]SourceIdentity, len(kinds))
	observations := make([]SourceObservation, len(kinds))
	registryIDs := make([]string, len(kinds))
	for i := range kinds {
		name := fmt.Sprintf("registry-%d", i)
		registryID, err := inventory.CreateDevice(db, name, "server", inventory.ProtocolSSH, "192.0.2.10", 22, "operator", "vault-secret", "credential-id", "preserve credentials", []string{"managed"}, "")
		if err != nil {
			t.Fatal(err)
		}
		registryIDs[i] = registryID
		identities[i] = SourceIdentity{Kind: "registry", Instance: "inventory", Key: registryID}
		observations[i] = SourceObservation{
			Identity: identities[i], Name: name,
			Observed: SourceObserved{Kind: NodeDevice, Name: name, Icon: "generic-device"},
		}
	}
	if err := store.ObserveSources(ctx, "registry", "inventory", observations, true); err != nil {
		t.Fatal(err)
	}
	for i, kind := range kinds {
		node, err := store.CreateNode(ctx, testNode(kind, fmt.Sprintf("Infra %s", kind), "generic-device"))
		if err != nil {
			t.Fatal(err)
		}
		linked, err := store.LinkSource(ctx, node.ID, identities[i])
		if err != nil {
			t.Fatalf("link registry source to %s: %v", kind, err)
		}
		if linked.RegistryID != registryIDs[i] || linked.Version != node.Version+1 || len(linked.Sources) != 1 {
			t.Fatalf("linked %s node = registry %q version %d sources %d; want %q version %d and one source", kind, linked.RegistryID, linked.Version, len(linked.Sources), registryIDs[i], node.Version+1)
		}
		linked.Name += " edited"
		linked.Profile.Notes = "Infra edit must not erase registry credentials"
		updated, err := store.UpdateNode(ctx, linked)
		if err != nil || updated.Kind != kind || updated.Version != linked.Version+1 {
			t.Fatalf("edit linked %s node = %#v, %v", kind, updated, err)
		}
		registry, err := inventory.GetDeviceByID(db, registryIDs[i])
		if err != nil || registry.Protocol != inventory.ProtocolSSH || registry.VaultSecretID != "vault-secret" || registry.CredentialID != "credential-id" {
			t.Fatalf("registry credentials changed after linking %s: %#v, %v", kind, registry, err)
		}
	}

	// A distinct source identity for the same inventory device cannot bypass
	// the one-to-one registry association constraint.
	aliasIdentity := SourceIdentity{Kind: "registry", Instance: "inventory-alias", Key: registryIDs[0]}
	if err := store.ObserveSources(ctx, aliasIdentity.Kind, aliasIdentity.Instance, []SourceObservation{{
		Identity: aliasIdentity, Name: "registry alias", Observed: SourceObserved{Kind: NodeDevice, Name: "registry alias", Icon: "generic-device"},
	}}, true); err != nil {
		t.Fatal(err)
	}
	secondVM, err := store.CreateNode(ctx, testNode(NodeVM, "Second VM", "vm"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.LinkSource(ctx, secondVM.ID, aliasIdentity); err == nil {
		t.Fatal("second node linked an already associated registry device")
	}
	unchanged, err := store.GetNode(ctx, secondVM.ID)
	if err != nil || unchanged.Version != secondVM.Version || unchanged.RegistryID != "" {
		t.Fatalf("failed registry association changed node: %#v, %v", unchanged, err)
	}
	if _, err := store.GetCandidate(ctx, CandidateID(aliasIdentity)); err != nil {
		t.Fatalf("failed association consumed source candidate: %v", err)
	}

	service, err := store.CreateNode(ctx, testNode(NodeService, "Not a registry target", "web-service"))
	if err != nil {
		t.Fatal(err)
	}
	serviceRegistryID, err := inventory.CreateDevice(db, "service target", "server", inventory.ProtocolSSH, "192.0.2.30", 22, "operator", "vault-service", "credential-service", "", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	serviceIdentity := SourceIdentity{Kind: "registry", Instance: "inventory", Key: serviceRegistryID}
	if err := store.ObserveSources(ctx, "registry", "inventory", []SourceObservation{{
		Identity: serviceIdentity, Name: "service target", Observed: SourceObserved{Kind: NodeDevice, Name: "service target", Icon: "generic-device"},
	}}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LinkSource(ctx, service.ID, serviceIdentity); !errors.Is(err, ErrValidation) {
		t.Fatalf("registry link to service error = %v; want validation error", err)
	}
	unchanged, err = store.GetNode(ctx, service.ID)
	if err != nil || unchanged.Version != service.Version || unchanged.RegistryID != "" {
		t.Fatalf("rejected service registry link changed node: %#v, %v", unchanged, err)
	}

	// Source links that do not carry a registry association still change the
	// node's CAS version because the node's source list changed.
	placementIdentity := SourceIdentity{Kind: "proxmox", Instance: "pve-1", Key: "vm:104"}
	if err := store.ObserveSources(ctx, "proxmox", "pve-1", []SourceObservation{{
		Identity: placementIdentity, Name: "Guest 104", Observed: SourceObserved{Kind: NodeVM, Name: "Guest 104", Icon: "vm"},
	}}, true); err != nil {
		t.Fatal(err)
	}
	plainNode, err := store.CreateNode(ctx, testNode(NodeVM, "Guest 104", "vm"))
	if err != nil {
		t.Fatal(err)
	}
	linked, err := store.LinkSource(ctx, plainNode.ID, placementIdentity)
	if err != nil || linked.Version != plainNode.Version+1 {
		t.Fatalf("non-registry source link version = %d, err %v; want %d", linked.Version, err, plainNode.Version+1)
	}
}

func TestActionLedgerDeduplicatesAndMarksInterruptedWriteUnknown(t *testing.T) {
	store, db, _ := openInfraTestStore(t)
	ctx := context.Background()
	request := ActionRequest{RequestID: "request-1", Fingerprint: fmt.Sprintf("%064x", 1), Action: "restart"}
	first, fresh, err := store.ReserveAction(ctx, request)
	if err != nil || !fresh || first.Status != ActionRunning {
		t.Fatalf("first reservation = %#v, fresh=%t, err=%v", first, fresh, err)
	}
	replay, fresh, err := store.ReserveAction(ctx, request)
	if err != nil || fresh || replay.Status != ActionRunning {
		t.Fatalf("duplicate reservation = %#v, fresh=%t, err=%v", replay, fresh, err)
	}
	otherAction := request
	otherAction.Action = "stop"
	if _, _, err := store.ReserveAction(ctx, otherAction); !errors.Is(err, ErrActionConflict) {
		t.Fatalf("request id reuse error = %v", err)
	}
	finished, err := store.FinishAction(ctx, request.RequestID, request.Fingerprint, ActionCompleted, "UPID:host:task")
	if err != nil || finished.Status != ActionCompleted || finished.UPID != "UPID:host:task" {
		t.Fatalf("finished action = %#v, %v", finished, err)
	}
	acceptedRequest := ActionRequest{RequestID: "request-accepted", Fingerprint: fmt.Sprintf("%064x", 3), Action: "wake"}
	if _, fresh, err := store.ReserveAction(ctx, acceptedRequest); err != nil || !fresh {
		t.Fatalf("accepted reservation fresh=%t err=%v", fresh, err)
	}
	accepted, err := store.FinishAction(ctx, acceptedRequest.RequestID, acceptedRequest.Fingerprint, ActionAccepted, "")
	if err != nil || accepted.Status != ActionAccepted || accepted.UPID != "" {
		t.Fatalf("accepted action = %#v, %v", accepted, err)
	}
	proxmoxRequest := ActionRequest{RequestID: "request-proxmox", Fingerprint: fmt.Sprintf("%064x", 4), Action: "start"}
	if _, fresh, err := store.ReserveAction(ctx, proxmoxRequest); err != nil || !fresh {
		t.Fatalf("proxmox reservation fresh=%t err=%v", fresh, err)
	}
	proxmoxAccepted, err := store.FinishAction(ctx, proxmoxRequest.RequestID, proxmoxRequest.Fingerprint, ActionAccepted, "UPID:node:task")
	if err != nil || proxmoxAccepted.Status != ActionAccepted || proxmoxAccepted.UPID != "UPID:node:task" {
		t.Fatalf("accepted Proxmox action = %#v, %v", proxmoxAccepted, err)
	}
	interrupted := ActionRequest{RequestID: "request-2", Fingerprint: fmt.Sprintf("%064x", 2), Action: "wake"}
	if _, fresh, err := store.ReserveAction(ctx, interrupted); err != nil || !fresh {
		t.Fatalf("interrupted reservation fresh=%t err=%v", fresh, err)
	}
	if _, err := New(db); err != nil {
		t.Fatal(err)
	}
	unknown, fresh, err := store.ReserveAction(ctx, interrupted)
	if err != nil || fresh || unknown.Status != ActionUnknown {
		t.Fatalf("restart reservation = %#v, fresh=%t, err=%v; want persisted unknown", unknown, fresh, err)
	}
	acceptedReplay, fresh, err := store.ReserveAction(ctx, acceptedRequest)
	if err != nil || fresh || acceptedReplay.Status != ActionAccepted {
		t.Fatalf("accepted restart replay = %#v, fresh=%t, err=%v", acceptedReplay, fresh, err)
	}
	proxmoxReplay, fresh, err := store.ReserveAction(ctx, proxmoxRequest)
	if err != nil || fresh || proxmoxReplay.Status != ActionAccepted || proxmoxReplay.UPID != "UPID:node:task" {
		t.Fatalf("Proxmox accepted restart replay = %#v, fresh=%t, err=%v", proxmoxReplay, fresh, err)
	}
}

func TestActionLedgerConcurrentReservationsHaveOneDispatcher(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	request := ActionRequest{RequestID: "concurrent-request", Fingerprint: fmt.Sprintf("%064x", 20), Action: "restart"}
	const callers = 12
	start := make(chan struct{})
	results := make(chan struct {
		fresh bool
		err   error
	}, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, fresh, err := store.ReserveAction(context.Background(), request)
			results <- struct {
				fresh bool
				err   error
			}{fresh: fresh, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	freshCount := 0
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent reservation: %v", result.err)
		}
		if result.fresh {
			freshCount++
		}
	}
	if freshCount != 1 {
		t.Fatalf("fresh reservation winners = %d; want exactly one", freshCount)
	}
}

func TestActionLedgerV1UpgradePreservesResultsAndMakesPendingUnknown(t *testing.T) {
	_, db, path := openInfraTestStore(t)
	if _, err := db.Exec(`
		DROP INDEX idx_infra_actions_updated;
		DROP TABLE infra_action_requests;
		CREATE TABLE infra_action_requests (
			request_id TEXT PRIMARY KEY,
			fingerprint TEXT NOT NULL,
			action TEXT NOT NULL,
			status TEXT NOT NULL CHECK(status IN ('accepted','completed','failed','unknown')),
			upid TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE INDEX idx_infra_actions_updated ON infra_action_requests(updated_at);
		INSERT INTO infra_action_requests(request_id,fingerprint,action,status,upid,created_at,updated_at)
			VALUES('pending','000000000000000000000000000000000000000000000000000000000000000a','wake','accepted','','now','now');
		INSERT INTO infra_action_requests(request_id,fingerprint,action,status,upid,created_at,updated_at)
			VALUES('done','000000000000000000000000000000000000000000000000000000000000000b','restart','completed','UPID:x','now','now');
		UPDATE infra_meta SET version=1 WHERE id=1;
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := New(db); err != nil {
		t.Fatal(err)
	}
	var pendingStatus, doneStatus, upid string
	if err := db.QueryRow(`SELECT status FROM infra_action_requests WHERE request_id='pending'`).Scan(&pendingStatus); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status,upid FROM infra_action_requests WHERE request_id='done'`).Scan(&doneStatus, &upid); err != nil {
		t.Fatal(err)
	}
	if pendingStatus != ActionUnknown || doneStatus != ActionCompleted || upid != "UPID:x" {
		t.Fatalf("migrated statuses pending=%q done=%q upid=%q", pendingStatus, doneStatus, upid)
	}
	backups, err := filepath.Glob(path + ".infra-v3-*.bak")
	if err != nil || len(backups) != 2 {
		t.Fatalf("backups after fresh init and schema upgrade = %v, %v; want both pre-migration copies", backups, err)
	}
}

func TestSingleSourceUpsertKeepsSiblingObservationsCurrent(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	first := SourceObservation{Identity: SourceIdentity{Kind: "registry", Instance: "inventory", Key: "registry-1"}, Name: "alpha", Observed: SourceObserved{Kind: NodeDevice, Name: "alpha", Icon: "generic-device"}}
	second := SourceObservation{Identity: SourceIdentity{Kind: "registry", Instance: "inventory", Key: "registry-2"}, Name: "beta", Observed: SourceObserved{Kind: NodeDevice, Name: "beta", Icon: "generic-device"}}
	if err := store.ObserveSources(ctx, "registry", "inventory", []SourceObservation{first, second}, true); err != nil {
		t.Fatal(err)
	}
	renamed := first
	renamed.Observed.Name = "alpha renamed"
	renamed.Name = "alpha renamed"
	if err := store.UpsertObservation(ctx, renamed); err != nil {
		t.Fatal(err)
	}
	for _, identity := range []SourceIdentity{first.Identity, second.Identity} {
		source, err := store.GetSource(ctx, identity)
		if err != nil {
			t.Fatal(err)
		}
		if source.State != SourceCurrent {
			t.Fatalf("source %q state = %q after a single-item upsert; want current", identity.Key, source.State)
		}
	}
	updated, err := store.GetSource(ctx, first.Identity)
	if err != nil || updated.Observed.Name != "alpha renamed" {
		t.Fatalf("upserted observation = %#v, %v", updated.Observed, err)
	}
}

func TestUpdateNodeRefusesANonContainerKindWithChildren(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	container, err := store.CreateNode(ctx, testNode(NodeGroup, "Rack area", "group"))
	if err != nil {
		t.Fatal(err)
	}
	child := testNode(NodeDevice, "Access switch", "switch")
	child.ParentID = container.ID
	if _, err := store.CreateNode(ctx, child); err != nil {
		t.Fatal(err)
	}
	container.Kind = NodeDevice
	if _, err := store.UpdateNode(ctx, container); !errors.Is(err, ErrHasChildren) {
		t.Fatalf("kind change of a parent node error = %v; want ErrHasChildren", err)
	}
	empty, err := store.CreateNode(ctx, testNode(NodeGroup, "Empty area", "group"))
	if err != nil {
		t.Fatal(err)
	}
	empty.Kind = NodeDevice
	if _, err := store.UpdateNode(ctx, empty); err != nil {
		t.Fatalf("kind change of a childless group was refused: %v", err)
	}
}

func TestSnapshotOmitsUnlinkedRemovedSources(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	linked := SourceObservation{Identity: SourceIdentity{Kind: "docker", Instance: "host-a", Key: "sha256:linked"}, Name: "web", Observed: SourceObserved{Kind: NodeContainer, Name: "web", Icon: "container"}}
	orphan := SourceObservation{Identity: SourceIdentity{Kind: "docker", Instance: "host-a", Key: "sha256:orphan"}, Name: "old", Observed: SourceObserved{Kind: NodeContainer, Name: "old", Icon: "container"}}
	if err := store.ObserveSources(ctx, "docker", "host-a", []SourceObservation{linked, orphan}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateImportedNode(ctx, Node{
		Kind: NodeContainer, Name: "web", Icon: "container",
		Profile: Profile{Tags: []string{}, Interfaces: []NetworkInterface{}, Links: []Link{}, AppBindings: []AppLink{}, Custom: map[string]string{}},
	}, linked.Identity); err != nil {
		t.Fatal(err)
	}
	// Both observations disappear from the source: the linked one stays visible as
	// removed, the unlinked one is retained only for the retention sweep.
	if err := store.ObserveSources(ctx, "docker", "host-a", []SourceObservation{
		{Identity: SourceIdentity{Kind: "docker", Instance: "host-a", Key: "sha256:new"}, Name: "new", Observed: SourceObserved{Kind: NodeContainer, Name: "new", Icon: "container"}},
	}, true); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	keys := make(map[string]SourceState, len(snapshot.Sources))
	for _, source := range snapshot.Sources {
		keys[source.Key] = source.State
	}
	if state, ok := keys["sha256:orphan"]; ok {
		t.Fatalf("unlinked removed source reached the snapshot with state %q", state)
	}
	if state := keys["sha256:linked"]; state != SourceRemoved {
		t.Fatalf("linked removed source state = %q; want removed", state)
	}
	retained, err := store.ListSources(ctx, "docker", "host-a")
	if err != nil || len(retained) != 3 {
		t.Fatalf("retained source rows = %d, %v; want the orphan kept for retention", len(retained), err)
	}
}

func TestActionReplayHealsAnInterruptedRunningRow(t *testing.T) {
	store, db, _ := openInfraTestStore(t)
	ctx := context.Background()
	request := ActionRequest{RequestID: "heal-1", Fingerprint: strings.Repeat("a", 64), Action: "start"}
	if _, fresh, err := store.ReserveAction(ctx, request); err != nil || !fresh {
		t.Fatalf("reservation = %v, %v; want a fresh reservation", fresh, err)
	}
	stale := formatTime(time.Now().UTC().Add(-actionRunningTimeout - time.Minute))
	if _, err := db.Exec(`UPDATE infra_action_requests SET updated_at=? WHERE request_id=?`, stale, request.RequestID); err != nil {
		t.Fatal(err)
	}
	record, err := store.GetAction(ctx, request.RequestID)
	if err != nil || record.Status != ActionUnknown {
		t.Fatalf("abandoned action read = %#v, %v; want unknown", record, err)
	}
	replay, fresh, err := store.ReserveAction(ctx, request)
	if err != nil || fresh || replay.Status != ActionUnknown {
		t.Fatalf("replayed abandoned action = %#v fresh=%v err=%v; want a terminal unknown replay", replay, fresh, err)
	}
}

func TestLinkSourceRejectsARegistryDeviceClaimedByAnotherNode(t *testing.T) {
	store, db, _ := openInfraTestStore(t)
	ctx := context.Background()
	registryID, err := inventory.CreateDevice(db, "alpha", "server", inventory.ProtocolSSH, "192.0.2.10", 22, "operator", "vault", "", "note", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	identity := SourceIdentity{Kind: "registry", Instance: "inventory", Key: registryID}
	observation := SourceObservation{Identity: identity, Name: "alpha", Observed: SourceObserved{Kind: NodeDevice, Name: "alpha", Icon: "server"}}
	if err := store.ObserveSources(ctx, "registry", "inventory", []SourceObservation{observation}, true); err != nil {
		t.Fatal(err)
	}
	first, err := store.CreateNode(ctx, testNode(NodeDevice, "first", "server"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.LinkSource(ctx, first.ID, identity); err != nil {
		t.Fatal(err)
	}
	second, err := store.CreateNode(ctx, testNode(NodeDevice, "second", "server"))
	if err != nil {
		t.Fatal(err)
	}
	// Simulate an inconsistent row: the source link is gone while the first node
	// still owns the registry device. The unique index must surface as a conflict.
	if _, err := db.Exec(`DELETE FROM infra_sources WHERE kind='registry' AND source_key=?`, registryID); err != nil {
		t.Fatal(err)
	}
	if err := store.ObserveSources(ctx, "registry", "inventory", []SourceObservation{observation}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LinkSource(ctx, second.ID, identity); !errors.Is(err, ErrSourceLinked) {
		t.Fatalf("second registry link error = %v; want ErrSourceLinked", err)
	}
}

func TestMigrationBackupRetentionKeepsTheNewestCopies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inventory.db")
	now := time.Now()
	for index := 0; index < 5; index++ {
		backup := fmt.Sprintf("%s.infra-v2-backup%d.bak", path, index)
		if err := os.WriteFile(backup, []byte("copy"), 0o600); err != nil {
			t.Fatal(err)
		}
		stamp := now.Add(-time.Duration(index) * time.Minute)
		if err := os.Chtimes(backup, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	pruneMigrationBackups(path)
	matches, err := filepath.Glob(path + ".infra-v*.bak")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != migrationBackupRetention {
		t.Fatalf("retained backups = %d (%v); want %d", len(matches), matches, migrationBackupRetention)
	}
	for _, removed := range []string{path + ".infra-v2-backup3.bak", path + ".infra-v2-backup4.bak"} {
		if _, err := os.Stat(removed); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("old backup %s was not pruned: %v", removed, err)
		}
	}
}

func TestSchemaV3MigrationKeepsV2RowsAndAddsIgnoreAndSourceHosts(t *testing.T) {
	store, db, path := openInfraTestStore(t)
	ctx := context.Background()
	node, err := store.CreateNode(ctx, testNode(NodeDevice, "Docker host", "server"))
	if err != nil {
		t.Fatal(err)
	}
	identity := SourceIdentity{Kind: "docker", Instance: "docker_engine", Key: "container-1"}
	if err := store.ObserveSources(ctx, identity.Kind, identity.Instance, []SourceObservation{{
		Identity: identity, Name: "web", Observed: SourceObserved{Kind: NodeContainer, Name: "web", Icon: "container"},
	}}, true); err != nil {
		t.Fatal(err)
	}
	// Recreate the exact v2 shape with rows: no ignore column, no source host table.
	if _, err := db.Exec(`
		DROP TABLE infra_source_hosts;
		ALTER TABLE infra_sources DROP COLUMN ignored_at;
		UPDATE infra_meta SET version=2 WHERE id=1;
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := New(db); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := db.QueryRow(`SELECT version FROM infra_meta WHERE id=1`).Scan(&version); err != nil || version != 3 {
		t.Fatalf("schema version = %d, %v; want 3", version, err)
	}
	var ignoredAt string
	if err := db.QueryRow(`SELECT ignored_at FROM infra_sources WHERE kind=? AND instance=? AND source_key=?`,
		identity.Kind, identity.Instance, identity.Key).Scan(&ignoredAt); err != nil || ignoredAt != "" {
		t.Fatalf("migrated source ignored_at = %q, %v; want empty", ignoredAt, err)
	}
	if _, err := db.Exec(`INSERT INTO infra_source_hosts(kind, instance, node_id, version, updated_at) VALUES('docker','docker_engine',?,1,'now')`, node.ID); err != nil {
		t.Fatalf("source host table after migration: %v", err)
	}
	if loaded, err := store.GetNode(ctx, node.ID); err != nil || loaded.Name != "Docker host" {
		t.Fatalf("migrated node = %#v, %v", loaded, err)
	}
	backups, err := filepath.Glob(path + ".infra-v3-*.bak")
	if err != nil || len(backups) != 2 {
		t.Fatalf("backups = %v, %v; want the fresh-init and the v2 upgrade copies", backups, err)
	}
	if _, err := New(db); err != nil {
		t.Fatalf("reopening a v3 schema: %v", err)
	}
}

func TestPlacementSizeIsBoundedAndOnlyForPlaces(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	room := testNode(NodeRoom, "Living room", "room")
	room.Placements = Placements{ViewAtlas: {X: 10, Y: 20, W: 360, H: 240}}
	created, err := store.CreateNode(ctx, room)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := store.GetNode(ctx, created.ID)
	if err != nil || loaded.Placements[ViewAtlas].W != 360 || loaded.Placements[ViewAtlas].H != 240 {
		t.Fatalf("stored place size = %#v, %v", loaded.Placements, err)
	}
	for _, tc := range []struct {
		name      string
		kind      NodeKind
		placement Placement
	}{
		{"device with size", NodeDevice, Placement{W: 360, H: 240}},
		{"too small", NodeRoom, Placement{W: 119, H: 240}},
		{"too large", NodeRack, Placement{W: 360, H: 20001}},
		{"width only", NodeGroup, Placement{W: 360}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			node := testNode(tc.kind, "Sized", "group")
			node.Placements = Placements{ViewAtlas: tc.placement}
			if _, err := store.CreateNode(ctx, node); !errors.Is(err, ErrValidation) {
				t.Fatalf("CreateNode error = %v; want validation error", err)
			}
		})
	}
	// A sized place that becomes a device keeps its position and loses its size.
	loaded.Kind = NodeDevice
	loaded.Icon = "generic-device"
	changed, err := store.UpdateNode(ctx, loaded)
	if err != nil {
		t.Fatalf("kind change of a sized place: %v", err)
	}
	if got := changed.Placements[ViewAtlas]; got.W != 0 || got.H != 0 || got.X != 10 || got.Y != 20 {
		t.Fatalf("placement after kind change = %#v; want position kept and size dropped", got)
	}
}

func TestLayoutBatchChangesParentAndPlaceSize(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	room, err := store.CreateNode(ctx, testNode(NodeRoom, "Office", "room"))
	if err != nil {
		t.Fatal(err)
	}
	rackNode := testNode(NodeRack, "Rack", "rack")
	rackNode.ParentID = room.ID
	rack, err := store.CreateNode(ctx, rackNode)
	if err != nil {
		t.Fatal(err)
	}
	device, err := store.CreateNode(ctx, testNode(NodeDevice, "Switch", "switch"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.CreateNode(ctx, testNode(NodeDevice, "NAS", "nas"))
	if err != nil {
		t.Fatal(err)
	}
	text := func(value string) *string { return &value }
	size := func(value float64) *float64 { return &value }

	updated, err := store.SaveLayout(ctx, LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{
		{ID: room.ID, Version: room.Version, X: 0, Y: 0, W: size(600), H: size(400)},
		{ID: device.ID, Version: device.Version, X: 48, Y: 72, ParentID: text(room.ID)},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if updated[0].Placements[ViewAtlas].W != 600 || updated[0].Placements[ViewAtlas].H != 400 || updated[1].ParentID != room.ID {
		t.Fatalf("layout result = %#v", updated)
	}
	// Absent w/h keep the stored size; absent parent_id keeps the parent.
	moved, err := store.SaveLayout(ctx, LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{
		{ID: room.ID, Version: updated[0].Version, X: 24, Y: 24},
		{ID: device.ID, Version: updated[1].Version, X: 96, Y: 96},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if moved[0].Placements[ViewAtlas].H != 400 || moved[1].ParentID != room.ID {
		t.Fatalf("absent layout fields changed state: %#v", moved)
	}
	for _, tc := range []struct {
		name string
		item LayoutItem
	}{
		{"cycle", LayoutItem{ID: room.ID, Version: moved[0].Version, ParentID: text(rack.ID)}},
		{"non-container parent", LayoutItem{ID: other.ID, Version: other.Version, ParentID: text(device.ID)}},
		{"missing parent", LayoutItem{ID: other.ID, Version: other.Version, ParentID: text("missing")}},
		{"size on a device", LayoutItem{ID: other.ID, Version: other.Version, W: size(300), H: size(200)}},
		{"place too small", LayoutItem{ID: rack.ID, Version: rack.Version, W: size(100), H: size(200)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The valid first item proves the batch stays atomic.
			_, err := store.SaveLayout(ctx, LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{
				{ID: device.ID, Version: moved[1].Version, X: 500, Y: 500}, tc.item,
			}})
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("SaveLayout error = %v; want validation error", err)
			}
			unchanged, err := store.GetNode(ctx, device.ID)
			if err != nil || unchanged.Version != moved[1].Version {
				t.Fatalf("rejected batch committed its valid item: %#v, %v", unchanged, err)
			}
		})
	}
	cleared, err := store.SaveLayout(ctx, LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{
		{ID: device.ID, Version: moved[1].Version, X: 900, Y: 900, ParentID: text("")},
	}})
	if err != nil || cleared[0].ParentID != "" {
		t.Fatalf("clearing the parent = %#v, %v", cleared, err)
	}
}

func TestCreateEdgeRejectsSelfLoopsAndDuplicates(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	router, err := store.CreateNode(ctx, testNode(NodeDevice, "Router", "router"))
	if err != nil {
		t.Fatal(err)
	}
	access, err := store.CreateNode(ctx, testNode(NodeDevice, "Switch", "switch"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateEdge(ctx, Edge{From: router.ID, To: router.ID, Kind: EdgeEthernet}); !errors.Is(err, ErrValidation) {
		t.Fatalf("self-loop error = %v; want validation error", err)
	}
	if _, err := store.CreateEdge(ctx, Edge{From: router.ID, To: access.ID, Kind: EdgeEthernet}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateEdge(ctx, Edge{From: router.ID, To: access.ID, Kind: EdgeEthernet, Label: "again"}); !errors.Is(err, ErrEdgeExists) {
		t.Fatalf("duplicate edge error = %v; want ErrEdgeExists", err)
	}
	if _, err := store.CreateEdge(ctx, Edge{From: router.ID, To: access.ID, Kind: EdgeWiFi}); err != nil {
		t.Fatalf("same pair with another kind: %v", err)
	}
}

func TestIgnoredSourcesLeaveDiscoveriesAndSurviveRefresh(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	observed := func(key string) SourceObservation {
		return SourceObservation{Identity: SourceIdentity{Kind: "docker", Instance: "engine", Key: key}, Name: key,
			Observed: SourceObserved{Kind: NodeContainer, Name: key, Icon: "container"}}
	}
	tv, web := observed("tv"), observed("web")
	if err := store.ObserveSources(ctx, "docker", "engine", []SourceObservation{tv, web}, true); err != nil {
		t.Fatal(err)
	}
	webCandidate, err := store.GetCandidate(ctx, CandidateID(web.Identity))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateImportedNode(ctx, webCandidate.Node, web.Identity); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSourcesIgnored(ctx, []SourceIdentity{web.Identity}, true); !errors.Is(err, ErrSourceLinked) {
		t.Fatalf("ignoring a linked source = %v; want ErrSourceLinked", err)
	}
	if _, err := store.SetSourcesIgnored(ctx, []SourceIdentity{{Kind: "docker", Instance: "engine", Key: "missing"}}, true); !errors.Is(err, ErrSourceNotFound) {
		t.Fatalf("ignoring an unknown source = %v; want ErrSourceNotFound", err)
	}
	if _, err := store.SetSourcesIgnored(ctx, nil, true); !errors.Is(err, ErrValidation) {
		t.Fatalf("empty ignore request = %v; want validation error", err)
	}
	for range 2 {
		ignored, err := store.SetSourcesIgnored(ctx, []SourceIdentity{tv.Identity}, true)
		if err != nil || len(ignored) != 1 || !ignored[0].Ignored {
			t.Fatalf("ignore result = %#v, %v; want one ignored source on every call", ignored, err)
		}
	}
	// A provider refresh keeps the decision.
	if err := store.ObserveSources(ctx, "docker", "engine", []SourceObservation{tv, web}, true); err != nil {
		t.Fatal(err)
	}
	listed, err := store.ListSources(ctx, "docker", "engine")
	if err != nil || len(listed) != 1 || listed[0].Key != "web" {
		t.Fatalf("default source list = %#v, %v; want the ignored source left out", listed, err)
	}
	all, err := store.ListSourcesIncludingIgnored(ctx, "docker", "engine")
	if err != nil || len(all) != 2 {
		t.Fatalf("source list with ignored = %#v, %v; want both sources", all, err)
	}
	candidate, err := store.GetCandidate(ctx, CandidateID(tv.Identity))
	if err != nil || !candidate.Ignored || !candidate.Source.Ignored {
		t.Fatalf("ignored candidate = %#v, %v; want the ignored flag after a refresh", candidate, err)
	}
	// Ignored observations outlive the removed-source retention window.
	if err := store.ObserveSources(ctx, "docker", "engine", []SourceObservation{web}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PruneSources(ctx, time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if source, err := store.GetSource(ctx, tv.Identity); err != nil || !source.Ignored || source.State != SourceRemoved {
		t.Fatalf("ignored removed source after retention = %#v, %v; want it kept", source, err)
	}
	restored, err := store.SetSourcesIgnored(ctx, []SourceIdentity{tv.Identity}, false)
	if err != nil || len(restored) != 1 || restored[0].Ignored {
		t.Fatalf("restore = %#v, %v", restored, err)
	}
	// Importing an ignored source clears the flag: linked sources are never ignored.
	if err := store.ObserveSources(ctx, "docker", "engine", []SourceObservation{tv, web}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSourcesIgnored(ctx, []SourceIdentity{tv.Identity}, true); err != nil {
		t.Fatal(err)
	}
	tvCandidate, err := store.GetCandidate(ctx, CandidateID(tv.Identity))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateImportedNode(ctx, tvCandidate.Node, tv.Identity); err != nil {
		t.Fatalf("importing an ignored source: %v", err)
	}
	if source, err := store.GetSource(ctx, tv.Identity); err != nil || source.Ignored {
		t.Fatalf("imported source = %#v, %v; want the ignore flag cleared by the link", source, err)
	}
}

func TestSnapshotListsIgnoredSourcesOnlyOnRequest(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	identity := SourceIdentity{Kind: "docker", Instance: "engine", Key: "chromecast"}
	if err := store.ObserveSources(ctx, "docker", "engine", []SourceObservation{
		{Identity: identity, Name: "chromecast", Observed: SourceObserved{Kind: NodeContainer, Name: "chromecast", Icon: "container"}},
		{Identity: SourceIdentity{Kind: "docker", Instance: "engine", Key: "web"}, Name: "web", Observed: SourceObserved{Kind: NodeContainer, Name: "web", Icon: "container"}},
	}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSourcesIgnored(ctx, []SourceIdentity{identity}, true); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Snapshot(ctx)
	if err != nil || len(snapshot.Sources) != 1 || snapshot.Sources[0].Key != "web" {
		t.Fatalf("default snapshot sources = %#v, %v; want the ignored source left out", snapshot.Sources, err)
	}
	withIgnored, err := store.SnapshotWith(ctx, SnapshotOptions{IncludeIgnored: true})
	if err != nil || len(withIgnored.Sources) != 2 {
		t.Fatalf("snapshot with ignored = %#v, %v", withIgnored.Sources, err)
	}
	for _, source := range withIgnored.Sources {
		if source.Ignored != (source.Key == "chromecast") {
			t.Fatalf("source %s ignored = %v", source.Key, source.Ignored)
		}
	}
}

func TestDeleteNodeAndIgnoreSourcesIsAtomic(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	identity := SourceIdentity{Kind: "docker", Instance: "engine", Key: "duplicate-tv"}
	if err := store.ObserveSources(ctx, "docker", "engine", []SourceObservation{{
		Identity: identity, Name: "tv", Observed: SourceObserved{Kind: NodeContainer, Name: "tv", Icon: "container"},
	}}, true); err != nil {
		t.Fatal(err)
	}
	candidate, err := store.GetCandidate(ctx, CandidateID(identity))
	if err != nil {
		t.Fatal(err)
	}
	node, err := store.CreateImportedNode(ctx, candidate.Node, identity)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DeleteNodeAndIgnoreSources(ctx, node.ID, node.Version+1); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale delete = %v; want conflict", err)
	}
	if source, err := store.GetSource(ctx, identity); err != nil || source.Ignored {
		t.Fatalf("a refused delete ignored the source: %#v, %v", source, err)
	}
	ignored, err := store.DeleteNodeAndIgnoreSources(ctx, node.ID, node.Version)
	if err != nil || len(ignored) != 1 || ignored[0] != identity {
		t.Fatalf("delete and ignore = %#v, %v; want the linked source", ignored, err)
	}
	if _, err := store.GetNode(ctx, node.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted node lookup = %v; want not found", err)
	}
	if source, err := store.GetSource(ctx, identity); err != nil || !source.Ignored {
		t.Fatalf("source after delete and ignore = %#v, %v; want ignored", source, err)
	}
	if listed, err := store.ListSources(ctx, "docker", "engine"); err != nil || len(listed) != 0 {
		t.Fatalf("discoverable sources = %#v, %v; want none", listed, err)
	}
}

func TestSourceHostMappingUsesVersionCAS(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	host, err := store.CreateNode(ctx, testNode(NodeDevice, "TheBrain", "tower-server"))
	if err != nil {
		t.Fatal(err)
	}
	room, err := store.CreateNode(ctx, testNode(NodeRoom, "Office", "room"))
	if err != nil {
		t.Fatal(err)
	}
	identity := SourceIdentity{Kind: "docker", Instance: "docker_engine", Key: "web"}
	if err := store.ObserveSources(ctx, identity.Kind, identity.Instance, []SourceObservation{{
		Identity: identity, Name: "web", Observed: SourceObserved{Kind: NodeContainer, Name: "web", Icon: "container"},
	}}, true); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		host SourceHost
		want error
	}{
		{"unobserved engine", SourceHost{Kind: "docker", Instance: "docker_unknown", NodeID: host.ID}, ErrSourceNotFound},
		{"other source kind", SourceHost{Kind: "proxmox", Instance: "docker_engine", NodeID: host.ID}, ErrValidation},
		{"place as host", SourceHost{Kind: "docker", Instance: "docker_engine", NodeID: room.ID}, ErrValidation},
		{"missing node", SourceHost{Kind: "docker", Instance: "docker_engine", NodeID: "missing"}, ErrValidation},
	} {
		if _, err := store.SetSourceHost(ctx, tc.host); !errors.Is(err, tc.want) {
			t.Fatalf("%s error = %v; want %v", tc.name, err, tc.want)
		}
	}
	created, err := store.SetSourceHost(ctx, SourceHost{Kind: "docker", Instance: "docker_engine", NodeID: host.ID, InferredNodeID: "forged"})
	if err != nil || created.Version != 1 || created.NodeID != host.ID || created.InferredNodeID != "" {
		t.Fatalf("created mapping = %#v, %v", created, err)
	}
	if _, err := store.SetSourceHost(ctx, SourceHost{Kind: "docker", Instance: "docker_engine"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("second create error = %v; want conflict", err)
	}
	cleared, err := store.SetSourceHost(ctx, SourceHost{Kind: "docker", Instance: "docker_engine", Version: 1})
	if err != nil || cleared.Version != 2 || cleared.NodeID != "" {
		t.Fatalf("cleared mapping = %#v, %v", cleared, err)
	}
	if _, err := store.SetSourceHost(ctx, SourceHost{Kind: "docker", Instance: "docker_engine", NodeID: host.ID, Version: 2}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Snapshot(ctx)
	if err != nil || len(snapshot.SourceHosts) != 1 || snapshot.SourceHosts[0].NodeID != host.ID || snapshot.SourceHosts[0].Version != 3 {
		t.Fatalf("snapshot source hosts = %#v, %v", snapshot.SourceHosts, err)
	}
	// Deleting the host node cascades the mapping; the observed engine stays listed.
	if err := store.DeleteNode(ctx, host.ID, host.Version); err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.Snapshot(ctx)
	if err != nil || len(snapshot.SourceHosts) != 1 || snapshot.SourceHosts[0].NodeID != "" || snapshot.SourceHosts[0].Version != 0 {
		t.Fatalf("source hosts after deleting the host = %#v, %v; want an unassigned observed engine", snapshot.SourceHosts, err)
	}
}

func TestHostProjectionFieldsAreRuntimeOnly(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	node := testNode(NodeContainer, "web", "container")
	node.HostID, node.HostInferred = "forged-host", true
	created, err := store.CreateNode(context.Background(), node)
	if err != nil {
		t.Fatal(err)
	}
	if created.HostID != "" || created.HostInferred {
		t.Fatalf("caller-supplied host projection survived: %#v", created)
	}
}

func TestSaveLayoutUnplaceRemovesOnlyThatView(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	room, err := store.CreateNode(ctx, testNode(NodeRoom, "Room", "room"))
	if err != nil {
		t.Fatal(err)
	}
	device, err := store.CreateNode(ctx, testNode(NodeDevice, "Switch", "switch"))
	if err != nil {
		t.Fatal(err)
	}
	size := func(value float64) *float64 { return &value }
	atlas, err := store.SaveLayout(ctx, LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{
		{ID: room.ID, Version: room.Version, X: 10, Y: 20, W: size(600), H: size(400)},
		{ID: device.ID, Version: device.Version, X: 48, Y: 72},
	}})
	if err != nil {
		t.Fatal(err)
	}
	topology, err := store.SaveLayout(ctx, LayoutRequest{View: ViewTopology, Nodes: []LayoutItem{
		{ID: device.ID, Version: atlas[1].Version, X: 300, Y: 400, Pinned: true},
	}})
	if err != nil {
		t.Fatal(err)
	}
	roomNode, deviceNode := atlas[0], topology[0]

	// An unplace removes the placement of that view only and bumps the version.
	removed, err := store.SaveLayout(ctx, LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{
		{ID: room.ID, Version: roomNode.Version, Unplace: true},
		{ID: device.ID, Version: deviceNode.Version, Unplace: true},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 2 || removed[0].Version != roomNode.Version+1 || removed[1].Version != deviceNode.Version+1 {
		t.Fatalf("unplace result = %#v", removed)
	}
	if _, ok := removed[0].Placements[ViewAtlas]; ok || len(removed[0].Placements) != 0 {
		t.Fatalf("room placements after unplace = %#v", removed[0].Placements)
	}
	if _, ok := removed[1].Placements[ViewAtlas]; ok || removed[1].Placements[ViewTopology].X != 300 || !removed[1].Placements[ViewTopology].Pinned {
		t.Fatalf("device placements after unplace = %#v; want only the topology view kept", removed[1].Placements)
	}
	stored, err := store.GetNode(ctx, device.ID)
	if err != nil || stored.Version != deviceNode.Version+1 || len(stored.Placements) != 1 {
		t.Fatalf("stored device after unplace = %#v, %v", stored, err)
	}

	// A node without a placement in that view is a successful no-op: the version
	// is still checked but not advanced.
	noop, err := store.SaveLayout(ctx, LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{
		{ID: device.ID, Version: removed[1].Version, Unplace: true},
	}})
	if err != nil || len(noop) != 1 || noop[0].Version != removed[1].Version {
		t.Fatalf("unplace without placement = %#v, %v; want unchanged success", noop, err)
	}
	if _, err := store.SaveLayout(ctx, LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{
		{ID: device.ID, Version: removed[1].Version + 5, Unplace: true},
	}}); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale no-op unplace error = %v; want conflict", err)
	}
}

func TestSaveLayoutUnplaceChecksVersionsAndStaysAtomic(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	a, err := store.CreateNode(ctx, testNode(NodeDevice, "A", "generic-device"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := store.CreateNode(ctx, testNode(NodeDevice, "B", "generic-device"))
	if err != nil {
		t.Fatal(err)
	}
	size := func(value float64) *float64 { return &value }
	placed, err := store.SaveLayout(ctx, LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{
		{ID: a.ID, Version: a.Version, X: 10, Y: 20},
		{ID: b.ID, Version: b.Version, X: 30, Y: 40},
	}})
	if err != nil {
		t.Fatal(err)
	}
	// One stale item rolls back the valid unplace before it.
	if _, err := store.SaveLayout(ctx, LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{
		{ID: a.ID, Version: placed[0].Version, Unplace: true},
		{ID: b.ID, Version: placed[1].Version + 1, Unplace: true},
	}}); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale mixed batch error = %v; want conflict", err)
	}
	after, err := store.GetNode(ctx, a.ID)
	if err != nil || after.Version != placed[0].Version || after.Placements[ViewAtlas].X != 10 {
		t.Fatalf("stale batch committed its valid unplace: %#v, %v", after, err)
	}
	// An unplace may share a batch with an ordinary move.
	mixed, err := store.SaveLayout(ctx, LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{
		{ID: a.ID, Version: placed[0].Version, Unplace: true},
		{ID: b.ID, Version: placed[1].Version, X: 99, Y: 98},
	}})
	if err != nil || len(mixed[0].Placements) != 0 || mixed[1].Placements[ViewAtlas].X != 99 {
		t.Fatalf("mixed unplace and move = %#v, %v", mixed, err)
	}
	// An unplace item carries an id and a version only.
	for _, tc := range []struct {
		name string
		item LayoutItem
	}{
		{"x", LayoutItem{ID: b.ID, Version: mixed[1].Version, Unplace: true, X: 1}},
		{"y", LayoutItem{ID: b.ID, Version: mixed[1].Version, Unplace: true, Y: 1}},
		{"pinned", LayoutItem{ID: b.ID, Version: mixed[1].Version, Unplace: true, Pinned: true}},
		{"width", LayoutItem{ID: b.ID, Version: mixed[1].Version, Unplace: true, W: size(200)}},
		{"height", LayoutItem{ID: b.ID, Version: mixed[1].Version, Unplace: true, H: size(200)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := store.SaveLayout(ctx, LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{tc.item}}); !errors.Is(err, ErrValidation) {
				t.Fatalf("unplace with %s error = %v; want validation error", tc.name, err)
			}
		})
	}
	unchanged, err := store.GetNode(ctx, b.ID)
	if err != nil || unchanged.Version != mixed[1].Version || unchanged.Placements[ViewAtlas].X != 99 {
		t.Fatalf("rejected unplace changed the node: %#v, %v", unchanged, err)
	}
}

func TestSaveLayoutUnplaceCanRestoreTheParentAtomically(t *testing.T) {
	store, _, _ := openInfraTestStore(t)
	ctx := context.Background()
	room, err := store.CreateNode(ctx, testNode(NodeRoom, "Room", "room"))
	if err != nil {
		t.Fatal(err)
	}
	device, err := store.CreateNode(ctx, testNode(NodeDevice, "Switch", "switch"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.CreateNode(ctx, testNode(NodeDevice, "NAS", "nas"))
	if err != nil {
		t.Fatal(err)
	}
	text := func(value string) *string { return &value }
	// A library drop into the room places the device and sets its parent.
	dropped, err := store.SaveLayout(ctx, LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{
		{ID: device.ID, Version: device.Version, X: 48, Y: 72, ParentID: text(room.ID)},
	}})
	if err != nil || dropped[0].ParentID != room.ID || len(dropped[0].Placements) != 1 {
		t.Fatalf("drop = %#v, %v", dropped, err)
	}
	// An invalid parent on an unplace item rolls back the whole batch.
	for _, parent := range []string{other.ID, "missing"} {
		if _, err := store.SaveLayout(ctx, LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{
			{ID: device.ID, Version: dropped[0].Version, Unplace: true, ParentID: text(parent)},
		}}); !errors.Is(err, ErrValidation) {
			t.Fatalf("unplace with parent %q error = %v; want validation error", parent, err)
		}
	}
	unchanged, err := store.GetNode(ctx, device.ID)
	if err != nil || unchanged.Version != dropped[0].Version || unchanged.ParentID != room.ID || len(unchanged.Placements) != 1 {
		t.Fatalf("rejected unplace changed the node: %#v, %v", unchanged, err)
	}
	// Undoing the drop removes the placement and clears the parent in one call.
	undone, err := store.SaveLayout(ctx, LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{
		{ID: device.ID, Version: dropped[0].Version, Unplace: true, ParentID: text("")},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if undone[0].Version != dropped[0].Version+1 || undone[0].ParentID != "" || len(undone[0].Placements) != 0 {
		t.Fatalf("undo result = %#v; want one version step with no placement and no parent", undone[0])
	}
	stored, err := store.GetNode(ctx, device.ID)
	if err != nil || stored.Version != undone[0].Version || stored.ParentID != "" || len(stored.Placements) != 0 {
		t.Fatalf("stored node after undo = %#v, %v", stored, err)
	}
	// A parent change still applies when the node has no placement in the view.
	restored, err := store.SaveLayout(ctx, LayoutRequest{View: ViewAtlas, Nodes: []LayoutItem{
		{ID: device.ID, Version: undone[0].Version, Unplace: true, ParentID: text(room.ID)},
	}})
	if err != nil || restored[0].ParentID != room.ID || restored[0].Version != undone[0].Version+1 || len(restored[0].Placements) != 0 {
		t.Fatalf("parent restore without placement = %#v, %v", restored, err)
	}
}
