package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/infra"
	"aurago/internal/inventory"
	"aurago/internal/tools"
)

func infraActionFixture(t *testing.T) (*Server, *config.Config, *desktopInfraRuntime, string, string) {
	t.Helper()
	s, cfg, store, runtime := newDesktopInfraRefreshFixture(t)
	cfg.VirtualDesktop.Infra.AllowControl = true
	ctx := context.Background()
	containerID := strings.Repeat("ab", 32)
	identity := infra.SourceIdentity{Kind: "docker", Instance: desktopInfraDockerInstance(cfg), Key: containerID}
	observation := infra.SourceObservation{
		Identity: identity, Name: "web",
		Observed: infra.SourceObserved{Kind: infra.NodeContainer, Name: "web", Icon: "container", Power: infra.PowerRunning},
	}
	if err := store.ObserveSources(ctx, identity.Kind, identity.Instance, []infra.SourceObservation{observation}, true); err != nil {
		t.Fatal(err)
	}
	candidate, err := store.GetCandidate(ctx, infra.CandidateID(identity))
	if err != nil {
		t.Fatal(err)
	}
	node, err := store.CreateImportedNode(ctx, candidate.Node, identity)
	if err != nil {
		t.Fatal(err)
	}
	return s, cfg, runtime, node.ID, containerID
}

func infraActionRequest(t *testing.T, s *Server, cfg *config.Config, store *infra.Store, runtime *desktopInfraRuntime, nodeID, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, desktopInfraBasePath+"/nodes/"+nodeID+"/actions", strings.NewReader(body))
	response := httptest.NewRecorder()
	desktopInfraAction(response, request, s, cfg, store, runtime, nodeID)
	return response
}

func TestDesktopInfraActionConfirmsReplaysAndConflicts(t *testing.T) {
	s, cfg, runtime, nodeID, containerID := infraActionFixture(t)
	store := runtime.store
	writes := 0
	runtime.dockerRequest = func(_ context.Context, _ tools.DockerConfig, method, endpoint, _ string) ([]byte, int, error) {
		if strings.HasSuffix(endpoint, "/json") {
			return []byte(`{"Id":"` + containerID + `","Name":"/web","State":{"Status":"running","StartedAt":"2026-01-01T00:00:00Z"},"Config":{"Image":"nginx:latest","Labels":{}}}`), http.StatusOK, nil
		}
		writes++
		return nil, http.StatusNoContent, nil
	}

	response := infraActionRequest(t, s, cfg, store, runtime, nodeID,
		`{"source":"docker","action":"stop","request_id":"stop-unconfirmed"}`)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "confirmation_required") {
		t.Fatalf("unconfirmed stop = %d %s; want 400 confirmation_required", response.Code, response.Body.String())
	}

	response = infraActionRequest(t, s, cfg, store, runtime, nodeID,
		`{"source":"docker","action":"start","request_id":"start-1"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("start = %d %s", response.Code, response.Body.String())
	}
	var started desktopInfraActionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	if started.Status != infra.ActionCompleted || writes != 1 {
		t.Fatalf("start response = %#v after %d dispatches; want completed once", started, writes)
	}

	response = infraActionRequest(t, s, cfg, store, runtime, nodeID,
		`{"source":"docker","action":"start","request_id":"start-1"}`)
	var replayed desktopInfraActionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &replayed); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || replayed.Status != infra.ActionCompleted || writes != 1 {
		t.Fatalf("replay = %d %#v after %d dispatches; want the recorded result without a second write", response.Code, replayed, writes)
	}

	response = infraActionRequest(t, s, cfg, store, runtime, nodeID,
		`{"source":"docker","action":"stop","request_id":"start-1","confirmed":true}`)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "request_id_conflict") {
		t.Fatalf("request id reuse = %d %s; want 409 request_id_conflict", response.Code, response.Body.String())
	}
}

func TestDesktopInfraSnapshotReportsSourceRefreshFailure(t *testing.T) {
	s, cfg, store, runtime := newDesktopInfraRefreshFixture(t)
	runtime.dockerRequest = func(context.Context, tools.DockerConfig, string, string, string) ([]byte, int, error) {
		return nil, http.StatusBadGateway, nil
	}
	response := httptest.NewRecorder()
	desktopInfraSnapshot(response, httptest.NewRequest(http.MethodGet, desktopInfraBasePath+"/snapshot?refresh=1", nil), s, cfg, store, runtime)
	if response.Code != http.StatusOK {
		t.Fatalf("snapshot status = %d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		SourceErrors map[string]string `json:"source_errors"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	key := "docker|" + desktopInfraDockerInstance(cfg)
	if payload.SourceErrors[key] != "provider_unreachable" {
		t.Fatalf("source_errors = %#v; want a sanitized docker failure code", payload.SourceErrors)
	}
}

func TestDesktopInfraLayoutRouteReturnsOnlyChangedNodes(t *testing.T) {
	s, cfg, _, runtime := newDesktopInfraRefreshFixture(t)
	store := runtime.store
	ctx := context.Background()
	first, err := store.CreateNode(ctx, infra.Node{Kind: infra.NodeDevice, Name: "web", Icon: "generic-device"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateNode(ctx, infra.Node{Kind: infra.NodeDevice, Name: "db", Icon: "generic-device"}); err != nil {
		t.Fatal(err)
	}
	body := `{"view":"atlas","nodes":[{"id":"` + first.ID + `","version":1,"x":120,"y":140,"pinned":true}]}`
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, desktopInfraBasePath+"/layout", strings.NewReader(body))
	desktopInfraSaveLayout(response, request, s, cfg, store)
	if response.Code != http.StatusOK {
		t.Fatalf("layout status = %d %s", response.Code, response.Body.String())
	}
	var payload struct {
		Nodes []infra.Node `json:"nodes"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Nodes) != 1 || payload.Nodes[0].ID != first.ID {
		t.Fatalf("layout answer = %d nodes; want only the changed node", len(payload.Nodes))
	}
	if payload.Nodes[0].Placements[infra.ViewAtlas].X != 120 {
		t.Fatalf("layout answer placements = %#v", payload.Nodes[0].Placements)
	}
}

func TestDesktopInfraDeleteRefusesANodeWithChildren(t *testing.T) {
	s, cfg, _, runtime := newDesktopInfraRefreshFixture(t)
	store := runtime.store
	ctx := context.Background()
	parent, err := store.CreateNode(ctx, infra.Node{Kind: infra.NodeGroup, Name: "rack", Icon: "group"})
	if err != nil {
		t.Fatal(err)
	}
	child := infra.Node{Kind: infra.NodeDevice, Name: "switch", Icon: "switch", ParentID: parent.ID}
	if _, err := store.CreateNode(ctx, child); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, desktopInfraBasePath+"/nodes/"+parent.ID+"?version="+strconv.FormatInt(parent.Version, 10), nil)
	desktopInfraNodeRoute(response, request, s, cfg, store, runtime, parent.ID)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "has_children") {
		t.Fatalf("delete with children = %d %s; want 409 has_children", response.Code, response.Body.String())
	}
}

func TestDesktopInfraImportCreatesANodeFromARegistryCandidate(t *testing.T) {
	s, cfg, _, runtime := newDesktopInfraRefreshFixture(t)
	store := runtime.store
	ctx := context.Background()
	deviceID, err := inventory.CreateDevice(s.InventoryDB, "Office TV", "tv", inventory.ProtocolNone, "192.0.2.30", 0, "", "", "", "", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ObserveSources(ctx, "registry", "inventory", []infra.SourceObservation{{
		Identity: infra.SourceIdentity{Kind: "registry", Instance: "inventory", Key: deviceID},
		Name:     "Office TV",
		Observed: infra.SourceObserved{Kind: infra.NodeDevice, Name: "Office TV", Icon: "generic-device"},
	}}, true); err != nil {
		t.Fatal(err)
	}
	candidateID := infra.CandidateID(infra.SourceIdentity{Kind: "registry", Instance: "inventory", Key: deviceID})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, desktopInfraBasePath+"/import", strings.NewReader(`{"candidate_id":"`+candidateID+`"}`))
	desktopInfraImport(response, request, s, cfg, store, runtime)
	if response.Code != http.StatusCreated {
		t.Fatalf("import = %d %s", response.Code, response.Body.String())
	}
	var node infra.Node
	if err := json.Unmarshal(response.Body.Bytes(), &node); err != nil {
		t.Fatal(err)
	}
	if node.ID == "" || node.RegistryID != deviceID || len(node.Sources) != 1 {
		t.Fatalf("imported node = %#v", node)
	}
	replay := httptest.NewRecorder()
	desktopInfraImport(replay, httptest.NewRequest(http.MethodPost, desktopInfraBasePath+"/import", strings.NewReader(`{"candidate_id":"`+candidateID+`"}`)), s, cfg, store, runtime)
	if replay.Code != http.StatusConflict || !strings.Contains(replay.Body.String(), "source_unavailable") {
		t.Fatalf("second import = %d %s; want 409 source_unavailable", replay.Code, replay.Body.String())
	}
}

func TestDesktopInfraEdgeUpdateRouteRefusesSelfLoopsAndDuplicates(t *testing.T) {
	_, _, store, _ := newDesktopInfraRefreshFixture(t)
	ctx := context.Background()
	node := func(name string) infra.Node {
		created, err := store.CreateNode(ctx, infra.Node{Kind: infra.NodeDevice, Name: name, Icon: "generic-device"})
		if err != nil {
			t.Fatal(err)
		}
		return created
	}
	router, access := node("router"), node("switch")
	if _, err := store.CreateEdge(ctx, infra.Edge{From: router.ID, To: access.ID, Kind: infra.EdgeEthernet}); err != nil {
		t.Fatal(err)
	}
	wireless, err := store.CreateEdge(ctx, infra.Edge{From: router.ID, To: access.ID, Kind: infra.EdgeWiFi})
	if err != nil {
		t.Fatal(err)
	}
	put := func(edge infra.Edge) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(edge)
		if err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		desktopInfraEdgeRoute(response, httptest.NewRequest(http.MethodPut, desktopInfraBasePath+"/edges/"+edge.ID, strings.NewReader(string(body))), store, edge.ID)
		return response
	}
	selfLoop := wireless
	selfLoop.To = selfLoop.From
	if response := put(selfLoop); response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "invalid_request") {
		t.Fatalf("self-loop update = %d %s; want 400 invalid_request", response.Code, response.Body.String())
	}
	duplicate := wireless
	duplicate.Kind = infra.EdgeEthernet
	if response := put(duplicate); response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "edge_exists") {
		t.Fatalf("duplicate update = %d %s; want 409 edge_exists", response.Code, response.Body.String())
	}
	relabelled := wireless
	relabelled.Label = "guest"
	if response := put(relabelled); response.Code != http.StatusOK {
		t.Fatalf("relabel update = %d %s; want 200", response.Code, response.Body.String())
	}
}

func TestDesktopInfraStoreErrorMapsEverySentinel(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{infra.ErrConflict, http.StatusConflict, "conflict"},
		{infra.ErrNotFound, http.StatusNotFound, "not_found"},
		{infra.ErrSourceNotFound, http.StatusNotFound, "not_found"},
		{infra.ErrSourceRemoved, http.StatusConflict, "source_unavailable"},
		{infra.ErrSourceLinked, http.StatusConflict, "source_unavailable"},
		{infra.ErrHasChildren, http.StatusConflict, "has_children"},
		{infra.ErrEdgeExists, http.StatusConflict, "edge_exists"},
		{infra.ErrReadOnly, http.StatusForbidden, "source_readonly"},
		{infra.ErrValidation, http.StatusBadRequest, "invalid_request"},
		{errors.New("boom"), http.StatusInternalServerError, "infra_error"},
	}
	for _, testCase := range cases {
		response := httptest.NewRecorder()
		desktopInfraStoreError(response, testCase.err)
		if response.Code != testCase.status || !strings.Contains(response.Body.String(), testCase.code) {
			t.Fatalf("%v mapped to %d %s; want %d %s", testCase.err, response.Code, response.Body.String(), testCase.status, testCase.code)
		}
	}
}

func TestDesktopInfraDecodeRejectsAnOversizedBodyWith413(t *testing.T) {
	body := `{"name":"` + strings.Repeat("a", desktopInfraBodyLimit) + `"}`
	request := httptest.NewRequest(http.MethodPost, desktopInfraBasePath+"/nodes", strings.NewReader(body))
	response := httptest.NewRecorder()
	if desktopInfraDecode(response, request, &infra.Node{}) {
		t.Fatal("oversized body was accepted")
	}
	if response.Code != http.StatusRequestEntityTooLarge || !strings.Contains(response.Body.String(), "request_too_large") {
		t.Fatalf("oversized body = %d %s; want 413 request_too_large", response.Code, response.Body.String())
	}
}

func TestDesktopInfraRefreshContextHonoursRevocation(t *testing.T) {
	s, cfg, _, runtime := newDesktopInfraRefreshFixture(t)
	request := httptest.NewRequest(http.MethodGet, desktopInfraBasePath+"/snapshot?refresh=1", nil)
	ctx, finish, ok := desktopInfraRefreshContext(s, request, cfg, runtime)
	if !ok || ctx == nil {
		t.Fatal("an admitted configuration did not register a refresh context")
	}
	finish()
	revoked := &config.Config{}
	revoked.VirtualDesktop.Enabled = true
	revoked.VirtualDesktop.Infra.Enabled = false
	s.Cfg = revoked
	s.initConfigSnapshot()
	if _, finish, ok := desktopInfraRefreshContext(s, request, cfg, runtime); ok {
		finish()
		t.Fatal("a revoked configuration still registered a refresh context")
	}
}

func TestDesktopInfraLayoutRouteUnplacesANodeInOneView(t *testing.T) {
	s, cfg, _, runtime := newDesktopInfraRefreshFixture(t)
	store := runtime.store
	ctx := context.Background()
	node, err := store.CreateNode(ctx, infra.Node{Kind: infra.NodeDevice, Name: "web", Icon: "generic-device"})
	if err != nil {
		t.Fatal(err)
	}
	save := func(body string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		desktopInfraSaveLayout(response, httptest.NewRequest(http.MethodPut, desktopInfraBasePath+"/layout", strings.NewReader(body)), s, cfg, store)
		return response
	}
	id := `"id":"` + node.ID + `"`
	if response := save(`{"view":"atlas","nodes":[{` + id + `,"version":1,"x":120,"y":140}]}`); response.Code != http.StatusOK {
		t.Fatalf("atlas placement = %d %s", response.Code, response.Body.String())
	}
	if response := save(`{"view":"topology","nodes":[{` + id + `,"version":2,"x":30,"y":40}]}`); response.Code != http.StatusOK {
		t.Fatalf("topology placement = %d %s", response.Code, response.Body.String())
	}
	response := save(`{"view":"atlas","nodes":[{` + id + `,"version":3,"unplace":true}]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("unplace = %d %s", response.Code, response.Body.String())
	}
	var payload struct {
		Nodes []infra.Node `json:"nodes"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Nodes) != 1 || payload.Nodes[0].Version != 4 {
		t.Fatalf("unplace answer = %#v; want the node at version 4", payload.Nodes)
	}
	if _, ok := payload.Nodes[0].Placements[infra.ViewAtlas]; ok || payload.Nodes[0].Placements[infra.ViewTopology].X != 30 {
		t.Fatalf("unplace answer placements = %#v; want the topology view only", payload.Nodes[0].Placements)
	}
	// A stale version conflicts and an item with coordinates is invalid.
	if stale := save(`{"view":"atlas","nodes":[{` + id + `,"version":3,"unplace":true}]}`); stale.Code != http.StatusConflict {
		t.Fatalf("stale unplace = %d %s; want 409", stale.Code, stale.Body.String())
	}
	if invalid := save(`{"view":"atlas","nodes":[{` + id + `,"version":4,"x":5,"unplace":true}]}`); invalid.Code != http.StatusBadRequest {
		t.Fatalf("unplace with coordinates = %d %s; want 400", invalid.Code, invalid.Body.String())
	}
}

func TestDesktopInfraNewWriteRoutesAreRoutedAndHonourReadOnly(t *testing.T) {
	s, _, writeToken := testDesktopPermissionServer(t)
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
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		return response
	}
	type route struct {
		method, path, body string
		status             int
	}
	routes := []route{
		{http.MethodPut, desktopInfraBasePath + "/sources/ignore", `{}`, http.StatusBadRequest},
		{http.MethodPut, desktopInfraBasePath + "/source-hosts", `{}`, http.StatusBadRequest},
		{http.MethodPost, desktopInfraBasePath + "/import/batch", `{}`, http.StatusBadRequest},
		{http.MethodDelete, desktopInfraBasePath + "/nodes/missing?version=1&ignore_sources=1", "", http.StatusNotFound},
	}
	for _, tc := range routes {
		if response := call(tc.method, tc.path, writeToken, tc.body); response.Code != http.StatusForbidden && response.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s without admin scope = %d %s", tc.method, tc.path, response.Code, response.Body.String())
		}
		// An admin reaches the handler's own validation, so the route exists.
		if response := call(tc.method, tc.path, adminToken, tc.body); response.Code != tc.status {
			t.Fatalf("%s %s = %d %s; want %d", tc.method, tc.path, response.Code, response.Body.String(), tc.status)
		}
	}
	s.Cfg.VirtualDesktop.Infra.ReadOnly = true
	for _, tc := range routes {
		response := call(tc.method, tc.path, adminToken, tc.body)
		if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "infra_readonly") {
			t.Fatalf("%s %s on read-only Infra = %d %s; want 403 infra_readonly", tc.method, tc.path, response.Code, response.Body.String())
		}
	}
}
