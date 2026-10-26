package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/infra"
	"aurago/internal/inventory"
	"aurago/internal/tools"
)

// desktopInfraEmptyDocker answers a Docker container list without containers.
func desktopInfraEmptyDocker(context.Context, tools.DockerConfig, string, string, string) ([]byte, int, error) {
	return []byte("[]"), http.StatusOK, nil
}

func TestDesktopInfraCandidatesAndSnapshotHideIgnoredSourcesByDefault(t *testing.T) {
	s, cfg, store, runtime := newDesktopInfraRefreshFixture(t)
	runtime.dockerRequest = desktopInfraEmptyDocker
	ctx := context.Background()
	ignoredKey := ""
	for _, name := range []string{"Office TV", "Kitchen speaker"} {
		id, err := inventory.CreateDevice(s.InventoryDB, name, "device", inventory.ProtocolNone, "192.0.2.40", 0, "", "", "", "", nil, "")
		if err != nil {
			t.Fatal(err)
		}
		ignoredKey = id
	}
	desktopInfraRefreshRegistry(ctx, s, cfg, store, runtime, true)
	if _, err := store.SetSourcesIgnored(ctx, []infra.SourceIdentity{{Kind: "registry", Instance: "inventory", Key: ignoredKey}}, true); err != nil {
		t.Fatal(err)
	}
	candidates := func(query string) []infra.Candidate {
		t.Helper()
		response := httptest.NewRecorder()
		desktopInfraCandidates(response, httptest.NewRequest(http.MethodGet, desktopInfraBasePath+"/candidates"+query, nil), s, cfg, store, runtime)
		if response.Code != http.StatusOK {
			t.Fatalf("candidates%s = %d %s", query, response.Code, response.Body.String())
		}
		var payload struct {
			Candidates []infra.Candidate `json:"candidates"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		return payload.Candidates
	}
	if got := candidates(""); len(got) != 1 || got[0].Node.Name != "Office TV" {
		t.Fatalf("default candidates = %+v; want the ignored speaker left out", got)
	}
	all := candidates("?include_ignored=1")
	if len(all) != 2 {
		t.Fatalf("candidates with ignored = %+v; want both", all)
	}
	for _, candidate := range all {
		if candidate.Ignored != (candidate.Source.Key == ignoredKey) {
			t.Fatalf("candidate %q ignored = %v", candidate.Node.Name, candidate.Ignored)
		}
	}
	sources := func(query string) int {
		t.Helper()
		response := httptest.NewRecorder()
		desktopInfraSnapshot(response, httptest.NewRequest(http.MethodGet, desktopInfraBasePath+"/snapshot"+query, nil), s, cfg, store, runtime)
		if response.Code != http.StatusOK {
			t.Fatalf("snapshot%s = %d %s", query, response.Code, response.Body.String())
		}
		var payload struct {
			Sources []infra.Source `json:"sources"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		return len(payload.Sources)
	}
	if got := sources(""); got != 1 {
		t.Fatalf("default snapshot sources = %d; want 1", got)
	}
	if got := sources("?include_ignored=1"); got != 2 {
		t.Fatalf("snapshot sources with ignored = %d; want 2", got)
	}
}

func TestDesktopInfraIgnoreRouteAndRemoveAndIgnore(t *testing.T) {
	s, cfg, store, runtime := newDesktopInfraRefreshFixture(t)
	ctx := context.Background()
	instance := desktopInfraDockerInstance(cfg)
	free := infra.SourceIdentity{Kind: "docker", Instance: instance, Key: strings.Repeat("a1", 32)}
	linked := infra.SourceIdentity{Kind: "docker", Instance: instance, Key: strings.Repeat("b2", 32)}
	if err := store.ObserveSources(ctx, "docker", instance, []infra.SourceObservation{
		{Identity: free, Name: "chromecast", Observed: infra.SourceObserved{Kind: infra.NodeContainer, Name: "chromecast", Icon: "container"}},
		{Identity: linked, Name: "web", Observed: infra.SourceObserved{Kind: infra.NodeContainer, Name: "web", Icon: "container"}},
	}, true); err != nil {
		t.Fatal(err)
	}
	candidate, err := store.GetCandidate(ctx, infra.CandidateID(linked))
	if err != nil {
		t.Fatal(err)
	}
	node, err := store.CreateImportedNode(ctx, candidate.Node, linked)
	if err != nil {
		t.Fatal(err)
	}
	identityJSON := func(identity infra.SourceIdentity) string {
		t.Helper()
		data, err := json.Marshal(identity)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	ignore := func(body string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		desktopInfraIgnoreSources(response, httptest.NewRequest(http.MethodPut, desktopInfraBasePath+"/sources/ignore", strings.NewReader(body)), cfg, store)
		return response
	}
	for _, tc := range []struct {
		name, body, code string
		status           int
	}{
		{"linked source", `{"sources":[` + identityJSON(linked) + `],"ignored":true}`, "source_linked", http.StatusConflict},
		{"unknown source", `{"sources":[{"kind":"docker","instance":"` + instance + `","key":"missing"}],"ignored":true}`, "not_found", http.StatusNotFound},
		{"missing flag", `{"sources":[` + identityJSON(free) + `]}`, "invalid_request", http.StatusBadRequest},
	} {
		if response := ignore(tc.body); response.Code != tc.status || !strings.Contains(response.Body.String(), tc.code) {
			t.Fatalf("%s = %d %s; want %d %s", tc.name, response.Code, response.Body.String(), tc.status, tc.code)
		}
	}
	response := ignore(`{"sources":[` + identityJSON(free) + `],"ignored":true}`)
	var ignored struct {
		Sources []infra.Source `json:"sources"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &ignored); err != nil || response.Code != http.StatusOK || len(ignored.Sources) != 1 || !ignored.Sources[0].Ignored {
		t.Fatalf("ignore = %d %s, %v", response.Code, response.Body.String(), err)
	}

	removePath := desktopInfraBasePath + "/nodes/" + node.ID + "?version=" + strconv.FormatInt(node.Version, 10)
	invalid := httptest.NewRecorder()
	desktopInfraNodeRoute(invalid, httptest.NewRequest(http.MethodDelete, removePath+"&ignore_sources=yes", nil), s, cfg, store, runtime, node.ID)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid ignore flag = %d %s; want 400", invalid.Code, invalid.Body.String())
	}
	removed := httptest.NewRecorder()
	desktopInfraNodeRoute(removed, httptest.NewRequest(http.MethodDelete, removePath+"&ignore_sources=1", nil), s, cfg, store, runtime, node.ID)
	var payload struct {
		Status  string                 `json:"status"`
		Ignored []infra.SourceIdentity `json:"ignored"`
	}
	if err := json.Unmarshal(removed.Body.Bytes(), &payload); err != nil || removed.Code != http.StatusOK {
		t.Fatalf("remove and ignore = %d %s, %v", removed.Code, removed.Body.String(), err)
	}
	if payload.Status != "deleted" || len(payload.Ignored) != 1 || payload.Ignored[0] != linked {
		t.Fatalf("remove and ignore answer = %#v", payload)
	}
	if source, err := store.GetSource(ctx, linked); err != nil || !source.Ignored {
		t.Fatalf("source after remove and ignore = %#v, %v", source, err)
	}
}
