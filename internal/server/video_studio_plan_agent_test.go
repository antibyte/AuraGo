package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/agent"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/tools"
	"aurago/internal/videostudio/storyplan"

	"github.com/sashabaranov/go-openai"
)

// A native route needs a model no name rule or catalog knows, so the config flag decides. glm-4 is forced to text
// mode by its name.
const (
	vspScriptNativeModel = "test-plan-model"
	vspScriptTextModel   = "glm-4"
)

// vspTurn is one model reply: prose, or a tool call (native, or the raw JSON of the text-mode guide). block holds
// the reply until the context ends.
type vspTurn struct {
	prose string
	tool  string
	args  map[string]any
	block bool
}

// vspScriptClient answers each request with the next turn of the phase its system prompt names. A phase with no
// turn left answers "Done.".
type vspScriptClient struct {
	textMode bool
	onBlock  func()

	mu       sync.Mutex
	turns    map[string][]vspTurn
	requests []openai.ChatCompletionRequest
	calls    int
}

var vspScriptHeadings = []struct{ heading, phase string }{
	{"## Outline", agent.VideoStudioPlanPhaseOutline},
	{"## Details", agent.VideoStudioPlanPhaseDetails},
	{"## Check the plan", agent.VideoStudioPlanPhaseCheck},
	{"## Revise one scene", agent.VideoStudioPlanPhaseRevise},
}

func (c *vspScriptClient) reply(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionMessage, openai.FinishReason, error) {
	if err := ctx.Err(); err != nil {
		return openai.ChatCompletionMessage{}, "", err
	}
	c.mu.Lock()
	req.Messages = slices.Clone(req.Messages)
	c.requests = append(c.requests, req)
	phase := ""
	if len(req.Messages) > 0 {
		for _, h := range vspScriptHeadings {
			if strings.Contains(req.Messages[0].Content, h.heading) {
				phase = h.phase
			}
		}
	}
	turn := vspTurn{prose: "Done."}
	if queue := c.turns[phase]; len(queue) > 0 {
		turn, c.turns[phase] = queue[0], queue[1:]
	}
	c.calls++
	id := fmt.Sprintf("call-%d", c.calls)
	c.mu.Unlock()

	if turn.block {
		if c.onBlock != nil {
			c.onBlock()
		}
		<-ctx.Done()
		return openai.ChatCompletionMessage{}, "", ctx.Err()
	}
	message := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant}
	if turn.tool == "" {
		message.Content = turn.prose
		return message, openai.FinishReasonStop, nil
	}
	args := turn.args
	if args == nil {
		args = map[string]any{}
	}
	if c.textMode {
		call := map[string]any{"action": turn.tool}
		for key, value := range args {
			call[key] = value
		}
		data, err := json.Marshal(call)
		if err != nil {
			return openai.ChatCompletionMessage{}, "", err
		}
		message.Content = string(data)
		return message, openai.FinishReasonStop, nil
	}
	data, err := json.Marshal(args)
	if err != nil {
		return openai.ChatCompletionMessage{}, "", err
	}
	index := 0
	message.ToolCalls = []openai.ToolCall{{Index: &index, ID: id, Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: turn.tool, Arguments: string(data)}}}
	return message, openai.FinishReasonToolCalls, nil
}

func (c *vspScriptClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	message, finish, err := c.reply(ctx, req)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	return openai.ChatCompletionResponse{Model: req.Model, Choices: []openai.ChatCompletionChoice{{Message: message, FinishReason: finish}}}, nil
}

// CreateChatCompletionStream yields one chunk per scripted turn.
func (c *vspScriptClient) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	message, finish, err := c.reply(ctx, req)
	if err != nil {
		return nil, err
	}
	return &vspScriptStream{chunks: []openai.ChatCompletionStreamResponse{{
		Model: req.Model,
		Choices: []openai.ChatCompletionStreamChoice{{
			Index:        0,
			Delta:        openai.ChatCompletionStreamChoiceDelta{Role: message.Role, Content: message.Content, ToolCalls: message.ToolCalls},
			FinishReason: finish,
		}},
	}}}, nil
}

func (c *vspScriptClient) seen() []openai.ChatCompletionRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.requests)
}

// saw reports whether any message other than a system prompt holds the text.
func (c *vspScriptClient) saw(text string) bool {
	for _, req := range c.seen() {
		for _, message := range req.Messages {
			if message.Role != openai.ChatMessageRoleSystem && strings.Contains(message.Content, text) {
				return true
			}
		}
	}
	return false
}

type vspScriptStream struct {
	chunks []openai.ChatCompletionStreamResponse
}

func (s *vspScriptStream) Recv() (openai.ChatCompletionStreamResponse, error) {
	if len(s.chunks) == 0 {
		return openai.ChatCompletionStreamResponse{}, io.EOF
	}
	chunk := s.chunks[0]
	s.chunks = s.chunks[1:]
	return chunk, nil
}

func (*vspScriptStream) Close() error { return nil }

// vspAgentEnv is vspFixture with the scripted client as the main LLM route.
func vspAgentEnv(t *testing.T, model string, client *vspScriptClient) (*Server, string, string) {
	t.Helper()
	s, token, projectID := vspFixture(t)
	s.Cfg.LLM.Model, s.Cfg.LLM.ProviderType, s.Cfg.LLM.UseNativeFunctions = model, "openai", true
	s.Cfg.Agent.ContextWindow, s.Cfg.CircuitBreaker.LLMTimeoutSeconds = 65536, 30
	s.Cfg.Directories.ToolsDir = filepath.Join(t.TempDir(), "tools")
	s.Cfg.VideoStudio.Plan.MaxDailyJobs, s.Cfg.VideoStudio.Plan.JobTimeoutSeconds = 20, 900
	s.LLMClient = client
	s.HistoryManager = memory.NewEphemeralHistoryManager()
	s.Registry = tools.NewProcessRegistry(s.Logger)
	stm, err := memory.NewSQLiteMemory(":memory:", s.Logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	s.ShortTermMem = stm
	return s, token, projectID
}

func vspWait(t *testing.T, m *videoStudioPlanAgents, projectID string) videoStudioPlanAgentStatus {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if status, ok := m.Status(projectID); ok && status.State != videoStudioPlanAgentRunning {
			return status
		}
		time.Sleep(5 * time.Millisecond)
	}
	status, _ := m.Status(projectID)
	t.Fatalf("the plan agent did not end: %+v", status)
	return videoStudioPlanAgentStatus{}
}

func vspCreateRequest(projectID string) videoStudioPlanAgentRequest {
	return videoStudioPlanAgentRequest{ProjectID: projectID, Mode: agent.VideoStudioPlanModeCreate, Idea: vspIdea, TargetSeconds: 12, Language: "de"}
}

// vspCreateTurns writes an outline whose second scene uses more than it generates, fills the prompts, and fixes
// that one blocking issue in the check phase.
func vspCreateTurns() map[string][]vspTurn {
	return map[string][]vspTurn{
		agent.VideoStudioPlanPhaseOutline: {{tool: agent.VideoStudioPlanToolWrite, args: map[string]any{"plan": map[string]any{"scenes": []any{
			map[string]any{"id": "s1", "title": "Hafen im Nebel", "generate_seconds": 6, "use_seconds": 6, "transition_in": map[string]any{"kind": "cut"}},
			map[string]any{"id": "s2", "title": "Ausfahrt", "generate_seconds": 6, "use_seconds": 6.5, "transition_in": map[string]any{"kind": "cut"}},
		}}}}},
		agent.VideoStudioPlanPhaseDetails: {
			{tool: agent.VideoStudioPlanToolPatchScene, args: map[string]any{"scene_id": "s1", "scene": map[string]any{"video_prompt": vspPromptOne, "keyframe": map[string]any{"source": "none"}}}},
			{tool: agent.VideoStudioPlanToolPatchScene, args: map[string]any{"scene_id": "s2", "scene": map[string]any{"video_prompt": vspPromptTwo}}},
		},
		agent.VideoStudioPlanPhaseCheck: {
			{tool: agent.VideoStudioPlanToolCheck},
			{tool: agent.VideoStudioPlanToolPatchScene, args: map[string]any{"scene_id": "s2", "scene": map[string]any{"use_seconds": 6}}},
			{tool: agent.VideoStudioPlanToolFinish},
		},
	}
}

func TestVideoStudioPlanAgentCreatesAPlan(t *testing.T) {
	for _, test := range []struct {
		name, model string
		text        bool
	}{
		{"native", vspScriptNativeModel, false},
		{"text", vspScriptTextModel, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &vspScriptClient{textMode: test.text, turns: vspCreateTurns()}
			s, _, projectID := vspAgentEnv(t, test.model, client)
			m := s.videoStudioPlanAgents()
			t.Cleanup(func() { m.Stop(projectID) })
			started, err := m.Start(vspCreateRequest(projectID))
			if err != nil || started.State != videoStudioPlanAgentRunning || started.Mode != agent.VideoStudioPlanModeCreate {
				t.Fatalf("start = %+v, %v", started, err)
			}
			status := vspWait(t, m, projectID)
			if status.State != videoStudioPlanAgentDone || status.Error != "" || status.FinishedAt.IsZero() || m.Running(projectID) {
				t.Fatalf("status = %+v", status)
			}
			plan := vspReadPlan(t, s, projectID)
			if plan.Status != storyplan.StatusReview || len(plan.Scenes) != 2 || plan.Scenes[0].VideoPrompt != vspPromptOne || plan.Scenes[1].VideoPrompt != vspPromptTwo || plan.Scenes[1].UseSeconds != 6 {
				t.Fatalf("plan = %+v", plan)
			}
			if client.saw("tool_scope_denied") || client.saw("tool_not_in_phase") {
				t.Fatal("a scripted call was refused")
			}
			if !client.saw("<external_data>") || !client.saw(vspIdea) {
				t.Fatal("the user message lacks the data packet")
			}
			requests := client.seen()
			for _, heading := range []string{"## Outline", "## Details", "## Check the plan"} {
				if !slices.ContainsFunc(requests, func(req openai.ChatCompletionRequest) bool { return strings.Contains(req.Messages[0].Content, heading) }) {
					t.Fatalf("no request ran the phase %s", heading)
				}
			}
			for i, req := range requests {
				guide := strings.Contains(req.Messages[0].Content, "## Calling tools")
				if test.text && (len(req.Tools) != 0 || !guide) {
					t.Fatalf("text-mode request %d: %d tools, guide %v", i, len(req.Tools), guide)
				}
				if !test.text && (len(req.Tools) == 0 || guide) {
					t.Fatalf("native request %d: %d tools, guide %v", i, len(req.Tools), guide)
				}
			}
		})
	}
}

func TestVideoStudioPlanAgentRevisesOneScene(t *testing.T) {
	night := "Tracking shot from behind. The boat glides out of the harbour under a starry sky. Moonlight on calm water."
	client := &vspScriptClient{turns: map[string][]vspTurn{
		agent.VideoStudioPlanPhaseRevise: {
			{tool: agent.VideoStudioPlanToolPatchScene, args: map[string]any{"scene_id": "s1", "scene": map[string]any{"title": "Anders"}}},
			{tool: agent.VideoStudioPlanToolPatchScene, args: map[string]any{"scene_id": "s2", "scene": map[string]any{"video_prompt": night}}},
			{tool: agent.VideoStudioPlanToolFinish},
		},
	}}
	s, _, projectID := vspAgentEnv(t, vspScriptNativeModel, client)
	plan := vspNewPlan(t, s, 12)
	plan.Status = storyplan.StatusReview
	plan.Scenes = []storyplan.Scene{vspScene("s1", "Hafen im Nebel", vspPromptOne), vspScene("s2", "Ausfahrt", vspPromptTwo)}
	vspSeed(t, s, projectID, plan)
	m := s.videoStudioPlanAgents()
	t.Cleanup(func() { m.Stop(projectID) })
	if _, err := m.Start(videoStudioPlanAgentRequest{ProjectID: projectID, Mode: agent.VideoStudioPlanModeRevise, SceneID: "s2", Note: "Make it night."}); err != nil {
		t.Fatal(err)
	}
	status := vspWait(t, m, projectID)
	if status.State != videoStudioPlanAgentDone || status.Mode != agent.VideoStudioPlanModeRevise {
		t.Fatalf("status = %+v", status)
	}
	saved := vspReadPlan(t, s, projectID)
	if saved.Status != storyplan.StatusReview || saved.Scenes[0].Title != "Hafen im Nebel" || saved.Scenes[1].VideoPrompt != night {
		t.Fatalf("plan = %+v", saved)
	}
	if !client.saw("scene_not_allowed") || !client.saw("Make it night.") {
		t.Fatal("the revise job did not keep the model on its scene, or the note did not reach the model")
	}
}

func TestVideoStudioPlanAgentStopBusyAndDailyLimit(t *testing.T) {
	reached := make(chan struct{})
	var once sync.Once
	client := &vspScriptClient{
		turns:   map[string][]vspTurn{agent.VideoStudioPlanPhaseOutline: {{block: true}}},
		onBlock: func() { once.Do(func() { close(reached) }) },
	}
	s, token, first := vspAgentEnv(t, vspScriptNativeModel, client)
	second, _ := videoStudioAPICreate(t, s, token)
	vspSeed(t, s, second.ID, vspNewPlan(t, s, 12))
	s.Cfg.VideoStudio.Plan.MaxDailyJobs = 1
	m := s.videoStudioPlanAgents()
	t.Cleanup(func() { m.Stop(first); m.Stop(second.ID) })

	if _, err := m.Start(vspCreateRequest(first)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-reached:
	case <-time.After(60 * time.Second):
		t.Fatal("the model never got the outline request")
	}
	if !m.Running(first) {
		t.Fatal("the job is not running")
	}
	if _, err := m.Start(vspCreateRequest(first)); !errors.Is(err, errVideoStudioPlanAgentBusy) {
		t.Fatalf("second start for the project = %v", err)
	}
	if _, err := m.Start(vspCreateRequest(second.ID)); !errors.Is(err, errVideoStudioPlanAgentDailyLimit) {
		t.Fatalf("start over the daily limit = %v", err)
	}
	if !m.Stop(first) {
		t.Fatal("the running job was not stopped")
	}
	if status := vspWait(t, m, first); status.State != videoStudioPlanAgentStopped || status.Error != "stopped" {
		t.Fatalf("stopped status = %+v", status)
	}
	if plan := vspReadPlan(t, s, first); plan.Status != storyplan.StatusReview {
		t.Fatalf("a stopped job left the plan %q", plan.Status)
	}
	if m.Stop(first) {
		t.Fatal("a finished job was stopped again")
	}

	// The refused start did not count: one more slot lets the second project start.
	s.Cfg.VideoStudio.Plan.MaxDailyJobs = 2
	if _, err := m.Start(vspCreateRequest(second.ID)); err != nil {
		t.Fatal(err)
	}
	// The script has no outline turn left, so the model only answers in prose and the outline never ends.
	if status := vspWait(t, m, second.ID); status.State != videoStudioPlanAgentFailed || status.Error != "tool_limit" {
		t.Fatalf("prose-only status = %+v", status)
	}
}
