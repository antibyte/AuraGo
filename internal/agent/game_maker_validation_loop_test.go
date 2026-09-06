package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/gamemaker"

	openai "github.com/sashabaranov/go-openai"
)

func TestGameMakerRepairsEndAtServerBoundary(t *testing.T) {
	cfg, _, cleanup := newPromptPipelineTestRunConfig(t, "game-maker-validation", "game_maker")
	defer cleanup()
	cfg.Config.LLM.UseNativeFunctions = true
	cfg.Config.GameMaker.Enabled = true
	cfg.Config.CircuitBreaker.MaxToolCalls = 2
	cfg.AllowedTools = []string{"game_maker_file", "game_maker_validate"}
	cfg.SuppressTurnSideEffects, cfg.IsMission = true, true
	root := t.TempDir()
	s, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "game.db"), WorkspacePath: filepath.Join(root, "workspace"), Enabled: true, AllowCreate: true, AllowEdit: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	previous := gamemaker.DefaultService()
	gamemaker.SetDefaultService(s)
	defer gamemaker.SetDefaultService(previous)
	s.SetSkillStatus(nil, true)
	project, err := s.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Breakout", Dimension: "2d", Description: "Breakout with power-ups"})
	if err != nil {
		t.Fatal(err)
	}
	rounds := 0
	s.SetRunner(gameMakerPlanTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
		if run.Stage == "planning" {
			return s.SetPlan(ctx, run.Job.ID, gamemaker.ExampleGamePlan(project))
		}
		rounds++
		cfg.SessionID = fmt.Sprintf("game-maker-validation-%d", rounds)
		if err := s.WriteJobFile(ctx, run.Job.ID, "src/main.ts", fmt.Sprintf("invalid TypeScript <<< %d", rounds)); err != nil {
			return err
		}
		if run.Stage == "building" {
			return nil // The server detects the initial failure and starts repair.
		}
		call := func(id, name string, args map[string]any) openai.ToolCall {
			args["job_id"] = run.Job.ID
			data, _ := json.Marshal(args)
			return openai.ToolCall{ID: id, Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: name, Arguments: string(data)}}
		}
		validate := call("validate", "game_maker_validate", map[string]any{"scope": "full"})
		tail := call("tail", "game_maker_file", map[string]any{"operation": "write", "path": "src/forbidden.ts", "content": "// must not dispatch"})
		client := &circuitBreakerSequenceClient{responses: []openai.ChatCompletionResponse{{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{validate, tail}}}}}}}
		cfg.LLMClient = client
		cfg.RunComplete = s.StopAfterValidation(run.Job.ID, run.Stage == "repair")
		_, err := ExecuteAgentLoop(ctx, openai.ChatCompletionRequest{Model: cfg.Config.LLM.Model, Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: project.Description}}}, cfg, false, NoopBroker{})
		if err != nil {
			return err
		}
		if len(client.requests) != 1 {
			return fmt.Errorf("continued model calls after validation: %d", len(client.requests))
		}
		if _, err := s.ReadJobFile(ctx, run.Job.ID, "src/forbidden.ts"); err == nil {
			return fmt.Errorf("trailing native write executed after validation")
		}
		messages, _ := cfg.ShortTermMem.GetSessionMessagesForBridge(cfg.SessionID)
		skipped := 0
		for _, message := range messages {
			if strings.Contains(message.Content, "not_executed_after_run_completion") {
				skipped++
			}
		}
		if skipped != 1 {
			return fmt.Errorf("trailing call must receive one skipped result, got %d", skipped)
		}
		return nil
	}))
	job, err := s.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(10 * time.Second); ; {
		finished, err := s.GetJob(context.Background(), job.ID)
		if err == nil && finished.Status == "failed" {
			if rounds != 4 || !strings.HasPrefix(finished.Error, "game validation failed:") || strings.Contains(finished.Error, "repair_limit_reached") || finished.ResultRevision != 0 {
				t.Fatalf("wrong bounded outcome: rounds=%d, job=%+v", rounds, finished)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("validation rounds did not terminate")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
