package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

func TestHandleGetConfigInjectsGameMakerDefaultsWithoutOverwritingConfiguredPermissions(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("game_maker:\n  enabled: true\n  allow_create: true\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	loaded.ConfigPath = configPath
	server := &Server{Cfg: loaded, Logger: slog.Default()}

	recorder := httptest.NewRecorder()
	handleGetConfig(server).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode config response: %v", err)
	}
	gameMaker, ok := body["game_maker"].(map[string]interface{})
	if !ok {
		t.Fatalf("game_maker response = %#v", body["game_maker"])
	}
	if gameMaker["enabled"] != true || gameMaker["allow_create"] != true {
		t.Fatalf("configured Game Maker permissions were overwritten: %#v", gameMaker)
	}
	for key, want := range map[string]interface{}{
		"readonly":               true,
		"allow_edit":             false,
		"allow_delete":           false,
		"allow_media_generation": false,
		"max_projects":           float64(25),
		"max_files_per_project":  float64(250),
		"max_file_size_kb":       float64(2048),
		"max_asset_size_mb":      float64(32),
		"max_project_size_mb":    float64(100),
		"job_timeout_seconds":    float64(1800),
	} {
		if got := gameMaker[key]; got != want {
			t.Errorf("game_maker.%s = %#v, want %#v", key, got, want)
		}
	}
	wantWorkspace := filepath.Join(filepath.Dir(configPath), "agent_workspace", "virtual_desktop")
	if got := gameMaker["workspace_path"]; got != wantWorkspace {
		t.Errorf("game_maker.workspace_path = %#v, want %#v", got, wantWorkspace)
	}
}
