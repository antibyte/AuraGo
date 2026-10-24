package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
