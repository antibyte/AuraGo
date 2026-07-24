package server

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/gamemaker"
)

func TestGameMakerAgentScopeContainsOnlyCuratedToolsAndSkills(t *testing.T) {
	wantTools := []string{
		"game_maker_project",
		"game_maker_file",
		"game_maker_asset",
		"game_maker_validate",
		"list_agent_skills",
		"activate_agent_skill",
	}
	if !slices.Equal(gameMakerAllowedTools, wantTools) {
		t.Fatalf("Game Maker AllowedTools = %v, want %v", gameMakerAllowedTools, wantTools)
	}
	for _, forbidden := range []string{
		"invoke_tool", "filesystem", "execute_shell", "execute_python",
		"api_request", "homepage_project", "desktop_computer",
	} {
		if slices.Contains(gameMakerAllowedTools, forbidden) {
			t.Fatalf("forbidden tool %q present in Game Maker scope", forbidden)
		}
	}
	wantSkills := []string{
		"aurago-game-assets",
		"aurago-game-maker-director",
		"aurago-game-qa",
		"aurago-phaser4-gameplay",
		"aurago-threejs-gameplay",
	}
	if got := gamemaker.CuratedSkillNames(); !slices.Equal(got, wantSkills) {
		t.Fatalf("curated Game Maker skills = %v, want %v", got, wantSkills)
	}
}

func TestGameMakerConfigSeparatesLivePolicyFromRuntimeSettings(t *testing.T) {
	oldCfg := config.GameMakerConfig{
		Enabled: true, ReadOnly: true, WorkspacePath: "workspace",
		MaxProjects: 10, MaxFilesPerProject: 20, MaxFileSizeKB: 30,
		MaxProjectSizeMB: 40, JobTimeoutSeconds: 50,
	}
	livePolicyChange := oldCfg
	livePolicyChange.ReadOnly = false
	livePolicyChange.AllowCreate = true
	if gameMakerRuntimeConfigChanged(oldCfg, livePolicyChange) {
		t.Fatal("permission-only Game Maker change unexpectedly requires restart")
	}
	policy := gameMakerPolicy(livePolicyChange)
	if policy.ReadOnly || !policy.AllowCreate {
		t.Fatalf("live policy = %+v", policy)
	}
	runtimeChange := livePolicyChange
	runtimeChange.WorkspacePath = "other"
	if !gameMakerRuntimeConfigChanged(oldCfg, runtimeChange) {
		t.Fatal("workspace change must require restart")
	}
}

func TestGameMakerPreviewIsTokenScopedAndIframeCompatible(t *testing.T) {
	previewPath := "/api/game-maker/preview/token/index.html"
	if !isAuthBypassed(previewPath) {
		t.Fatal("token-authenticated Game Maker preview must bypass session authentication")
	}
	if isAuthBypassed("/api/game-maker/projects") {
		t.Fatal("Game Maker project APIs must remain session authenticated")
	}
	if !isDesktopScopedAPIPath("/api/game-maker/projects") {
		t.Fatal("Game Maker APIs must accept scoped desktop bearer tokens")
	}
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'self'")
		w.WriteHeader(http.StatusNoContent)
	}), true, false)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "https://example.test"+previewPath, nil))
	if got := rec.Header().Get("X-Frame-Options"); got != "" {
		t.Fatalf("Game Maker preview X-Frame-Options = %q, want empty", got)
	}

	root := t.TempDir()
	service, err := gamemaker.NewService(gamemaker.Options{
		DBPath:        filepath.Join(root, "game_maker.db"),
		WorkspacePath: filepath.Join(root, "workspace"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	previewRecorder := httptest.NewRecorder()
	handleGameMakerPreview(&Server{GameMaker: service}).ServeHTTP(
		previewRecorder,
		httptest.NewRequest(http.MethodGet, previewPath, nil),
	)
	if previewRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("invalid preview token status = %d, want %d", previewRecorder.Code, http.StatusUnauthorized)
	}
	if got := previewRecorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("preview error Access-Control-Allow-Origin = %q, want *", got)
	}
	if got := previewRecorder.Header().Get("Cross-Origin-Resource-Policy"); got != "cross-origin" {
		t.Fatalf("preview error Cross-Origin-Resource-Policy = %q, want cross-origin", got)
	}
	if got := previewRecorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("preview error Cache-Control = %q, want no-store", got)
	}
}

func TestGameMakerSSEReplaysMoreThanOnePageWithoutGaps(t *testing.T) {
	root := t.TempDir()
	service, err := gamemaker.NewService(gamemaker.Options{
		DBPath: filepath.Join(root, "game_maker.db"), WorkspacePath: filepath.Join(root, "workspace"),
		Enabled: true, AllowCreate: true, MaxProjects: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.CreateProject(context.Background(), gamemaker.CreateProjectRequest{
		Name: "SSE", Dimension: "2d", Description: "Replay every persisted event.",
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 520; i++ {
		if err := service.EmitAgentEvent(context.Background(), project.ID, "", "phase", map[string]any{"index": i}); err != nil {
			t.Fatal(err)
		}
	}
	first, err := service.EventsAfter(context.Background(), project.ID, 0, 500)
	if err != nil || len(first) != 500 {
		t.Fatalf("first event page = %d, %v", len(first), err)
	}
	second, err := service.EventsAfter(context.Background(), project.ID, first[len(first)-1].ID, 500)
	if err != nil || len(second) == 0 {
		t.Fatalf("second event page = %d, %v", len(second), err)
	}
	wantLastID := second[len(second)-1].ID

	serverState := &Server{Cfg: &config.Config{}, GameMaker: service}
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleGameMakerEvents(w, r, serverState, project.ID)
	}))
	defer httpServer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, httpServer.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := httpServer.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	scanner := bufio.NewScanner(response.Body)
	var gotLastID int64
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "id: ") {
			continue
		}
		gotLastID, err = strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, "id: ")), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		if gotLastID == wantLastID {
			break
		}
	}
	if gotLastID != wantLastID {
		t.Fatal(fmt.Errorf("SSE replay stopped at event %d, want %d", gotLastID, wantLastID))
	}
}
