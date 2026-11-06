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

	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

// A native route needs a model no name rule or catalog knows, so the config flag decides. glm-4 is
// forced to text mode by its name, even with that flag on.
const (
	smScriptNativeModel = "test-story-model"
	smScriptTextModel   = "glm-4"
)

// The job asks for 15 s: 34.5 target words, 2 suggested scenes, and at most 39.675 words before the
// outline is refused. These lines have 9 and 10 words, under the scene budget of 19.
const (
	smScriptFirstLine   = "Eine Firewall kontrolliert jedes Paket am Eingang deines Netzes."
	smScriptFirstMarked = "Eine Firewall kontrolliert jedes {1}Paket am Eingang deines Netzes."
	smScriptSecondLine  = "Fremde Pakete kommen nicht hinein, und dein Netz bleibt sicher."
)

// smScriptTurn is one model reply. prose is plain text. tool and args are a tool call, sent as a native
// call or, on a text-mode route, as the raw JSON object of the text-mode guide. block holds the reply
// until the context ends.
type smScriptTurn struct {
	prose string
	tool  string
	args  map[string]any
	block bool
}

// smScriptClient answers each request with the next turn of the phase its system prompt names. A phase
// with no turn left answers "Done.".
type smScriptClient struct {
	textMode bool
	onBlock  func()

	mu       sync.Mutex
	turns    map[string][]smScriptTurn
	requests []openai.ChatCompletionRequest
	calls    int
}

var smScriptPhases = []struct{ heading, phase string }{
	{"## Outline", storymaker.PhaseOutline},
	{"## Cast", storymaker.PhaseCast},
	{"## Fill one scene", storymaker.PhaseFill},
	{"## Check and fix", storymaker.PhaseCheck},
	{"## Revise", storymaker.PhaseRevise},
}

func smScriptPhase(req openai.ChatCompletionRequest) string {
	if len(req.Messages) == 0 {
		return ""
	}
	for _, p := range smScriptPhases {
		if strings.Contains(req.Messages[0].Content, p.heading) {
			return p.phase
		}
	}
	return ""
}

func (c *smScriptClient) reply(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionMessage, openai.FinishReason, error) {
	if err := ctx.Err(); err != nil {
		return openai.ChatCompletionMessage{}, "", err
	}
	c.mu.Lock()
	req.Messages = slices.Clone(req.Messages)
	c.requests = append(c.requests, req)
	phase := smScriptPhase(req)
	turn := smScriptTurn{prose: "Done."}
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
	if c.textMode {
		call := map[string]any{"action": turn.tool}
		for key, value := range turn.args {
			call[key] = value
		}
		data, err := json.Marshal(call)
		if err != nil {
			return openai.ChatCompletionMessage{}, "", err
		}
		message.Content = string(data)
		return message, openai.FinishReasonStop, nil
	}
	data, err := json.Marshal(turn.args)
	if err != nil {
		return openai.ChatCompletionMessage{}, "", err
	}
	index := 0
	message.ToolCalls = []openai.ToolCall{{Index: &index, ID: id, Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: turn.tool, Arguments: string(data)}}}
	return message, openai.FinishReasonToolCalls, nil
}

func (c *smScriptClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	message, finish, err := c.reply(ctx, req)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	return openai.ChatCompletionResponse{Model: req.Model, Choices: []openai.ChatCompletionChoice{{Message: message, FinishReason: finish}}}, nil
}

// CreateChatCompletionStream yields one chunk per scripted turn.
func (c *smScriptClient) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	message, finish, err := c.reply(ctx, req)
	if err != nil {
		return nil, err
	}
	return &smScriptStream{chunks: []openai.ChatCompletionStreamResponse{{
		Model: req.Model,
		Choices: []openai.ChatCompletionStreamChoice{{
			Index:        0,
			Delta:        openai.ChatCompletionStreamChoiceDelta{Role: message.Role, Content: message.Content, ToolCalls: message.ToolCalls},
			FinishReason: finish,
		}},
	}}}, nil
}

// seen returns the requests the model received.
func (c *smScriptClient) seen() []openai.ChatCompletionRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.requests)
}

// saw reports whether any message other than a system prompt holds the text.
func (c *smScriptClient) saw(text string) bool {
	for _, req := range c.seen() {
		for _, message := range req.Messages {
			if message.Role != openai.ChatMessageRoleSystem && strings.Contains(message.Content, text) {
				return true
			}
		}
	}
	return false
}

type smScriptStream struct {
	chunks []openai.ChatCompletionStreamResponse
}

func (s *smScriptStream) Recv() (openai.ChatCompletionStreamResponse, error) {
	if len(s.chunks) == 0 {
		return openai.ChatCompletionStreamResponse{}, io.EOF
	}
	chunk := s.chunks[0]
	s.chunks = s.chunks[1:]
	return chunk, nil
}

func (*smScriptStream) Close() error { return nil }

func smScriptStoryCall(operation, sceneID string, data map[string]any) smScriptTurn {
	args := map[string]any{"operation": operation}
	if sceneID != "" {
		args["scene_id"] = sceneID
	}
	if data != nil {
		args["data"] = data
	}
	return smScriptTurn{tool: storymaker.ToolStory, args: args}
}

func smScriptCatalogCall(kind, query string) smScriptTurn {
	return smScriptTurn{tool: storymaker.ToolCatalog, args: map[string]any{"operation": "search", "kind": kind, "query": query}}
}

func smScriptOutlineData() map[string]any {
	return map[string]any{
		"template":       "concept_explainer",
		"target_seconds": 15,
		"scenes": []any{
			map[string]any{"id": "s1", "narration": smScriptFirstLine},
			map[string]any{"id": "s2", "narration": smScriptSecondLine},
		},
	}
}

func smScriptMetaData() map[string]any {
	return map[string]any{
		"title": "Was ist eine Firewall?",
		"cast":  []any{map[string]any{"id": "anna", "name": "Anna", "character": "char.office-woman-1"}},
	}
}

func smScriptSceneOne() map[string]any {
	return map[string]any{
		"narration":   smScriptFirstMarked,
		"environment": "env.office",
		"actors":      []any{map[string]any{"cast": "anna", "at": "left", "pose": "present"}},
		"beats": []any{
			map[string]any{"do": "show", "item": "prop.lock", "at": "right", "entry": "pop", "cue": 1},
			map[string]any{"do": "gesture", "who": "anna", "gesture": "point"},
		},
		"transition": "wipe",
	}
}

func smScriptSceneTwo() map[string]any {
	return map[string]any{
		"environment": "env.office",
		"actors":      []any{map[string]any{"cast": "anna", "at": "center", "pose": "think"}},
		"beats": []any{
			map[string]any{"do": "show", "item": "prop.shield", "at": "right", "entry": "fade"},
			map[string]any{"do": "gesture", "who": "anna", "gesture": "present"},
		},
		"transition": "fade",
	}
}

func smScriptHappyTurns() map[string][]smScriptTurn {
	return map[string][]smScriptTurn{
		storymaker.PhaseOutline: {smScriptStoryCall("outline", "", smScriptOutlineData())},
		storymaker.PhaseCast: {
			smScriptCatalogCall("character", "office"),
			smScriptStoryCall("set_meta", "", smScriptMetaData()),
		},
		storymaker.PhaseFill: {
			smScriptCatalogCall("prop", "lock"),
			smScriptStoryCall("put_scene", "s1", smScriptSceneOne()),
			smScriptStoryCall("put_scene", "s2", smScriptSceneTwo()),
		},
	}
}

// newSMScriptEnv builds a server whose main route is the scripted client. It sets the config before the
// first config snapshot is taken, and it has a ready local voice with a stub synth.
func newSMScriptEnv(t *testing.T, model string, client *smScriptClient) *narrateJobEnv {
	t.Helper()
	s, _, token := storymakerAPIFixture(t)
	s.Cfg.LLM.Model, s.Cfg.LLM.ProviderType, s.Cfg.LLM.UseNativeFunctions = model, "openai", true
	s.Cfg.Agent.ContextWindow, s.Cfg.CircuitBreaker.LLMTimeoutSeconds = 65536, 30
	s.Cfg.Directories.ToolsDir = filepath.Join(t.TempDir(), "tools")
	s.Cfg.TTS.Provider = "sanotts"
	s.Cfg.Storymaker.Agent.Enabled = true
	// The fixture's fake Chrome reports ready, and a compile check would then try to start it.
	s.storymakerBrowsers = render.NewBrowsersWithProbe(t.TempDir(),
		func() (string, bool) { return "", false },
		func(context.Context, string) (string, error) { return "", errors.New("no browser in this test") })
	s.storymakerFreeDiskOverride = func(string) (int64, error) { return 1 << 40, nil }
	s.storymakerSynthOverride = func(context.Context, tools.TTSConfig, string) ([]byte, string, error) {
		return narrateJobWAV(), "wav", nil
	}
	s.LLMClient = client
	s.HistoryManager = memory.NewEphemeralHistoryManager()
	s.Registry = tools.NewProcessRegistry(s.Logger)
	stm, err := memory.NewSQLiteMemory(":memory:", s.Logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	s.ShortTermMem = stm
	p := createStorymakerProject(t, s, token)
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	return &narrateJobEnv{s: s, rt: rt, token: token, id: p.Project.ID, etag: p.ETag}
}

func smScriptEnqueue(t *testing.T, m *render.Manager, env *narrateJobEnv) render.Job {
	t.Helper()
	job, _, err := m.Enqueue(render.EnqueueRequest{
		ProjectID: env.id,
		Kind:      render.KindAgent,
		Coalesce:  true,
		Payload: storymakerAgentPayload{
			Kind:          storymaker.AgentKindCreate,
			ProjectID:     env.id,
			Topic:         "Wie funktioniert eine Firewall?",
			Language:      "de",
			LengthSeconds: 15,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return job
}

func smScriptWait(t *testing.T, m *render.Manager, id string) render.Job {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if job, ok := m.Get(id); ok && job.Terminal() {
			return job
		}
		time.Sleep(5 * time.Millisecond)
	}
	last, _ := m.Get(id)
	t.Fatalf("the agent job did not end: %+v", last)
	return render.Job{}
}

// smScriptStory reads the stored story with its defaults applied and requires that it validates.
func smScriptStory(t *testing.T, env *narrateJobEnv) storymaker.Story {
	t.Helper()
	data, _, err := env.rt.projects.LoadStory(context.Background(), env.id)
	if err != nil {
		t.Fatal(err)
	}
	st, _, err := storymaker.DecodeStory(data, storymaker.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	storymaker.ApplyDefaults(&st)
	if issues := storymaker.Validate(&st, env.rt.catalog, storymaker.DefaultLimits()); storymaker.HasErrors(issues) {
		t.Fatalf("the stored story has errors: %+v", issues)
	}
	return st
}

func smScriptFilled(sc storymaker.Scene) bool { return len(sc.Actors) > 0 && len(sc.Beats) > 0 }

func smScriptRequireFilled(t *testing.T, st storymaker.Story, ids ...string) {
	t.Helper()
	for _, id := range ids {
		found := false
		for _, sc := range st.Scenes {
			if sc.ID == id {
				found = true
				if !smScriptFilled(sc) {
					t.Fatalf("scene %s is not filled: %+v", id, sc)
				}
			}
		}
		if !found {
			t.Fatalf("scene %s is missing from %+v", id, st.Scenes)
		}
	}
}

func smScriptRequireReady(t *testing.T, job render.Job) {
	t.Helper()
	if job.Status != render.StatusSucceeded || job.Agent == nil || job.Agent.Outcome != storymaker.OutcomeReadyUnverified {
		t.Fatalf("job = %+v, agent = %+v, want succeeded with %s", job, job.Agent, storymaker.OutcomeReadyUnverified)
	}
}

func smScriptRunHappyJob(t *testing.T, model string, textMode bool) {
	t.Helper()
	client := &smScriptClient{textMode: textMode, turns: smScriptHappyTurns()}
	env := newSMScriptEnv(t, model, client)
	job := smScriptWait(t, env.rt.agent, smScriptEnqueue(t, env.rt.agent, env).ID)
	smScriptRequireReady(t, job)
	if job.Agent.TextMode != textMode {
		t.Fatalf("agent.text_mode = %v, want %v", job.Agent.TextMode, textMode)
	}

	st := smScriptStory(t, env)
	smScriptRequireFilled(t, st, "s1", "s2")
	if st.Voice == nil {
		t.Fatal("the job wrote no voice")
	}
	// The two narrations: the script has no say beat.
	if blocks := storymaker.Blocks(&st); len(blocks) != 2 {
		t.Fatalf("spoken blocks = %+v, want 2", blocks)
	}
	if missing := env.missing(t); len(missing) != 0 {
		t.Fatalf("narration audio is missing: %+v", missing)
	}

	if client.saw("tool_scope_denied") {
		t.Fatal("a scripted call was refused by the tool scope")
	}
	requests := client.seen()
	if len(requests) == 0 {
		t.Fatal("the model got no request")
	}
	for i, req := range requests {
		guide := strings.Contains(req.Messages[0].Content, "## Calling tools")
		if textMode && (len(req.Tools) != 0 || !guide) {
			t.Fatalf("text-mode request %d: %d tools, guide %v", i, len(req.Tools), guide)
		}
		if !textMode && (len(req.Tools) == 0 || guide) {
			t.Fatalf("native request %d: %d tools, guide %v", i, len(req.Tools), guide)
		}
	}
}

func TestStorymakerAgentScriptedJobNative(t *testing.T) {
	smScriptRunHappyJob(t, smScriptNativeModel, false)
}

func TestStorymakerAgentScriptedJobTextMode(t *testing.T) {
	smScriptRunHappyJob(t, smScriptTextModel, true)
}

func TestStorymakerAgentScriptedWeakModel(t *testing.T) {
	// Flat fields: the scene sits beside operation and scene_id instead of inside data.
	invalid := smScriptSceneOne()
	invalid["operation"], invalid["scene_id"] = "put_scene", "s1"
	invalid["actors"] = []any{map[string]any{"cast": "anna", "at": "left", "pose": "moonwalk"}}
	flat := smScriptSceneTwo()
	flat["operation"], flat["scene_id"] = "put_scene", "s2"

	client := &smScriptClient{turns: map[string][]smScriptTurn{
		storymaker.PhaseOutline: {{tool: "outline", args: map[string]any{"data": smScriptOutlineData()}}},
		storymaker.PhaseCast: {
			{prose: "Ich suche jetzt die Besetzung aus."},
			smScriptStoryCall("set_meta", "", smScriptMetaData()),
		},
		storymaker.PhaseFill: {
			{tool: "execute_shell", args: map[string]any{"command": "whoami"}},
			{tool: storymaker.ToolStory, args: invalid},
			smScriptStoryCall("patch_scene", "s1", map[string]any{
				"actors": []any{map[string]any{"cast": "anna", "at": "left", "pose": "present"}},
			}),
			{tool: storymaker.ToolStory, args: flat},
		},
	}}
	env := newSMScriptEnv(t, smScriptNativeModel, client)
	job := smScriptWait(t, env.rt.agent, smScriptEnqueue(t, env.rt.agent, env).ID)
	smScriptRequireReady(t, job)

	st := smScriptStory(t, env)
	smScriptRequireFilled(t, st, "s1", "s2")
	if got := st.Scenes[0].Actors[0].Pose; got != "present" {
		t.Fatalf("the patch did not reach the saved scene: pose = %q", got)
	}
	if !client.saw(`"status":"draft"`) {
		t.Fatal("the invalid put_scene did not answer with a draft")
	}
	if !client.saw("tool_scope_denied") {
		t.Fatal("execute_shell was not refused by the tool scope")
	}
	if !client.saw("Your last reply had no Storymaker tool call.") {
		t.Fatal("the prose reply got no recovery nudge")
	}
}

// smScriptDeadline is a job context whose deadline passes when pass is called.
type smScriptDeadline struct {
	context.Context
	done chan struct{}
	once sync.Once
}

func (d *smScriptDeadline) Done() <-chan struct{} { return d.done }

func (d *smScriptDeadline) Err() error {
	select {
	case <-d.done:
	default:
		return nil
	}
	if err := d.Context.Err(); err != nil {
		return err
	}
	return context.DeadlineExceeded
}

func (d *smScriptDeadline) pass() { d.once.Do(func() { close(d.done) }) }

// smScriptBlockedTurns saves scene s1 and then stops the model on scene s2.
func smScriptBlockedTurns() map[string][]smScriptTurn {
	turns := smScriptHappyTurns()
	turns[storymaker.PhaseFill] = []smScriptTurn{
		smScriptStoryCall("put_scene", "s1", smScriptSceneOne()),
		{block: true},
	}
	return turns
}

// smScriptRequireKept requires a valid story with s1 saved and s2 still in its outline form.
func smScriptRequireKept(t *testing.T, env *narrateJobEnv) {
	t.Helper()
	st := smScriptStory(t, env)
	smScriptRequireFilled(t, st, "s1")
	if len(st.Scenes) != 2 || st.Scenes[1].ID != "s2" || st.Scenes[1].Narration != smScriptSecondLine || smScriptFilled(st.Scenes[1]) {
		t.Fatalf("scenes after the stop = %+v", st.Scenes)
	}
}

func TestStorymakerAgentCancelAndTimeoutKeepValidScenes(t *testing.T) {
	t.Run("cancel", func(t *testing.T) {
		reached := make(chan struct{})
		client := &smScriptClient{turns: smScriptBlockedTurns(), onBlock: func() { close(reached) }}
		env := newSMScriptEnv(t, smScriptNativeModel, client)
		started := smScriptEnqueue(t, env.rt.agent, env)
		select {
		case <-reached:
		case <-time.After(60 * time.Second):
			t.Fatal("the model never reached scene s2")
		}
		if _, ok := env.rt.agent.Cancel(started.ID); !ok {
			t.Fatal("the job was not found for its cancel")
		}
		job := smScriptWait(t, env.rt.agent, started.ID)
		if job.Status != render.StatusCancelled {
			t.Fatalf("job = %+v, want cancelled", job)
		}
		smScriptRequireKept(t, env)
	})

	t.Run("timeout", func(t *testing.T) {
		deadline := &smScriptDeadline{done: make(chan struct{})}
		client := &smScriptClient{turns: smScriptBlockedTurns(), onBlock: deadline.pass}
		env := newSMScriptEnv(t, smScriptNativeModel, client)
		m, err := render.NewManager(render.ManagerOptions{
			Dir:     t.TempDir(),
			Kinds:   []string{render.KindAgent},
			Workers: 1,
			Run: func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
				deadline.Context = ctx
				stop := context.AfterFunc(ctx, deadline.pass)
				defer stop()
				return env.s.storymakerAgentRun(deadline, env.rt, job, payload, h)
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(m.Close)
		job := smScriptWait(t, m, smScriptEnqueue(t, m, env).ID)
		if job.Status != render.StatusSucceeded || job.Agent == nil || job.Agent.Outcome != storymaker.OutcomeIncomplete {
			t.Fatalf("job = %+v, agent = %+v, want succeeded with %s", job, job.Agent, storymaker.OutcomeIncomplete)
		}
		if queued := env.rt.narrate.List(env.id); len(queued) != 0 {
			t.Fatalf("a timed-out job queued %d narrate jobs", len(queued))
		}
		smScriptRequireKept(t, env)
	})
}

// A narration the agent job joined (Coalesce returned a job that was already queued or running) is not
// the job's to cancel. One it started itself is.
func TestStorymakerAgentLeavesAJoinedNarrationRunning(t *testing.T) {
	for _, joined := range []bool{true, false} {
		t.Run(fmt.Sprintf("joined=%v", joined), func(t *testing.T) {
			started := make(chan struct{})
			release := make(chan struct{})
			m, err := render.NewManager(render.ManagerOptions{
				Dir:     t.TempDir(),
				Kinds:   []string{render.KindNarrate},
				Workers: 1,
				Run: func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
					close(started)
					select {
					case <-ctx.Done():
						return render.Output{}, ctx.Err()
					case <-release:
						return render.Output{}, nil
					}
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(m.Close)
			t.Cleanup(func() { close(release) })
			job, _, err := m.Enqueue(render.EnqueueRequest{ProjectID: "p1", Kind: render.KindNarrate, Coalesce: true, Payload: storymakerNarratePayload{ProjectID: "p1"}})
			if err != nil {
				t.Fatal(err)
			}
			select {
			case <-started:
			case <-time.After(30 * time.Second):
				t.Fatal("the narrate job never started")
			}
			j := &storymakerAgentJob{rt: &storymakerRuntime{narrate: m}, job: render.Job{ProjectID: "p1"}}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if got := j.waitNarration(ctx, job.ID, time.Minute, joined); got != storymakerCheckNarrationPending {
				t.Fatalf("waitNarration = %q, want %q", got, storymakerCheckNarrationPending)
			}
			if joined {
				time.Sleep(100 * time.Millisecond)
			} else {
				deadline := time.Now().Add(30 * time.Second)
				for time.Now().Before(deadline) {
					if cur, _ := m.Get(job.ID); cur.Terminal() {
						break
					}
					time.Sleep(5 * time.Millisecond)
				}
			}
			cur, _ := m.Get(job.ID)
			if joined && cur.Terminal() {
				t.Fatalf("a joined narration was stopped: %+v", cur)
			}
			if !joined && cur.Status != render.StatusCancelled {
				t.Fatalf("a narration the job started was not cancelled: %+v", cur)
			}
		})
	}
}
