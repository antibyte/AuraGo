package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
