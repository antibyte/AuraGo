package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
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

// Exercise the real agent request construction/fitting, not a replacement job runner.

func TestGameMakerToolCallLimit(t *testing.T) {
	for _, tc := range []struct{ system, want int }{
		{-1, 40}, {0, 40}, {1, 40}, {20, 40}, {31, 40}, {32, 40},
		{33, 42}, {40, 50}, {50, 63}, {51, 64}, {100, 125}, {math.MaxInt, math.MaxInt},
	} {
		if got := gameMakerToolCallLimit(tc.system); got != tc.want {
			t.Errorf("system=%d: got %d, want %d", tc.system, got, tc.want)
		}
	}
}

// Curated SKILL.md files are not injected into Studio runs, so the prompt must
// not claim supplied skills; it still rules out activation and discovery calls.
func TestGameMakerPromptDoesNotClaimUnsuppliedSkills(t *testing.T) {
	for _, stage := range []string{"planning", "building"} {
		for _, dimension := range []string{"2d", "3d"} {
			profile, err := gameMakerPromptProfile(stage, dimension)
			if err != nil {
				t.Fatal(err)
			}
			system := profile.SystemPrompt()
			if strings.Contains(system, "skills are already active") {
				t.Fatalf("%s/%s claims supplied skills", stage, dimension)
			}
		}
	}
}

// A real read travels through dispatch, Guardian isolation, compression and
// bounding. The model must receive plain JSON, not entity-escaped quotes.
func TestGameMakerReadReachesModelReadable(t *testing.T) {
	root := t.TempDir()
	service, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "games.db"), WorkspacePath: filepath.Join(root, "games"), Enabled: true, AllowCreate: true, AllowEdit: true})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	previous := gamemaker.DefaultService()
	gamemaker.SetDefaultService(service)
	defer gamemaker.SetDefaultService(previous)
	service.SetSkillStatus(nil, true)
	var calls atomic.Int32
	var toolMessage atomic.Value
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body openai.ChatCompletionRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "text/event-stream")
		if calls.Add(1) == 1 {
			args, _ := json.Marshal(map[string]any{"operation": "read", "path": "src/common.ts", "start_line": 1, "end_line": 40})
			delta := map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "read-source", "type": "function", "function": map[string]any{"name": "game_maker_file", "arguments": string(args)}}}}
			data, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": "tool_calls"}}})
			fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", data)
			return
		}
		for _, message := range body.Messages {
			if message.Role == openai.ChatMessageRoleTool {
				toolMessage.Store(message.Content)
			}
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer provider.Close()
	cfg := &config.Config{}
	cfg.LLM.Model, cfg.LLM.ProviderType = "test-game-model", "openai"
	cfg.Agent.ContextWindow = 65536
	cfg.CircuitBreaker.LLMTimeoutSeconds = 10
	cfg.GameMaker.Enabled = true
	cfg.Directories.ToolsDir, cfg.Directories.WorkspaceDir = filepath.Join(root, "tools"), root
	clientConfig := openai.DefaultConfig("local-test")
	clientConfig.BaseURL = provider.URL
	server := &Server{Cfg: cfg, LLMClient: llm.WrapOpenAIClient(openai.NewClientWithConfig(clientConfig)), GameMaker: service, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), HistoryManager: memory.NewEphemeralHistoryManager()}
	server.Registry = tools.NewProcessRegistry(server.Logger)
	server.ShortTermMem, err = memory.NewSQLiteMemory(":memory:", server.Logger)
	if err != nil {
		t.Fatal(err)
	}
	defer server.ShortTermMem.Close()
	runner := &gameMakerAgentRunner{server: server, service: service}
	service.SetRunner(implementationTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
		if err := service.UseTemplate(ctx, run.Job.ID, "minimal"); err != nil {
			return err
		}
		_ = runner.RunGameMakerJob(ctx, run)
		return errors.New("read captured")
	}))
	project, err := service.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Readable read", Description: "Reach the flag", Dimension: "2d"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := service.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{Prompt: "Implement the game"})
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(20 * time.Second); ; {
		if done, err := service.GetJob(context.Background(), job.ID); err == nil && done.Status == "failed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("read job did not terminate")
		}
		time.Sleep(5 * time.Millisecond)
	}
	content, _ := toolMessage.Load().(string)
	if !strings.Contains(content, "<external_data>") {
		t.Fatalf("read result was not isolated: %q", content)
	}
	if strings.Contains(content, "&#34;") || strings.Contains(content, "&#39;") || strings.Contains(content, `\u003c`) {
		i := max(max(strings.Index(content, "&#34;"), strings.Index(content, "&#39;")), strings.Index(content, `\u003c`))
		t.Fatalf("read result is still escaped at %d: %.300s", i, content[max(0, i-150):])
	}
	if !strings.Contains(content, `"sha256":"`) || !strings.Contains(content, "import {") {
		t.Fatalf("read result lost plain JSON or source text: %.400s", content)
	}
}

// Real runs spent 7–30 reads before the first edit. The building context hands
// over the current entry file with its sha256 so the first call can be a write.

// Each prompt carries only its own phase and engine: planning cannot edit, 2D
// never sees Three.js integration, 3D never sees Phaser. Guidance is structured
// into short lines instead of multi-thousand-character paragraphs.

// A new 3D plan sees the five guided bases before phase guidance. Free-code
// three stays available for a custom engine and still leaves gameplay unverified.

// Planning a new game has no source to read; voxel planning also needs the
// presentation choices it otherwise looked up in minified bundles.

// Reasoning models spend output before their answer; the allowance for it is
// separate, so the answer reserve survives a long reasoning phase.
func TestGameMakerOutputTokensSeparateReasoning(t *testing.T) {
	qwen := llm.ModelLimits{ContextWindow: 127999, MaxOutputTokens: 127998}
	for _, tc := range []struct {
		name   string
		limits llm.ModelLimits
		answer int
		want   int
	}{
		{"planning", qwen, 4096, 4096 + gameMakerReasoningTokens},
		{"building", qwen, 16384, 16384 + gameMakerReasoningTokens},
		{"building on a large route gets the wide allowance", llm.ModelLimits{ContextWindow: 512000, MaxOutputTokens: 128000}, 16384, 16384 + gameMakerWideReasoningTokens},
		{"planning on a large route keeps the standard allowance", llm.ModelLimits{ContextWindow: 512000, MaxOutputTokens: 128000}, 4096, 4096 + gameMakerReasoningTokens},
		{"building on a 262144 route", llm.ModelLimits{ContextWindow: 262144, MaxOutputTokens: 128000}, 16384, 65536},
		{"building on a 200000 route stays standard", llm.ModelLimits{ContextWindow: 200000, MaxOutputTokens: 64000}, 16384, 32768},
		{"route output caps the sum", llm.ModelLimits{ContextWindow: 127999, MaxOutputTokens: 20000}, 16384, 20000},
		{"small context keeps the general reserve", llm.ModelLimits{ContextWindow: 32768, MaxOutputTokens: 32768}, 16384, 0},
		{"small output keeps the general reserve", llm.ModelLimits{ContextWindow: 127999, MaxOutputTokens: 8192}, 16384, 0},
	} {
		if got := gameMakerOutputTokens(tc.limits, tc.answer); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}

// Small reasoning models are slow because they think long. A Game Maker call
// that keeps streaming past the per-call timeout must be allowed to finish.
func TestGameMakerSlowSteadyStreamOutlastsCallTimeout(t *testing.T) {
	previousPolicy := gameMakerStreamThroughput
	gameMakerStreamThroughput.MinSample = 200 * time.Millisecond
	defer func() { gameMakerStreamThroughput = previousPolicy }()
	root := t.TempDir()
	service, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "games.db"), WorkspacePath: filepath.Join(root, "games"), Enabled: true, AllowCreate: true, AllowEdit: true})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	service.SetSkillStatus(nil, true)
	var finished atomic.Bool
	var requestedMaxTokens atomic.Int32
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
		requestedMaxTokens.Store(int32(request.MaxTokens))
		w.Header().Set("Content-Type", "text/event-stream")
		chunk, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"reasoning_content": strings.Repeat("plan the game ", 8)}}}})
		for end := time.Now().Add(2500 * time.Millisecond); time.Now().Before(end); {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(50 * time.Millisecond):
			}
			fmt.Fprintf(w, "data: %s\n\n", chunk)
			w.(http.Flusher).Flush()
		}
		finished.Store(true)
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer provider.Close()
	cfg := &config.Config{}
	cfg.LLM.Provider, cfg.LLM.Model, cfg.LLM.ProviderType = "fixture", "tencent/hy4-preview", "openai"
	cfg.Providers = []config.ProviderEntry{{ID: "fixture", Type: "openai", Model: cfg.LLM.Model, BaseURL: provider.URL, ContextWindow: 65536, MaxOutputTokens: 32768}}
	cfg.Agent.ContextWindow = 65536
	cfg.CircuitBreaker.LLMTimeoutSeconds = 1
	cfg.CircuitBreaker.LLMStreamChunkTimeoutSeconds = 1
	cfg.GameMaker.Enabled = true
	cfg.Directories.ToolsDir, cfg.Directories.WorkspaceDir = filepath.Join(root, "tools"), root
	cfg.LLM.BaseURL = provider.URL
	client := llm.NewClientFromProviderWithConfig(cfg, "openai", provider.URL, "local-test", "")
	server := &Server{Cfg: cfg, LLMClient: llm.WrapOpenAIClient(client), GameMaker: service, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), HistoryManager: memory.NewEphemeralHistoryManager()}
	server.Registry = tools.NewProcessRegistry(server.Logger)
	server.ShortTermMem, err = memory.NewSQLiteMemory(":memory:", server.Logger)
	if err != nil {
		t.Fatal(err)
	}
	defer server.ShortTermMem.Close()
	runner := &gameMakerAgentRunner{server: server, service: service}
	done := make(chan error, 1)
	service.SetRunner(implementationTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
		done <- runner.RunGameMakerJob(ctx, run)
		return errors.New("stream observed")
	}))
	project, err := service.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Slow model", Description: "Reach the flag", Dimension: "2d"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{Prompt: "Plan the game"}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !finished.Load() || err != nil && strings.Contains(err.Error(), "deadline") {
			t.Fatalf("slow steady stream was cut off: finished=%v err=%v", finished.Load(), err)
		}
		if got := requestedMaxTokens.Load(); got != 8192 {
			t.Fatalf("provider received max_tokens=%d, want provider ceiling 8192", got)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("planning round did not end")
	}
}
