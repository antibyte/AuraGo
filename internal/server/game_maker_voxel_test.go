package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/gamemaker"
)

func TestGameMakerVoxelPromptProfiles(t *testing.T) {
	planning, err := gameMakerPromptProfile("planning", "3d", "voxel")
	if err != nil {
		t.Fatal(err)
	}
	building, err := gameMakerPromptProfile("building", "3d", "voxel")
	if err != nil {
		t.Fatal(err)
	}
	repair, err := gameMakerPromptProfile("repair", "3d", "voxel")
	if err != nil {
		t.Fatal(err)
	}
	if building.Revision() != repair.Revision() || building.SystemPrompt() != repair.SystemPrompt() || !reflect.DeepEqual(building.Tools(), repair.Tools()) {
		t.Fatal("voxel repair changed the cache prefix")
	}
	if planning.Revision() == building.Revision() || len(planning.Tools()) != 3 || len(building.Tools()) != 4 {
		t.Fatal("invalid voxel phase contract")
	}
	for _, word := range []string{"src/voxel.json", "startVoxelGame", "survival", "24"} {
		if !strings.Contains(building.SystemPrompt(), word) {
			t.Fatal("missing voxel contract", word)
		}
	}
	for _, word := range []string{"For an explicitly peaceful request, remove all enemies", "keep the wood→stone→metal mining and crafting progression"} {
		if !strings.Contains(planning.SystemPrompt(), word) || !strings.Contains(building.SystemPrompt(), word) {
			t.Fatal("missing explicit peaceful voxel guidance", word)
		}
	}
	planningContext := compactGameMakerContext(gamemaker.JobRun{Stage: "planning", Project: gamemaker.Project{Dimension: "3d", Variant: "voxel"}})
	contract, ok := planningContext["planning_contract"].(string)
	if !ok || !strings.Contains(contract, "JSON-encoded VoxelDefinition string") || !strings.Contains(contract, "schema 5") || !strings.Contains(contract, "movement, jump, mining, crafting, placement and pause") || !strings.Contains(contract, "only key, wait and observe") || !strings.Contains(contract, "retain the wood-to-stone-to-metal progression") {
		t.Fatalf("voxel planning context has the wrong contract: %#v", planningContext["planning_contract"])
	}
	if _, exists := planningContext["target_test_example"]; exists {
		t.Fatal("voxel planning context contains a fabricated non-voxel target test")
	}
	if strings.Contains(contract, "pointer") || strings.Contains(contract, "target IDs") {
		t.Fatalf("voxel custom check guidance advertises unsupported steps: %s", contract)
	}
	if example, ok := planningContext["design_example"].(gamemaker.GameDesign); !ok || example.Voxel == nil || len(example.Voxel.Enemies) == 0 {
		t.Fatal("voxel context lost the intentional survival example defaults")
	}
	data, _ := json.Marshal(planning.Tools())
	schema := string(data)
	if !strings.Contains(schema, `"voxel"`) || strings.Contains(schema, `"scene_set"`) || strings.Contains(schema, `"fps"`) {
		t.Fatal("voxel discovery leaked unrelated schemas")
	}
	ordinary, _ := gameMakerPromptProfile("building", "3d")
	if ordinary.Revision() == building.Revision() {
		t.Fatal("variant did not change revision")
	}
}

func TestGameMakerVoxelPlayStateHTTPBoundary(t *testing.T) {
	const path = "/api/game-maker/projects/p/play-state"
	if isAuthBypassed(path) || isAuthBypassed("/api/game-maker/projects/p/play") {
		t.Fatal("voxel host or saves bypass authentication")
	}
	s := &Server{Cfg: &config.Config{}}
	s.Cfg.Auth.Enabled = true
	for _, method := range []string{"GET", "PUT", "DELETE"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, path, strings.NewReader(`{"version":0,"state":{}}`))
		handleGameMakerPlayState(w, r, s, "p")
		if w.Code != 401 && w.Code != 403 {
			t.Fatalf("unauthenticated %s: %d", method, w.Code)
		}
	}
	root := t.TempDir()
	service, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "game.db"), WorkspacePath: filepath.Join(root, "workspace"), Enabled: true, AllowCreate: true, AllowEdit: true, AllowDelete: true})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	s.Cfg.Auth.Enabled = false
	s.GameMaker = service
	_, err = service.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Voxel", Description: "Build", Dimension: "3d", Variant: "voxel"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		method, body string
		status       int
	}{{"GET", "", 403}, {"PUT", `{"version":0,"state":{}}`, 403}, {"PUT", `{"version":0,"extra":true}`, 400}, {"PUT", `{"version":0,"state":"` + strings.Repeat("x", gamemaker.MaxPlayStateBytes) + `"}`, 413}, {"POST", "", 405}} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(tc.method, path, strings.NewReader(tc.body))
		r.Header.Set("X-Game-Maker-Play", "not-a-play-grant")
		handleGameMakerPlayState(w, r, s, "p")
		if w.Code != tc.status {
			t.Errorf("%s: %d want %d", tc.method, w.Code, tc.status)
		}
	}
}
