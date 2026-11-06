package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/llm"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

// storymakerReviseStream yields one chunk and ends.
type storymakerReviseStream struct {
	text string
	sent bool
}

func (s *storymakerReviseStream) Recv() (openai.ChatCompletionStreamResponse, error) {
	if s.sent {
		return openai.ChatCompletionStreamResponse{}, io.EOF
	}
	s.sent = true
	return openai.ChatCompletionStreamResponse{Choices: []openai.ChatCompletionStreamChoice{{
		Delta:        openai.ChatCompletionStreamChoiceDelta{Content: s.text},
		FinishReason: openai.FinishReasonStop,
	}}}, nil
}

func (s *storymakerReviseStream) Close() error { return nil }

// storymakerReviseClient plays its replies in order and then repeats the last one.
type storymakerReviseClient struct {
	mu      sync.Mutex
	replies []string
	calls   int
}

func (c *storymakerReviseClient) next() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	reply := c.replies[min(c.calls, len(c.replies)-1)]
	c.calls++
	return reply
}

func (c *storymakerReviseClient) CreateChatCompletion(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{
		Message:      openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: c.next()},
		FinishReason: openai.FinishReasonStop,
	}}}, nil
}

func (c *storymakerReviseClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	return &storymakerReviseStream{text: c.next()}, nil
}

func storymakerReviseScenes(t *testing.T, rt *storymakerRuntime, id string) map[string]storymaker.Scene {
	t.Helper()
	data, _, err := rt.projects.LoadStory(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	story, _, err := storymaker.DecodeStory(data, storymaker.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	storymaker.ApplyDefaults(&story)
	scenes := make(map[string]storymaker.Scene, len(story.Scenes))
	for _, scene := range story.Scenes {
		scenes[scene.ID] = scene
	}
	return scenes
}

func TestStorymakerReviseKeepsOtherScenes(t *testing.T) {
	s, _, token := storymakerAPIFixture(t)
	// The glm name selects text mode, so a reply is one raw JSON object.
	s.Cfg.LLM.Model, s.Cfg.LLM.ProviderType = "glm-4", "openai"
	s.Cfg.Agent.ContextWindow = 65536
	// No browser: the compile check starts nothing.
	s.storymakerBrowsers = render.NewBrowsersWithProbe(t.TempDir(),
		func() (string, bool) { return "", false },
		func(context.Context, string) (string, error) { return "", errors.New("no browser in this test") })
	// A voice that happens to be ready must not reach a real provider.
	s.storymakerFreeDiskOverride = func(string) (int64, error) { return 1 << 40, nil }
	s.storymakerSynthOverride = func(context.Context, tools.TTSConfig, string) ([]byte, string, error) {
		return nil, "", errors.New("no speech in this test")
	}
	const changed = "Die zweite Szene ist jetzt neu."
	s.LLMClient = &storymakerReviseClient{replies: []string{
		`{"action":"storymaker_story","operation":"patch_scene","scene_id":"s2","data":{"narration":"` + changed + `"}}`,
		// A first prose reply gets the recovery nudge, so the accepted answer is the second one.
		"Szene 2 ist neu.",
		"Szene 2 ist neu.",
		"Pruefung abgeschlossen.",
	}}

	p := createStorymakerProject(t, s, token)
	scene := func(id, narration string) string {
		return `{"id":"` + id + `","environment":"env.office","narration":"` + narration + `",` +
			`"actors":[{"cast":"anna","at":"left","pose":"present"}],` +
			`"beats":[{"do":"gesture","who":"anna","gesture":"point"}]}`
	}
	story := `{"schema":"storymaker/1","title":"Revise","format":{"aspect":"16:9","fps":30},` +
		`"cast":[{"id":"anna","name":"Anna","character":"char.office-woman-1"}],"scenes":[` +
		scene("s1", "Die erste Szene bleibt.") + `,` + scene("s2", "Die zweite Szene ist alt.") + `,` + scene("s3", "Die dritte Szene bleibt.") + `]}`
	saved := storymakerRequest(s, token, http.MethodPut, "/projects/"+p.Project.ID+"/story", strings.NewReader(story), map[string]string{"If-Match": p.ETag})
	if saved.Code != http.StatusOK {
		t.Fatalf("save = %d %s", saved.Code, saved.Body.String())
	}
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	before := storymakerReviseScenes(t, rt, p.Project.ID)

	job, _, err := rt.agent.Enqueue(render.EnqueueRequest{
		ProjectID: p.Project.ID,
		Kind:      render.KindAgent,
		Coalesce:  true,
		Payload: storymakerAgentPayload{
			Kind: storymaker.AgentKindRevise, ProjectID: p.Project.ID,
			Instruction: "Mach Szene 2 neu.", Language: "de", LengthSeconds: 30,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var done render.Job
	for deadline := time.Now().Add(2 * time.Minute); ; time.Sleep(20 * time.Millisecond) {
		current, found := rt.agent.Get(job.ID)
		if found && current.Terminal() {
			done = current
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the agent job did not end: %+v", current)
		}
	}
	if done.Status != render.StatusSucceeded {
		t.Fatalf("job = %s, error %+v", done.Status, done.Error)
	}
	if done.Agent == nil || done.Agent.Kind != storymaker.AgentKindRevise {
		t.Fatalf("agent block = %+v", done.Agent)
	}
	if !reflect.DeepEqual(done.Agent.ChangedScenes, []string{"s2"}) ||
		!reflect.DeepEqual(done.Agent.UnchangedScenes, []string{"s1", "s3"}) || len(done.Agent.RemovedScenes) != 0 {
		t.Fatalf("scene lists = changed %v, unchanged %v, removed %v", done.Agent.ChangedScenes, done.Agent.UnchangedScenes, done.Agent.RemovedScenes)
	}
	if done.Agent.Outcome == storymaker.OutcomeReadyUnverified && done.Agent.Summary != "Szene 2 ist neu." {
		t.Fatalf("summary = %q, want the revise reply", done.Agent.Summary)
	}

	after := storymakerReviseScenes(t, rt, p.Project.ID)
	if after["s2"].Narration != changed {
		t.Fatalf("s2 narration = %q", after["s2"].Narration)
	}
	for _, id := range []string{"s1", "s3"} {
		if !reflect.DeepEqual(before[id], after[id]) {
			t.Fatalf("scene %s changed:\nbefore %+v\nafter  %+v", id, before[id], after[id])
		}
	}
}
