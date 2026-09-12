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

func TestGameMakerPlanningEndsAtServerBoundary(t *testing.T) {
	for _, mode := range []string{"native", "batched", "xml", "correction", "exhausted", "schema_exhausted", "without_boundary"} {
		t.Run(mode, func(t *testing.T) {
			runCfg, _, cleanup := newPromptPipelineTestRunConfig(t, "game-maker-phase-"+mode, "game_maker")
			defer cleanup()
			runCfg.Config.LLM.UseNativeFunctions = true
			runCfg.Config.GameMaker.Enabled = true
			runCfg.Config.CircuitBreaker.MaxToolCalls = 3
			runCfg.AllowedTools = []string{"game_maker_project", "game_maker_file"}
			runCfg.SuppressTurnSideEffects = true
			runCfg.IsMission = true
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
			project, err := s.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Bout", Dimension: "2d", Description: "Breakout with power-ups and sounds"})
			if err != nil {
				t.Fatal(err)
			}
			loopResult := make(chan error, 1)
			s.SetRunner(gameMakerPlanTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
				if run.Stage != "planning" {
					return fmt.Errorf("test reached building")
				}
				call := func(id, name string, args map[string]any) openai.ToolCall {
					args["job_id"] = run.Job.ID
					data, _ := json.Marshal(args)
					return openai.ToolCall{ID: id, Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: name, Arguments: string(data)}}
				}
				plan := gamemaker.ExampleGamePlan(project)
				plan.Template = "blocks"
				plan.Objective = project.Description
				data, _ := json.Marshal(plan)
				planCall := call("plan", "game_maker_project", map[string]any{"operation": "set_plan", "plan": string(data)})
				tail := call("tail", "game_maker_file", map[string]any{"operation": "read", "path": "src/main.ts"})
				message := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{planCall, tail}}
				if mode == "batched" || mode == "without_boundary" {
					inspect := call("inspect", "game_maker_project", map[string]any{"operation": "list_files"})
					message.ToolCalls = append([]openai.ToolCall{inspect}, message.ToolCalls...)
				}
				if mode == "xml" {
					message.ToolCalls = nil
					message.Content = "<tool_call><function=game_maker_project><parameter=job_id>" + run.Job.ID + "</parameter><parameter=operation>set_plan</parameter><parameter=plan>" + string(data) + "</parameter></function></tool_call>"
				}
				response := func(m openai.ChatCompletionMessage) openai.ChatCompletionResponse {
					return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: m, FinishReason: openai.FinishReasonStop}}}
				}
				responses := []openai.ChatCompletionResponse{}
				exhausted := mode == "exhausted" || mode == "schema_exhausted"
				if mode == "correction" || exhausted {
					attempts := 1
					if exhausted {
						attempts = 3
					}
					for i := 0; i < attempts; i++ {
						plan.SchemaVersion = i + 5 // Distinct invalid plans; versions 1–4 are supported.
						var payload any = plan
						if mode == "schema_exhausted" {
							// Unknown fields must survive native parsing and exhaust
							// planning, rather than disappearing into a typed struct.
							payload = strings.TrimSuffix(string(data), "}") + fmt.Sprintf(`,"unknown_%d":true}`, i)
						}
						bad := call(fmt.Sprintf("bad-%d", i), "game_maker_project", map[string]any{"operation": "set_plan", "plan": payload})
						responses = append(responses, response(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{bad}}))
					}
				}
				if !exhausted {
					responses = append(responses, response(message))
				}
				expectedRequests := len(responses)
				// A model continuing after completion reproduces the reported error.
				responses = append(responses, response(openai.ChatCompletionMessage{ToolCalls: []openai.ToolCall{tail}}))
				client := &circuitBreakerSequenceClient{responses: responses}
				runCfg.LLMClient = client
				if mode != "without_boundary" {
					runCfg.RunComplete = func() bool { return s.PlanningComplete(run.Job.ID) }
				}
				_, loopErr := ExecuteAgentLoop(ctx, openai.ChatCompletionRequest{Model: runCfg.Config.LLM.Model, Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: project.Description}}}, runCfg, false, NoopBroker{})
				if mode == "without_boundary" {
					if !IsToolLimitFinalResponseInvalid(loopErr) {
						loopResult <- fmt.Errorf("control did not reproduce reported error: %v", loopErr)
					} else {
						loopResult <- nil
					}
					return loopErr
				}
				if loopErr == nil && len(client.requests) != expectedRequests {
					loopErr = fmt.Errorf("made %d LLM requests, want %d", len(client.requests), expectedRequests)
				}
				messages, _ := runCfg.ShortTermMem.GetSessionMessagesForBridge(runCfg.SessionID)
				skipped := 0
				for _, m := range messages {
					if strings.Contains(m.Content, "not_executed_after_run_completion") {
						skipped++
					}
				}
				if mode != "xml" && !exhausted && skipped != 1 {
					loopErr = fmt.Errorf("trailing call was not skipped exactly once: %d", skipped)
				}
				loopResult <- loopErr
				return loopErr
			}))
			job, err := s.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{})
			if err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-loopResult:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("planning loop did not terminate")
			}
			for deadline := time.Now().Add(3 * time.Second); ; {
				finished, err := s.GetJob(context.Background(), job.ID)
				if err == nil && finished.Status == "failed" {
					want := "test reached building"
					if mode == "exhausted" {
						want = "plan.schema_version"
					} else if mode == "schema_exhausted" {
						want = `unknown field "unknown_2"`
					}
					if mode != "without_boundary" && !strings.Contains(finished.Error, want) {
						t.Fatalf("wrong phase outcome: %+v", finished)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("job did not finish")
				}
				time.Sleep(5 * time.Millisecond)
			}
		})
	}
}
