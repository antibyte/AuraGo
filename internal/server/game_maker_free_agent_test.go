package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestFreeGameMakerModelImageDelivery(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 for Chromium")
	}
	for _, mode := range []string{"native", "vision", "missing_vision"} {
		t.Run(mode, func(t *testing.T) {
			var codingCalls, visionCalls atomic.Int32
			var sawImage, sawDescription, sawMissing atomic.Bool
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				var request openai.ChatCompletionRequest
				if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				imageCount := 0
				for _, message := range request.Messages {
					if strings.Contains(message.Content, "data:image/") {
						t.Error("pixels leaked into text content")
					}
					if strings.Contains(message.Content, "visible blue square") {
						sawDescription.Store(true)
					}
					if strings.Contains(message.Content, "configured vision provider") {
						sawMissing.Store(true)
					}
					for _, part := range message.MultiContent {
						if part.Type == openai.ChatMessagePartTypeImageURL && part.ImageURL != nil && strings.HasPrefix(part.ImageURL.URL, "data:image/jpeg;base64,") {
							imageCount++
						}
					}
				}
				if request.Model == "fixture-vision" {
					visionCalls.Add(1)
					if imageCount != 1 || len(request.Tools) != 0 {
						t.Errorf("vision request images=%d tools=%d", imageCount, len(request.Tools))
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: "assistant", Content: "A visible blue square and a counter."}, FinishReason: openai.FinishReasonStop}}})
					return
				}
				if request.Model != "fixture-coding" {
					t.Errorf("selected model replaced: %s", request.Model)
				}
				if mode == "native" && imageCount > 0 {
					sawImage.Store(true)
				}
				if mode != "native" && imageCount > 0 {
					t.Error("text-only coding route received pixels")
				}
				if len(request.Tools) != 4 {
					t.Errorf("tool count=%d", len(request.Tools))
				}
				for _, tool := range request.Tools {
					if tool.Function.Name == "game_maker_validate" {
						t.Error("old validation advertised")
					}
				}
				n := codingCalls.Add(1)
				name, args := "game_maker_preview", `{"operation":"open"}`
				if n == 2 {
					args = `{"operation":"screenshot"}`
				}
				if n >= 3 {
					name, args = "game_maker_project", `{"operation":"finish"}`
				}
				event := map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": fmt.Sprintf("call-%d", n), "type": "function", "function": map[string]any{"name": name, "arguments": args}}}}, "finish_reason": "tool_calls"}}}
				b, _ := json.Marshal(event)
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", b)
			}))
			defer provider.Close()
			root := t.TempDir()
			svc, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "games.db"), WorkspacePath: filepath.Join(root, "games"), Enabled: true, AllowCreate: true, AllowEdit: true, JobTimeout: 90 * time.Second})
			if err != nil {
				t.Fatal(err)
			}
			defer svc.Close()
			oldService := gamemaker.DefaultService()
			gamemaker.SetDefaultService(svc)
			defer gamemaker.SetDefaultService(oldService)
			manual := false
			p := config.ProviderEntry{ID: "coding", Type: "openai", BaseURL: provider.URL + "/v1", APIKey: "synthetic-fixture", Model: "fixture-coding", ContextWindow: 32768, MaxOutputTokens: 4096, Capabilities: config.ProviderCapabilities{Auto: &manual, ToolCalling: true, Multimodal: mode == "native"}}
			cfg := &config.Config{Providers: []config.ProviderEntry{p}}
			cfg.LLM.Provider = "coding"
			if mode == "vision" {
				v := p
				v.ID, v.Model = "vision", "fixture-vision"
				v.Capabilities.Multimodal = true
				cfg.Providers = append(cfg.Providers, v)
				cfg.Vision.Provider = "vision"
			}
			cfg.ResolveProviders()
			cfg.Agent.ContextWindow = 32768
			cfg.CircuitBreaker.LLMTimeoutSeconds = 30
			cfg.CircuitBreaker.MaxToolCalls = 10
			cfg.GameMaker.Enabled = true
			cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir = root, filepath.Join(root, "tools")
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			client := llm.NewFailoverManager(cfg, logger)
			defer client.Stop()
			server := &Server{Cfg: cfg, LLMClient: client, GameMaker: svc, Logger: logger, HistoryManager: memory.NewEphemeralHistoryManager(), Registry: tools.NewProcessRegistry(logger)}
			server.ShortTermMem, err = memory.NewSQLiteMemory(":memory:", logger)
			if err != nil {
				t.Fatal(err)
			}
			defer server.ShortTermMem.Close()
			runner := &gameMakerAgentRunner{server: server, service: svc}
			svc.SetRunner(implementationTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
				if run.Stage != "building" {
					return fmt.Errorf("unexpected phase %s", run.Stage)
				}
				if err := svc.WriteJobFile(ctx, run.Job.ID, "index.html", `<!doctype html><div style="width:100px;height:100px;background:blue">0</div>`); err != nil {
					return err
				}
				return runner.RunGameMakerJob(ctx, run)
			}))
			project, err := svc.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Image delivery", Dimension: "2d", Description: "Show a blue square", ProviderID: "coding", Model: "fixture-coding"})
			if err != nil {
				t.Fatal(err)
			}
			job, err := svc.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{})
			if err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(100 * time.Second)
			for time.Now().Before(deadline) {
				job, err = svc.GetJob(context.Background(), job.ID)
				if err != nil {
					t.Fatal(err)
				}
				if job.Status == "ready" || job.Status == "failed" || job.Status == "cancelled" {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			if job.Status != "ready" || job.ResultRevision != 1 {
				t.Fatalf("job=%+v", job)
			}
			if codingCalls.Load() != 3 {
				t.Errorf("coding calls=%d", codingCalls.Load())
			}
			switch mode {
			case "native":
				if !sawImage.Load() || visionCalls.Load() != 0 {
					t.Fatal("native image was not delivered directly")
				}
			case "vision":
				if visionCalls.Load() != 1 || !sawDescription.Load() {
					t.Fatal("vision description did not reach coding agent")
				}
			case "missing_vision":
				if !sawMissing.Load() || visionCalls.Load() != 0 {
					t.Fatal("missing vision did not produce recoverable tool error")
				}
			}
		})
	}
}
