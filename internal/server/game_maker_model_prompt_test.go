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
	"aurago/internal/memory"
	"aurago/internal/tools"

	openai "github.com/sashabaranov/go-openai"
)

// Exercise the real agent request construction/fitting, not a replacement job runner.
func TestGameMakerModelContractReachesAgentRequest(t *testing.T) {
	root := t.TempDir()
	service, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "games.db"), WorkspacePath: filepath.Join(root, "games"), Enabled: true, AllowCreate: true, AllowEdit: true})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	service.SetSkillStatus(nil, true)
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
		fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":%q,\"reasoning_content\":\"Keep the forest and FPS controls\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", content)
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
	project, err := service.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "FPS", Dimension: "3d", Description: "FPS shooter"})
	if err != nil {
		t.Fatal(err)
	}
	invoke := func(ctx context.Context, fixture gamemaker.JobRun) error {
		result := make(chan error, 1)
		service.SetRunner(implementationTestRunner(func(ctx context.Context, actual gamemaker.JobRun) error {
			fixture.Job, fixture.Project = actual.Job, actual.Project
			result <- runner.RunGameMakerJob(ctx, fixture)
			return errors.New("fixture complete")
		}))
		_, err := service.StartJob(ctx, project.ID, gamemaker.StartJobRequest{Prompt: "continue", Resume: true})
		if err != nil {
			return err
		}
		select {
		case err = <-result:
		case <-ctx.Done():
			return ctx.Err()
		}
		for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
			if service.ActiveJobInfo(ctx) == nil {
				break
			}
			time.Sleep(time.Millisecond)
		}
		return err
	}
	for _, stage := range []string{"building", "repair"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := invoke(ctx, gamemaker.JobRun{
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
		priorReasoning := false
		for _, message := range request.Messages {
			priorReasoning = priorReasoning || message.ReasoningContent == "Keep the forest and FPS controls"
			if message.Role == "system" {
				system.WriteString(message.Content)
			} else if message.Role == "user" {
				user.WriteString(message.Content)
			}
		}
		if !strings.Contains(user.String(), "Original game request:\nFPS shooter") || (stage == "repair" && !priorReasoning) {
			t.Fatal("continuation lost the original request or provider reasoning")
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
	project, err = service.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Harbour", Dimension: "2d", Description: "Isometric harbour"})
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"planning", "building"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := invoke(ctx, gamemaker.JobRun{Stage: stage, AssetPacks: []gamemaker.ImportedAssetPack{{
			ID: "aurago-isometric", Kind: "sprite2d", Version: "1.0.0",
			Manifests: map[string]string{"people-adventurer-1": "assets/builtin/aurago-isometric/1.0.0/assets/people-adventurer-1.json"},
		}}})
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		request := <-requests
		var system strings.Builder
		for _, message := range request.Messages {
			if message.Role == "system" {
				system.WriteString(message.Content)
			}
		}
		for _, required := range []string{"schema_version:2 atlases", "variable frames", "setFacing", "exact imported manifest path", "playAction(art, 'idle')", "GameScene.step(deltaSeconds) already receives seconds", "never divide by 1000 again"} {
			if !strings.Contains(system.String(), required) {
				t.Errorf("%s lost the atlas contract: %s", stage, required)
			}
		}
		for _, obsolete := range []string{"preloadPack loads exact 64x64 frames", "Never load a built-in sheet as one image or use atlas JSON"} {
			if strings.Contains(system.String(), obsolete) {
				t.Errorf("%s still contradicts atlas support: %s", stage, obsolete)
			}
		}
		if stage == "planning" && !strings.Contains(system.String(), gamemaker.PresentationPlanningGuide) {
			t.Error("planning lost atmosphere/effect distinction and supported sound events")
		}
	}
	cfg.LLM.Model = "empty-game-model"
	for _, stage := range []string{"building", "repair"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := invoke(ctx, gamemaker.JobRun{
			Stage: stage, Job: gamemaker.Job{ID: "job_empty_" + stage, Prompt: "a fps game in the woods"},
			Project: gamemaker.Project{ID: "empty-project", Dimension: "2d"},
		})
		cancel()
		if err == nil || !strings.Contains(err.Error(), "empty response") {
			t.Errorf("%s treated empty provider output as success: %v", stage, err)
		}
	}
}

func TestCompactGameMakerContextPreservesExamplesAndFirstFailure(t *testing.T) {
	plan := &gamemaker.GamePlan{
		SchemaVersion: 4,
		Template:      "minimal",
		Objective:     "Explore",
		Scene: &gamemaker.Scene{
			SchemaVersion: 1, Dimension: "2d", Seed: 7,
			Levels: []gamemaker.SceneLevel{{ID: "main", Active: true}},
			Placements: []gamemaker.ScenePlacement{{
				ID: "player_art", NodeID: "player", AssetID: "hero",
				AssetRole: "player", Behavior: "controlled",
			}},
		},
		Mechanics: map[string]any{"outcomes": []any{"won"}},
	}
	run := gamemaker.JobRun{
		Stage: "repair", Plan: plan,
		Checks:      []gamemaker.CheckResult{{ID: "required_rules", Status: "failed", Expected: "hits increased", Observed: "before=0, after=0"}},
		Diagnostics: []gamemaker.Diagnostic{{Level: "error", File: "src/main.ts", Line: 9, Message: "compile failure"}},
		AssetPacks: []gamemaker.ImportedAssetPack{{
			ID: gamemaker.ModelPackID, Version: "1", Kind: "model3d",
			AssetIDs: []string{"hero"}, Manifests: map[string]string{"hero": "assets/hero.json"},
			ThreeExample: "EXECUTABLE_THREE_EXAMPLE",
		}},
	}
	data := compactGameMakerContext(run)
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, want := range []string{"EXECUTABLE_THREE_EXAMPLE", "required_rules", "first_failure", "rules_status", "player_art", "outcomes"} {
		if !strings.Contains(text, want) {
			t.Fatalf("compact repair context lost %s: %s", want, text)
		}
	}
	if strings.Contains(text, "\"nodes\":[") || strings.Contains(text, "\"regions\":[") {
		t.Fatalf("repair context included full scene arrays: %s", text)
	}
}

func TestGameMakerToolLimitStillValidatesSavedSource(t *testing.T) {
	for _, dimension := range []string{"2d", "3d"} {
		t.Run(dimension, func(t *testing.T) {
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
			cfg := &config.Config{}
			cfg.LLM.Model, cfg.LLM.ProviderType = "test-game-model", "openai"
			cfg.Agent.ContextWindow = 65536
			cfg.CircuitBreaker.LLMTimeoutSeconds, cfg.CircuitBreaker.MaxToolCalls = 10, 1
			cfg.GameMaker.Enabled = true
			cfg.Directories.ToolsDir, cfg.Directories.WorkspaceDir = filepath.Join(root, "tools"), root
			var calls, finalCalls atomic.Int32
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request openai.ChatCompletionRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				calls.Add(1)
				w.Header().Set("Content-Type", "text/event-stream")
				if len(request.Tools) > 0 {
					// The system limit is one, but Game Maker guarantees 40 calls.
					batch := make([]map[string]any, 40)
					for i := range batch {
						args, _ := json.Marshal(map[string]any{"operation": "write", "path": "src/main.ts", "content": fmt.Sprintf("// tool-%d\nconst broken = ;", i+1)})
						batch[i] = map[string]any{"index": i, "id": fmt.Sprintf("write-%d", i), "type": "function", "function": map[string]any{"name": "game_maker_file", "arguments": string(args)}}
					}
					encoded, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "tool_calls": batch}, "finish_reason": "tool_calls"}}})
					fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", encoded)
					return
				}
				finalCalls.Add(1)
				code := "export const forbiddenExtraWrite = 1;"
				content := `<tool_call><function=game_maker_file><parameter=operation>write</parameter><parameter=path>src/main.ts</parameter><parameter=content>` + code + `</parameter></function></tool_call>`
				encoded, _ := json.Marshal(content)
				fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":%s},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", encoded)
			}))
			defer provider.Close()
			clientConfig := openai.DefaultConfig("local-test")
			clientConfig.BaseURL = provider.URL
			server := &Server{Cfg: cfg, LLMClient: openai.NewClientWithConfig(clientConfig), GameMaker: service, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), HistoryManager: memory.NewEphemeralHistoryManager()}
			server.Registry = tools.NewProcessRegistry(server.Logger)
			server.ShortTermMem, err = memory.NewSQLiteMemory(":memory:", server.Logger)
			if err != nil {
				t.Fatal(err)
			}
			defer server.ShortTermMem.Close()
			runner := &gameMakerAgentRunner{server: server, service: service}
			// Planning must not accept a missing design at the same tool limit.
			unplanned, err := service.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "No plan", Dimension: dimension, Description: "Create a game"})
			if err != nil {
				t.Fatal(err)
			}
			service.SetRunner(runner)
			unplannedJob, err := service.StartJob(context.Background(), unplanned.ID, gamemaker.StartJobRequest{})
			if err != nil {
				t.Fatal(err)
			}
			for deadline := time.Now().Add(10 * time.Second); ; {
				done, err := service.GetJob(context.Background(), unplannedJob.ID)
				if err == nil && done.Status == "failed" && service.ActiveJobInfo(context.Background()) == nil {
					if !strings.Contains(done.Error, "tool_limit_final_response_invalid") {
						t.Fatalf("planning incorrectly recovered: %s", done.Error)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("planning did not stop at tool limit")
				}
				time.Sleep(time.Millisecond)
			}
			calls.Store(0)
			finalCalls.Store(0)
			project, err := service.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Bounded work", Description: "Custom challenge", Dimension: dimension})
			if err != nil {
				t.Fatal(err)
			}
			service.SetRunner(implementationTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
				if run.Stage == "planning" {
					base := "minimal"
					if dimension == "3d" {
						base = "three"
					}
					return service.SetDesignJSON(ctx, run.Job.ID, []byte(fmt.Sprintf(`{"base":%q,"objective":"Custom challenge","features":["Custom mechanic"]}`, base)))
				}
				if err := runner.RunGameMakerJob(ctx, run); err != nil {
					return err
				}
				code, err := service.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
				if err != nil || !strings.Contains(code, "// tool-40\nconst broken = ;") || strings.Contains(code, "forbiddenExtraWrite") {
					return fmt.Errorf("extra call changed source: %q, %v", code, err)
				}
				if run.Stage == "repair" {
					if len(run.Diagnostics) == 0 || !strings.Contains(run.Diagnostics[0].Message, "Unexpected") {
						return fmt.Errorf("compiler failure did not reach repair: %+v", run.Diagnostics)
					}
					return errors.New("verified validation handoff")
				}
				return nil
			}))
			job, err := service.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{Prompt: "Build a custom game"})
			if err != nil {
				t.Fatal(err)
			}
			for deadline := time.Now().Add(20 * time.Second); ; {
				done, err := service.GetJob(context.Background(), job.ID)
				if err == nil && done.Status == "failed" {
					if done.Error != "verified validation handoff" || done.ResultRevision != 0 || calls.Load() != 4 || finalCalls.Load() != 2 {
						t.Fatalf("unexpected result: %+v, requests=%d, final=%d", done, calls.Load(), finalCalls.Load())
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("tool limit did not terminate the job")
				}
				time.Sleep(5 * time.Millisecond)
			}
			if cfg.CircuitBreaker.MaxToolCalls != 1 {
				t.Fatal("Game Maker changed the global system limit")
			}
		})
	}
}

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

func TestGameMakerEmptyFinalValidatesOnlyNewlySavedWork(t *testing.T) {
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
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if calls.Add(1) == 1 {
			args, _ := json.Marshal(map[string]any{"operation": "write", "path": "src/main.ts", "content": "const broken = ;"})
			delta := map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "saved-edit", "type": "function", "function": map[string]any{"name": "game_maker_file", "arguments": string(args)}}}}
			data, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": "tool_calls"}}})
			fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", data)
			return
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
	server := &Server{Cfg: cfg, LLMClient: openai.NewClientWithConfig(clientConfig), GameMaker: service, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), HistoryManager: memory.NewEphemeralHistoryManager()}
	server.Registry = tools.NewProcessRegistry(server.Logger)
	server.ShortTermMem, err = memory.NewSQLiteMemory(":memory:", server.Logger)
	if err != nil {
		t.Fatal(err)
	}
	defer server.ShortTermMem.Close()
	runner := &gameMakerAgentRunner{server: server, service: service}
	service.SetRunner(implementationTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
		if run.Stage == "planning" {
			return service.SetDesignJSON(ctx, run.Job.ID, []byte(`{"base":"minimal","objective":"Custom challenge","features":["Custom mechanic"]}`))
		}
		if run.Stage == "repair" {
			if len(run.Diagnostics) == 0 || !strings.Contains(run.Diagnostics[0].Message, "Unexpected") {
				return fmt.Errorf("saved invalid source bypassed validation: %+v", run.Diagnostics)
			}
			// Old building writes cannot count as progress in a later empty repair.
			if err := runner.RunGameMakerJob(ctx, run); err == nil || !strings.Contains(err.Error(), "empty response") {
				return fmt.Errorf("empty repair reused earlier writes: %v", err)
			}
			return errors.New("verified empty-final validation handoff")
		}
		return runner.RunGameMakerJob(ctx, run)
	}))
	project, err := service.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Empty final", Description: "Custom challenge", Dimension: "2d"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := service.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{Prompt: "Implement the game"})
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(20 * time.Second); ; {
		done, err := service.GetJob(context.Background(), job.ID)
		if err == nil && done.Status == "failed" {
			if done.Error != "verified empty-final validation handoff" || done.ResultRevision != 0 {
				t.Fatalf("unexpected completion: %+v", done)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("empty-final job did not terminate")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
