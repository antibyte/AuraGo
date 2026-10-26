package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"aurago/internal/gamemaker"

	openai "github.com/sashabaranov/go-openai"
)

// The fixture holds assistant replies recorded verbatim from two failed
// production planning jobs (MiniMax M3, GLM-5.3-Flash; text-mode tool calls
// that used operation names as tool names and were refused as out of scope).
type textModeReplayFixture struct {
	Source string `json:"source"`
	Date   string `json:"date"`
	Models []struct {
		Model     string            `json:"model"`
		Dimension string            `json:"dimension"`
		Replies   map[string]string `json:"replies"`
		Sequence  []string          `json:"sequence"`
	} `json:"models"`
}

func loadTextModeReplayFixture(t *testing.T) textModeReplayFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "game_maker_replay", "text_mode_planning.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture textModeReplayFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Source != "production replay" || fixture.Date != "2026-10" || len(fixture.Models) != 2 {
		t.Fatalf("fixture labels: %+v", fixture)
	}
	return fixture
}

type textModeReplayResult struct {
	events      []gamemaker.Event
	shown       string // the conversation the model last saw, tool results included
	accepted    bool
	nativeTools int // tool schemas sent with the first request; 0 in text mode
}

// replayGameMakerPlanning runs the planning phase of a real Game Maker job in
// text tool-call mode, answering each model turn with the next recorded reply
// and then with plain prose. The model name is pinned to a GLM id so text mode
// does not depend on the MiniMax name rule.
func replayGameMakerPlanning(t *testing.T, dimension string, replies []string, tune ...func(*RunConfig)) textModeReplayResult {
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
	for _, definition := range GameMakerPhaseToolSchemas("planning", dimension) {
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
	project, err := s.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Replay", Dimension: dimension, Description: "Replay of a recorded planning job"})
	if err != nil {
		t.Fatal(err)
	}
	var responses []openai.ChatCompletionResponse
	for _, reply := range append(append([]string(nil), replies...), "Planning is done.", "Planning is done.", "Planning is done.") {
		responses = append(responses, openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: reply}, FinishReason: openai.FinishReasonStop}}})
	}
	done := make(chan textModeReplayResult, 1)
	s.SetRunner(gameMakerPlanTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
		if run.Stage != "planning" {
			return fmt.Errorf("replay reached %s", run.Stage)
		}
		client := &circuitBreakerSequenceClient{responses: responses}
		runCfg.LLMClient = client
		runCfg.RunComplete = func() bool { return s.PlanningComplete(run.Job.ID) }
		_, loopErr := ExecuteAgentLoop(gamemaker.WithJobContext(ctx, run.Job.ID), openai.ChatCompletionRequest{Model: runCfg.Config.LLM.Model, Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: project.Description}}}, runCfg, false, NoopBroker{})
		result := textModeReplayResult{accepted: s.PlanningComplete(run.Job.ID)}
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

// Each model's recorded first set_design must reach the service as
// game_maker_project/set_design: accepted, or rejected by design validation
// with issues. The GLM design is valid. The MiniMax design is rejected only
// for its assets: it names a pack and version ("space-shooter" v2, role and
// description) instead of catalog asset IDs, which the service cannot resolve.
func TestGameMakerTextModeReplay(t *testing.T) {
	fixture := loadTextModeReplayFixture(t)
	wants := map[string]struct {
		accepted    bool
		designState string
		sequence    []string
	}{
		"MiniMax M3":               {false, "error", []string{"game_maker_project/list_files:ok", "game_maker_project/set_design:error"}},
		"hf:zai-org/GLM-5.3-Flash": {true, "ok", []string{"game_maker_project/get_plan:ok", "game_maker_asset/search_assets:ok", "game_maker_project/set_design:ok"}},
	}
	for _, recorded := range fixture.Models {
		want, known := wants[recorded.Model]
		if !known {
			t.Fatalf("no expectation for recorded model %q", recorded.Model)
		}
		label := strings.NewReplacer("/", "_", ":", "_", " ", "_").Replace(recorded.Model)
		t.Run(label+"/first set_design reaches the service", func(t *testing.T) {
			result := replayGameMakerPlanning(t, recorded.Dimension, []string{recorded.Replies["set_design"]})
			requireTextModeWithoutDenial(t, result)
			if got := result.toolResults(); len(got) != 1 || got[0] != "game_maker_project/set_design:"+want.designState || result.accepted != want.accepted {
				t.Fatalf("tool results %v accepted=%v, want set_design:%s accepted=%v", got, result.accepted, want.designState, want.accepted)
			}
			if !want.accepted && (!strings.Contains(result.shown, `"errors":[`) || !strings.Contains(result.shown, "assets")) {
				t.Fatalf("a rejected design must come back with its validation issues:\n%s", result.shown)
			}
		})
		t.Run(label+"/recorded sequence is never scope-denied", func(t *testing.T) {
			var replies []string
			for _, id := range recorded.Sequence {
				replies = append(replies, recorded.Replies[id])
			}
			result := replayGameMakerPlanning(t, recorded.Dimension, replies)
			requireTextModeWithoutDenial(t, result)
			if got := result.toolResults(); !slices.Equal(got, want.sequence) || result.accepted != want.accepted {
				t.Fatalf("tool results %v accepted=%v, want %v accepted=%v", got, result.accepted, want.sequence, want.accepted)
			}
		})
	}
}
