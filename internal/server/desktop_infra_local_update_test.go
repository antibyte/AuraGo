package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"

	"aurago/internal/credentials"
	"aurago/internal/infra"
	"aurago/internal/inventory"
)

func TestDesktopInfraLocalRegistryEditDoesNotWriteRegistryWithoutSourceGrant(t *testing.T) {
	s, _, _ := testDesktopPermissionServer(t)
	adminToken, _, err := s.TokenManager.Create("infra local edit", []string{desktopScopeAdmin}, nil)
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
	s.Cfg.VirtualDesktop.Infra.AllowSourceEdit = false

	if err := credentials.EnsureSchema(db); err != nil {
		t.Fatal(err)
	}
	credentialID, err := credentials.Create(db, credentials.Record{Name: "fixture credential", Type: "ssh", Host: "192.0.2.10", Username: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	registryID, err := inventory.CreateDevice(db, "Imported router", "router", inventory.ProtocolSSH, "192.0.2.10", 2222,
		"operator", "", credentialID, "source description", []string{"source"}, "02:00:00:00:00:10")
	if err != nil {
		t.Fatal(err)
	}
	registryBefore, err := inventory.GetDeviceByID(db, registryID)
	if err != nil {
		t.Fatal(err)
	}
	identity := infra.SourceIdentity{Kind: "registry", Instance: "inventory", Key: registryID}
	runtime, err := s.desktopInfraRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.store.ObserveSources(t.Context(), identity.Kind, identity.Instance, []infra.SourceObservation{{
		Identity: identity, Name: registryBefore.Name, Observed: desktopInfraRegistryObserved(registryBefore),
	}}, true); err != nil {
		t.Fatal(err)
	}
	candidate, err := runtime.store.GetCandidate(t.Context(), infra.CandidateID(identity))
	if err != nil {
		t.Fatal(err)
	}
	candidate.Node.RegistryID = ""
	node, err := runtime.store.CreateImportedNode(t.Context(), candidate.Node, identity)
	if err != nil {
		t.Fatal(err)
	}
	node.Name = "Changed locally only"
	node.Profile.Role = "changed-type"
	node.Profile.IPAddress = "192.0.2.99"
	node.Profile.Notes = "changed description"
	node.Profile.Tags = []string{"changed"}
	node.Profile.MACAddress = "02:00:00:00:00:99"
	payload, err := json.Marshal(node)
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	registerDesktopInfraRoutes(mux, s)
	req := httptest.NewRequest(http.MethodPut, desktopInfraBasePath+"/nodes/"+node.ID, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("local PUT status=%d body=%s", response.Code, response.Body.String())
	}
	var updated infra.Node
	if err := json.Unmarshal(response.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Version != node.Version+1 || updated.Name != node.Name || updated.Profile.Role != node.Profile.Role || updated.Profile.IPAddress != node.Profile.IPAddress {
		t.Fatalf("local node update = version %d name %q role %q ip %q; want version %d with local values", updated.Version, updated.Name, updated.Profile.Role, updated.Profile.IPAddress, node.Version+1)
	}
	registryAfter, err := inventory.GetDeviceByID(db, registryID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(registryAfter, registryBefore) {
		t.Fatalf("local Infra save changed Registry data: before=%#v after=%#v", registryBefore, registryAfter)
	}

	// The explicit editor remains the permission-gated path for Registry writes.
	editFields, err := json.Marshal(desktopInfraRegistryFields{
		Name: "Explicit Registry edit", Type: registryBefore.Type, Protocol: registryBefore.Protocol,
		IPAddress: registryBefore.IPAddress, Port: registryBefore.Port, Username: registryBefore.Username,
		CredentialID: registryBefore.CredentialID, Description: registryBefore.Description,
		Tags: registryBefore.Tags, MACAddress: registryBefore.MACAddress,
	})
	if err != nil {
		t.Fatal(err)
	}
	editPayload, err := json.Marshal(desktopInfraSourceEditRequest{Source: "registry", Version: desktopInfraRegistryVersion(registryBefore), Fields: editFields})
	if err != nil {
		t.Fatal(err)
	}
	sendExplicitEdit := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, desktopInfraBasePath+"/nodes/"+node.ID+"/source-edit?source=registry", bytes.NewReader(editPayload))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}
	editResponse := sendExplicitEdit()
	if editResponse.Code != http.StatusForbidden {
		t.Fatalf("explicit Registry edit without source grant status=%d body=%s", editResponse.Code, editResponse.Body.String())
	}
	next := *s.ConfigSnapshot()
	next.VirtualDesktop.Infra.AllowSourceEdit = true
	s.replaceConfigSnapshot(&next)
	editResponse = sendExplicitEdit()
	if editResponse.Code != http.StatusOK {
		t.Fatalf("explicit Registry edit status=%d body=%s", editResponse.Code, editResponse.Body.String())
	}
	registryAfter, err = inventory.GetDeviceByID(db, registryID)
	if err != nil {
		t.Fatal(err)
	}
	if registryAfter.Name != "Explicit Registry edit" {
		t.Fatalf("explicit Registry edit did not update the source: %#v", registryAfter)
	}
}
