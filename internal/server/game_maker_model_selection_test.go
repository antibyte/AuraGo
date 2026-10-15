package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/gamemaker"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/tools"

	openai "github.com/sashabaranov/go-openai"
)

func TestGameMakerSelectedModelReachesProvider(t *testing.T) {
	for _, selection := range []struct {
		name, provider, model, wantRoute, wantModel string
	}{
		{"model only", "", "chosen-model", "system", "chosen-model"},
		{"provider default", "selected", "", "selected", "provider-model"},
		{"provider and model", "selected", "chosen-model", "selected", "chosen-model"},
		{"system default", "", "", "system", "system-model"},
	} {
		for _, phase := range []string{"planning", "building", "repair", "source"} {
			t.Run(selection.name+"/"+phase, func(t *testing.T) {
				root := t.TempDir()
				service, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "games.db"), WorkspacePath: filepath.Join(root, "games"), Enabled: true, AllowCreate: true, AllowEdit: true})
				if err != nil {
					t.Fatal(err)
				}
				defer service.Close()
				service.SetSkillStatus(nil, true)
				type observed struct {
					route, model string
					tools        int
				}
				requests := make(chan observed, 8)
				var answer atomic.Value
				answer.Store("Done.<done/>")
				provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodPost {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					var request openai.ChatCompletionRequest
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						t.Error(err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					requests <- observed{strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")[0], request.Model, len(request.Tools)}
					chunk, _ := json.Marshal(openai.ChatCompletionStreamResponse{Choices: []openai.ChatCompletionStreamChoice{{Index: 0, Delta: openai.ChatCompletionStreamChoiceDelta{Content: answer.Load().(string)}, FinishReason: openai.FinishReasonStop}}})
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", chunk)
				}))
				defer provider.Close()
				cfg := &config.Config{}
				cfg.LLM.Provider, cfg.LLM.ProviderType, cfg.LLM.Model = "system", "openai", "system-model"
				cfg.LLM.BaseURL = provider.URL + "/system"
				cfg.Providers = []config.ProviderEntry{
					{ID: "system", Type: "openai", Model: "system-model", BaseURL: cfg.LLM.BaseURL, ContextWindow: 65536, MaxOutputTokens: 32768},
					{ID: "selected", Type: "openai", Model: "provider-model", BaseURL: provider.URL + "/selected", ContextWindow: 65536, MaxOutputTokens: 32768},
				}
				cfg.Agent.ContextWindow = 65536
				cfg.CircuitBreaker.LLMTimeoutSeconds, cfg.CircuitBreaker.MaxToolCalls = 10, 4
				cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir = root, filepath.Join(root, "tools")
				cfg.GameMaker.Enabled = true
				cfg.LLMRouter.Enabled = true
				cfg.FallbackLLM.Enabled = true
				cfg.FallbackLLM.ProviderType, cfg.FallbackLLM.Model = "openai", "fallback-model"
				cfg.FallbackLLM.BaseURL = provider.URL + "/fallback"
				logger := slog.New(slog.NewTextHandler(io.Discard, nil))
				client := llm.NewFailoverManager(cfg, logger)
				defer client.Stop()
				srv := &Server{Cfg: cfg, LLMClient: client, GameMaker: service, Logger: logger, HistoryManager: memory.NewEphemeralHistoryManager(), Registry: tools.NewProcessRegistry(logger)}
				srv.ShortTermMem, err = memory.NewSQLiteMemory(":memory:", logger)
				if err != nil {
					t.Fatal(err)
				}
				defer srv.ShortTermMem.Close()
				runner := &gameMakerAgentRunner{server: srv, service: service}
				finished := make(chan error, 1)
				service.SetRunner(implementationTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
					if phase != "planning" && run.Stage == "planning" {
						return service.SetDesignJSON(ctx, run.Job.ID, []byte(`{"base":"platformer","objective":"Model routing","features":["Use the selected model"]}`))
					}
					if phase == "source" {
						code, readErr := service.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
						if readErr != nil {
							finished <- readErr
							return readErr
						}
						answer.Store(code)
					} else {
						run.Stage, run.Plan = phase, nil
					}
					finished <- runner.RunGameMakerJob(ctx, run)
					return errors.New("model selection fixture complete")
				}))
				project, err := service.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Selected model", Dimension: "2d", Description: "Build a small game", ProviderID: selection.provider, Model: selection.model})
				if err != nil {
					t.Fatal(err)
				}
				job, err := service.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{})
				if err != nil {
					t.Fatal(err)
				}
				if job.ProviderID != selection.provider || job.Model != selection.model {
					t.Fatal("job lost the project's selected route")
				}
				select {
				case err := <-finished:
					if err != nil {
						t.Fatal(err)
					}
				case <-time.After(20 * time.Second):
					t.Fatal("model selection fixture timed out")
				}
				select {
				case got := <-requests:
					if got.route != selection.wantRoute || got.model != selection.wantModel {
						t.Errorf("wire route/model = %s/%s, want %s/%s", got.route, got.model, selection.wantRoute, selection.wantModel)
					}
					if (got.tools == 0) != (phase == "source") {
						t.Errorf("wrong phase request: tools=%d", got.tools)
					}
				default:
					t.Fatal("no provider request")
				}
				_, activeModel := client.ActiveProviderAndModel()
				if cfg.LLM.Model != "system-model" || cfg.LLM.Provider != "system" || activeModel != "system-model" || !cfg.FallbackLLM.Enabled {
					t.Error("Game Maker changed the global model")
				}
			})
		}
	}
}
