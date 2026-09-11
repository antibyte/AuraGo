package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/gamemaker"
	"aurago/internal/memory"
	"aurago/internal/tools"

	openai "github.com/sashabaranov/go-openai"
)

// Exercise the real agent request construction/fitting, not a replacement job runner.
func TestGameMakerModelContractReachesAgentRequest(t *testing.T) {
	root := t.TempDir()
	service, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "games.db"), WorkspacePath: filepath.Join(root, "games")})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	cfg := &config.Config{}
	cfg.LLM.Model = "test-game-model"
	cfg.LLM.ProviderType = "openai"
	cfg.Agent.ContextWindow = 65536
	cfg.CircuitBreaker.LLMTimeoutSeconds = 10
	cfg.CircuitBreaker.MaxToolCalls = 4
	cfg.GameMaker.Enabled = true
	cfg.Directories.ToolsDir = filepath.Join(root, "tools")
	cfg.Directories.WorkspaceDir = root
	requests := make(chan openai.ChatCompletionRequest, 8)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- request
		w.Header().Set("Content-Type", "text/event-stream")
		content := "Done.<done/>"
		if request.Model == "empty-game-model" {
			content = ""
		}
		fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":%q},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", content)
	}))
	defer provider.Close()
	clientConfig := openai.DefaultConfig("local-test")
	clientConfig.BaseURL = provider.URL
	client := openai.NewClientWithConfig(clientConfig)
	server := &Server{Cfg: cfg, LLMClient: client, GameMaker: service, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), HistoryManager: memory.NewEphemeralHistoryManager()}
	server.Registry = tools.NewProcessRegistry(server.Logger)
	server.ShortTermMem, err = memory.NewSQLiteMemory(":memory:", server.Logger)
	if err != nil {
		t.Fatal(err)
	}
	defer server.ShortTermMem.Close()
	runner := &gameMakerAgentRunner{server: server, service: service}
	for _, stage := range []string{"building", "repair"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := runner.RunGameMakerJob(ctx, gamemaker.JobRun{
			Stage: stage, Job: gamemaker.Job{ID: "job_model_prompt", Prompt: "FPS shooter"},
			Project:    gamemaker.Project{ID: "model-project", Dimension: "3d"},
			AssetPacks: []gamemaker.ImportedAssetPack{{ID: gamemaker.ModelPackID, Kind: "model3d", Version: "1.0.0", AssetIDs: []string{"fps-rifle"}, Manifests: map[string]string{"fps-rifle": "assets/builtin/aurago-low-poly/1.0.0/assets/fps-rifle.json"}, ThreeExample: "EXACT_PROJECT_LOCAL_MODEL_EXAMPLE"}},
		})
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		var request openai.ChatCompletionRequest
		select {
		case request = <-requests:
		default:
			t.Fatal("agent request was never constructed")
		}
		var system, user strings.Builder
		for _, message := range request.Messages {
			if message.Role == "system" {
				system.WriteString(message.Content)
			} else if message.Role == "user" {
				user.WriteString(message.Content)
			}
		}
		for _, required := range []string{"A.loadAsset(manifest", "A.createInstance(asset", "record.root", "A.releaseAsset(asset)", "fps_binding.weapon_translation", "do not read minified vendor"} {
			if !strings.Contains(system.String(), required) {
				t.Errorf("%s request lost runtime API: %s", stage, required)
			}
		}
		if !strings.Contains(user.String(), `"three_example":"EXACT_PROJECT_LOCAL_MODEL_EXAMPLE"`) || !strings.Contains(user.String(), "assets/fps-rifle.json") {
			t.Errorf("%s request lost local import paths/example", stage)
		}
	}
	cfg.LLM.Model = "empty-game-model"
	for _, stage := range []string{"building", "repair"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := runner.RunGameMakerJob(ctx, gamemaker.JobRun{
			Stage: stage, Job: gamemaker.Job{ID: "job_empty_" + stage, Prompt: "a fps game in the woods"},
			Project: gamemaker.Project{ID: "empty-project", Dimension: "2d"},
		})
		cancel()
		if err == nil || !strings.Contains(err.Error(), "empty response") {
			t.Errorf("%s treated empty provider output as success: %v", stage, err)
		}
	}
}
