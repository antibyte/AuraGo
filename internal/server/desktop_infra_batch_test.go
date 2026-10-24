package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/infra"
	"aurago/internal/tools"
)

func TestDesktopInfraBatchImportRefreshesOnceAndReportsPartialFailures(t *testing.T) {
	s, cfg, store, runtime := newDesktopInfraRefreshFixture(t)
	ctx := context.Background()
	instance := desktopInfraDockerInstance(cfg)
	ids := []string{strings.Repeat("d1", 32), strings.Repeat("d2", 32), strings.Repeat("d3", 32)}
	names := []string{"radarr", "sonarr", "plex"}
	listCalls := 0
	runtime.dockerRequest = func(_ context.Context, _ tools.DockerConfig, method, endpoint, _ string) ([]byte, int, error) {
		if method != http.MethodGet || endpoint != "/containers/json?all=true" {
			t.Errorf("unexpected Docker request %s %s", method, endpoint)
			return nil, http.StatusNotFound, nil
		}
		listCalls++
		rows := make([]map[string]any, 0, len(ids))
		for i, id := range ids {
			rows = append(rows, map[string]any{"Id": id, "Names": []string{"/" + names[i]}, "Image": "lscr.io/" + names[i], "State": "running", "Labels": map[string]string{}})
		}
		body, err := json.Marshal(rows)
		return body, http.StatusOK, err
	}
	desktopInfraRefreshDocker(ctx, cfg, store, runtime, true)
	listCalls = 0
	room, err := store.CreateNode(ctx, infra.Node{Kind: infra.NodeRoom, Name: "Living room", Icon: "room"})
	if err != nil {
		t.Fatal(err)
	}
	notAPlace, err := store.CreateNode(ctx, infra.Node{Kind: infra.NodeDevice, Name: "Switch", Icon: "switch"})
	if err != nil {
		t.Fatal(err)
	}
	manual, err := store.CreateNode(ctx, infra.Node{Kind: infra.NodeContainer, Name: "Plex (manual)", Icon: "container"})
	if err != nil {
		t.Fatal(err)
	}
	candidateID := func(i int) string {
		return infra.CandidateID(infra.SourceIdentity{Kind: "docker", Instance: instance, Key: ids[i]})
	}
	batch := func(body string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		desktopInfraImportBatch(response, httptest.NewRequest(http.MethodPost, desktopInfraBasePath+"/import/batch", strings.NewReader(body)), s, cfg, store, runtime)
		return response
	}
	response := batch(`{"items":[` +
		`{"candidate_id":"` + candidateID(0) + `","parent_id":"` + room.ID + `","placement":{"view":"atlas","x":48,"y":72}},` +
		`{"candidate_id":"` + candidateID(1) + `","parent_id":"` + notAPlace.ID + `"},` +
		`{"candidate_id":"` + candidateID(2) + `","node_id":"` + manual.ID + `","parent_id":"` + room.ID + `","placement":{"view":"topology","x":10,"y":20}},` +
		`{"candidate_id":"infra_bad"}]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("batch import = %d %s", response.Code, response.Body.String())
	}
	var payload struct {
		Nodes  []infra.Node             `json:"nodes"`
		Errors []desktopInfraBatchError `json:"errors"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if listCalls != 1 {
		t.Fatalf("Docker list calls = %d; want one forced refresh for the whole batch", listCalls)
	}
	if len(payload.Nodes) != 2 {
		t.Fatalf("imported nodes = %#v; want two", payload.Nodes)
	}
	created, linked := payload.Nodes[0], payload.Nodes[1]
	if created.Name != "radarr" || created.ParentID != room.ID || created.Placements[infra.ViewAtlas].X != 48 || created.Placements[infra.ViewAtlas].Y != 72 {
		t.Fatalf("created node = %#v", created)
	}
	if linked.ID != manual.ID || linked.ParentID != room.ID || linked.Placements[infra.ViewTopology].Y != 20 || len(linked.Sources) != 1 {
		t.Fatalf("linked node = %#v", linked)
	}
	codes := make(map[string]string, len(payload.Errors))
	for _, failure := range payload.Errors {
		codes[failure.CandidateID] = failure.Error
	}
	if len(codes) != 2 || codes[candidateID(1)] != "invalid_request" || codes["infra_bad"] != "invalid_request" {
		t.Fatalf("batch errors = %#v", payload.Errors)
	}
	if _, err := store.GetCandidate(ctx, candidateID(1)); err != nil {
		t.Fatalf("a failed item consumed its candidate: %v", err)
	}
	for _, body := range []string{
		`{"items":[]}`,
		`{"items":[{"candidate_id":"` + candidateID(1) + `"},{"candidate_id":"` + candidateID(1) + `"}]}`,
	} {
		if rejected := batch(body); rejected.Code != http.StatusBadRequest {
			t.Fatalf("batch %s = %d %s; want 400", body, rejected.Code, rejected.Body.String())
		}
	}
}
