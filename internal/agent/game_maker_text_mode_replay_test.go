package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/gamemaker"

	openai "github.com/sashabaranov/go-openai"
)

type textModeReplayResult struct {
	events      []gamemaker.Event
	shown       string // the conversation the model last saw, tool results included
	accepted    bool
	nativeTools int // tool schemas sent with the first request; 0 in text mode
}

// replayGameMaker exercises text-mode tool calls through a real bound job.
func replayGameMaker(t *testing.T, dimension string, replies []string, tune ...func(*RunConfig)) textModeReplayResult {
	t.Helper()
	runCfg, _, cleanup := newPromptPipelineTestRunConfig(t, "game-maker-replay-"+t.Name(), "game_maker")
	defer cleanup()
	runCfg.Config.LLM.UseNativeFunctions = false
	runCfg.Config.LLM.Model = "glm-4"
	runCfg.Config.GameMaker.Enabled = true
	runCfg.Config.CircuitBreaker.MaxToolCalls = 12
	runCfg.SuppressTurnSideEffects = true
	runCfg.IsMission = true
	runCfg.AllowedTools = nil
	for _, definition := range GameMakerPhaseToolSchemas("building", dimension) {
		runCfg.AllowedTools = append(runCfg.AllowedTools, definition.Function.Name)
	}
	for _, adjust := range tune {
		adjust(&runCfg)
	}
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
	project, err := s.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Replay", Dimension: dimension, Description: "Replay of a free agent job"})
	if err != nil {
		t.Fatal(err)
	}
	var responses []openai.ChatCompletionResponse
	for _, reply := range append(append([]string(nil), replies...), "Done.", "Done.", "Done.") {
		responses = append(responses, openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: reply}, FinishReason: openai.FinishReasonStop}}})
	}
	done := make(chan textModeReplayResult, 1)
	s.SetRunner(gameMakerPlanTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
		if run.Stage != "building" {
			return fmt.Errorf("replay reached %s", run.Stage)
		}
		client := &circuitBreakerSequenceClient{responses: responses}
		runCfg.LLMClient = client
		runCfg.RunComplete = func() bool { return s.JobFinished(run.Job.ID) }
		_, loopErr := ExecuteAgentLoop(gamemaker.WithJobContext(ctx, run.Job.ID), openai.ChatCompletionRequest{Model: runCfg.Config.LLM.Model, Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: project.Description}}}, runCfg, false, NoopBroker{})
		result := textModeReplayResult{accepted: s.JobFinished(run.Job.ID)}
		result.events, _ = s.EventsAfter(context.Background(), project.ID, 0, 500)
		if len(client.requests) > 0 {
			result.nativeTools = len(client.requests[0].Tools)
			for _, m := range client.requests[len(client.requests)-1].Messages {
				if m.Role != openai.ChatMessageRoleSystem {
					result.shown += m.Content + "\n"
				}
			}
		}
		done <- result
		return loopErr
	}))
	if _, err := s.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{}); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-done:
		return result
	case <-time.After(20 * time.Second):
		t.Fatal("planning replay did not finish")
	}
	return textModeReplayResult{}
}

// toolResults returns "tool/operation:status" of every tool_result the service
// emitted, i.e. of every call that actually reached a Game Maker tool. A call
// refused by the scope or as an unknown tool never gets that far.
func (r textModeReplayResult) toolResults() []string {
	var out []string
	for _, event := range r.events {
		if event.Type == "tool_result" {
			out = append(out, fmt.Sprintf("%v/%v:%v", event.Payload["tool"], event.Payload["operation"], event.Payload["status"]))
		}
	}
	return out
}

func requireTextModeWithoutDenial(t *testing.T, r textModeReplayResult) {
	t.Helper()
	if r.nativeTools != 0 {
		t.Fatalf("the replay must run in text tool-call mode, but %d native tool schemas were sent", r.nativeTools)
	}
	for _, refusal := range []string{"tool_scope_denied", "unknown_tool", "outside this run's scope", "is not allowed in this agent run"} {
		if strings.Contains(r.shown, refusal) {
			t.Fatalf("a recorded call was refused (%s); the model saw:\n%s", refusal, r.shown)
		}
	}
}

func TestGameMakerTextModeReplay(t *testing.T) {
	result := replayGameMaker(t, "2d", []string{
		`{"tool":"game_maker_file","arguments":{"operation":"write","path":"index.html","content":"<!doctype html><h1>Free game</h1>"}}`,
		`{"tool":"game_maker_project","arguments":{"operation":"build"}}`,
		`{"tool":"finish","arguments":{}}`,
	})
	requireTextModeWithoutDenial(t, result)
	if !result.accepted {
		t.Fatalf("finish was not accepted: %s", result.shown)
	}
}
