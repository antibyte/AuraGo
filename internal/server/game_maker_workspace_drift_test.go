package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/gamemaker"
)

func TestWorkspaceDriftStaysOutsideThePromptProfile(t *testing.T) {
	drift := &gamemaker.WorkspaceDrift{
		Revision:     2,
		Changed:      []string{"src/main.ts"},
		Added:        []string{},
		Removed:      []string{},
		ChangedCount: 1,
	}
	packet := compactGameMakerContext(gamemaker.JobRun{
		Stage:          "planning",
		Project:        gamemaker.Project{Dimension: "2d"},
		WorkspaceDrift: drift,
	})
	raw, err := json.Marshal(packet["workspace_drift"])
	if err != nil || !strings.Contains(string(raw), "src/main.ts") || strings.Contains(string(raw), "Revision") {
		t.Fatalf("planning context = %s err %v", raw, err)
	}
	building := compactGameMakerContext(gamemaker.JobRun{Stage: "building", Project: gamemaker.Project{Dimension: "2d"}, WorkspaceDrift: drift})
	if _, ok := building["workspace_drift"]; ok {
		t.Fatal("building context included workspace drift")
	}
	empty := compactGameMakerContext(gamemaker.JobRun{Stage: "planning", Project: gamemaker.Project{Dimension: "2d"}, WorkspaceDrift: &gamemaker.WorkspaceDrift{}})
	if _, ok := empty["workspace_drift"]; ok {
		t.Fatal("empty drift was attached")
	}
	profile, err := gameMakerPromptProfile("planning", "2d")
	if err != nil {
		t.Fatal(err)
	}
	again, err := gameMakerPromptProfile("planning", "2d")
	if err != nil {
		t.Fatal(err)
	}
	if profile.Revision() != again.Revision() || strings.Contains(profile.SystemPrompt(), "src/main.ts") {
		t.Fatal("workspace drift changed the stable planning profile")
	}
}

func TestWorkspaceDriftRouteReturnsRelativePaths(t *testing.T) {
	root := t.TempDir()
	service, err := gamemaker.NewService(gamemaker.Options{
		DBPath: filepath.Join(root, "gm.db"), WorkspacePath: filepath.Join(root, "workspace"),
		Enabled: true, AllowCreate: true, AllowEdit: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.CreateProject(context.Background(), gamemaker.CreateProjectRequest{
		Name: "Drift Route", Dimension: "2d", Description: "Compare the workspace.",
	})
	if err != nil {
		t.Fatal(err)
	}
	serverState := &Server{Cfg: &config.Config{}, GameMaker: service}
	request := httptest.NewRequest(http.MethodGet, "/api/game-maker/projects/"+project.ID+"/workspace-drift", nil)
	recorder := httptest.NewRecorder()
	handleGameMakerProjectPath(serverState).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status %d body %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), root) {
		t.Fatalf("drift response contains a host path: %s", recorder.Body.String())
	}
	var drift gamemaker.WorkspaceDrift
	if err := json.Unmarshal(recorder.Body.Bytes(), &drift); err != nil {
		t.Fatal(err)
	}
	if drift.Revision != 0 || !drift.Empty() {
		t.Fatalf("unpublished drift = %+v", drift)
	}
}
