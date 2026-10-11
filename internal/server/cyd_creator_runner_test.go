package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/cydcreator"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

func TestCYDCreatorPromptProfiles(t *testing.T) {
	for _, phase := range []string{cydcreator.PhasePlanning, cydcreator.PhaseLayout, cydcreator.PhaseCoding} {
		profile, err := cydCreatorPromptProfile(phase)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, tool := range profile.Tools() {
			names = append(names, tool.Function.Name)
		}
		if !slices.Equal(names, cydcreator.PhaseTools(phase)) {
			t.Errorf("%s tools = %v", phase, names)
		}
		for _, rule := range []string{"never send passwords", "LVGL only on the loop task", "omit job_id", "never by firmware hashes"} {
			if !strings.Contains(profile.SystemPrompt(), rule) {
				t.Errorf("%s profile lacks %q", phase, rule)
			}
		}
	}
	coding, _ := cydCreatorPromptProfile(cydcreator.PhaseCoding)
	repair, _ := cydCreatorPromptProfile(cydcreator.PhaseRepairing)
	if coding.Revision() != repair.Revision() {
		t.Fatal("repair must share the coding profile")
	}
	if _, err := cydCreatorPromptProfile(cydcreator.PhaseBuilding); err == nil {
		t.Fatal("building has no agent profile")
	}
}

func TestCYDCreatorToolCallBudget(t *testing.T) {
	for limit, want := range map[int]int{0: 40, 20: 40, 32: 40, 40: 50, 41: 52, 100: 125} {
		if got := gameMakerToolCallLimit(limit); got != want {
			t.Errorf("limit(%d) = %d, want %d", limit, got, want)
		}
	}
}

// cydScriptedProvider is an OpenAI-compatible streaming endpoint that answers
// each phase with scripted tool calls, chosen by the tools the request offers.
type cydScriptedProvider struct {
	mu       sync.Mutex
	requests []openai.ChatCompletionRequest
	coding   int
	failNext bool
}

func cydToolNames(req openai.ChatCompletionRequest) []string {
	var names []string
	for _, tool := range req.Tools {
		names = append(names, tool.Function.Name)
	}
	return names
}

func writeToolCall(w http.ResponseWriter, id, name string, args any) {
	data, _ := json.Marshal(args)
	delta := map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": id, "type": "function",
		"function": map[string]any{"name": name, "arguments": string(data)}}}}
	chunk, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": "tool_calls"}}})
	fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", chunk)
}

func writeText(w http.ResponseWriter, text string) {
	chunk, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0,
		"delta": map[string]any{"role": "assistant", "content": text}, "finish_reason": "stop"}}})
	fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", chunk)
}

func (p *cydScriptedProvider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req openai.ChatCompletionRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	p.mu.Lock()
	p.requests = append(p.requests, req)
	names := cydToolNames(req)
	last := req.Messages[len(req.Messages)-1]
	fail := p.failNext
	p.failNext = false
	step := 0
	if slices.Contains(names, cydcreator.ToolWriteFile) {
		p.coding++
		step = p.coding
	}
	p.mu.Unlock()
	if fail {
		http.Error(w, `{"error":{"message":"scripted outage"}}`, http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	switch {
	case slices.Contains(names, cydcreator.ToolSubmitPlan):
		writeToolCall(w, "plan-1", cydcreator.ToolSubmitPlan, map[string]any{"plan": map[string]any{"summary": "A clock",
			"screens": []string{"home"}, "wifi": "optional"}})
	case slices.Contains(names, cydcreator.ToolUpdateLayout) && !slices.Contains(names, cydcreator.ToolWriteFile):
		if last.Role == openai.ChatMessageRoleTool {
			writeText(w, "Layout ready.")
			return
		}
		writeToolCall(w, "layout-1", cydcreator.ToolUpdateLayout, map[string]any{"ops": []any{map[string]any{"op": "add_widget",
			"screen": "home", "widget": map[string]any{"type": "label", "name": "lbl_time", "x": 10, "y": 10, "props": `{"text":"12:00"}`}}}})
	case step == 1:
		writeToolCall(w, "code-1", cydcreator.ToolWriteFile, map[string]any{"path": "src/app/app_main.cpp",
			"content": "#include \"ui.h\"\nvoid app_setup() {}\nvoid app_loop() { lv_label_set_text(ui_lbl_time, \"12:01\"); }\n"})
	case step == 2:
		writeToolCall(w, "code-2", cydcreator.ToolBuild, map[string]any{})
	default:
		writeToolCall(w, fmt.Sprintf("code-%d", step), cydcreator.ToolFinish, map[string]any{"summary": "Shows the time."})
	}
}

func newCYDRunnerFixture(t *testing.T, provider http.Handler) (*cydTestFixture, *httptest.Server) {
	t.Helper()
	f := newCYDTestServer(t)
	upstream := httptest.NewServer(provider)
	t.Cleanup(upstream.Close)
	cfg := f.s.Cfg
	cfg.LLM.Model, cfg.LLM.ProviderType = "test-cyd-model", "openai"
	cfg.Agent.ContextWindow, cfg.CircuitBreaker.LLMTimeoutSeconds = 65536, 10
	cfg.Directories.ToolsDir, cfg.Directories.WorkspaceDir = filepath.Join(t.TempDir(), "tools"), t.TempDir()
	clientConfig := openai.DefaultConfig("local-test")
	clientConfig.BaseURL = upstream.URL
	f.s.LLMClient = llm.WrapOpenAIClient(openai.NewClientWithConfig(clientConfig))
	f.s.HistoryManager = memory.NewEphemeralHistoryManager()
	f.s.Registry = tools.NewProcessRegistry(f.s.Logger)
	stm, err := memory.NewSQLiteMemory(":memory:", f.s.Logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	f.s.ShortTermMem = stm
	previous := cydcreator.DefaultService()
	cydcreator.SetDefaultService(f.svc)
	t.Cleanup(func() { cydcreator.SetDefaultService(previous) })
	f.svc.SetRunner(&cydCreatorAgentRunner{server: f.s, service: f.svc})
	return f, upstream
}

func TestCYDCreatorRunnerBuildsAnAppThroughTheAgentLoop(t *testing.T) {
	provider := &cydScriptedProvider{}
	f, _ := newCYDRunnerFixture(t, provider)
	p := f.createProject(t, "Clock")
	ctx := context.Background()
	job, err := f.svc.StartJob(ctx, p.ID, cydcreator.StartJobInput{Mode: cydcreator.JobModeNew, Prompt: "Build a clock"})
	if err != nil {
		t.Fatal(err)
	}
	waiting := waitCYDServerJob(t, f, job.ID, cydcreator.JobAwaitingConfirmation, cydcreator.JobFailed)
	if waiting.Status != cydcreator.JobAwaitingConfirmation || waiting.Plan == nil || waiting.Plan.Summary != "A clock" {
		t.Fatalf("after planning: %+v", waiting)
	}
	provider.mu.Lock()
	planningRequests := len(provider.requests)
	planning := provider.requests[0]
	provider.mu.Unlock()
	if planningRequests != 1 {
		t.Fatalf("planning used %d LLM requests; the accepted plan must end the round", planningRequests)
	}
	if planning.Messages[0].Role != openai.ChatMessageRoleSystem || !strings.Contains(planning.Messages[0].Content, "You are CYD Creator") ||
		!slices.Equal(cydToolNames(planning), cydcreator.PhaseTools(cydcreator.PhasePlanning)) {
		t.Fatalf("planning request: system=%q tools=%v", planning.Messages[0].Content[:min(60, len(planning.Messages[0].Content))], cydToolNames(planning))
	}
	user := planning.Messages[len(planning.Messages)-1].Content
	if !strings.HasPrefix(user, "Build a clock") || !strings.Contains(user, "<external_data>") || !strings.Contains(user, `"job_id":"`+job.ID+`"`) {
		t.Fatalf("planning user message = %q", user)
	}

	if _, err := f.svc.ConfirmJob(ctx, job.ID, ""); err != nil {
		t.Fatal(err)
	}
	done := waitCYDServerJob(t, f, job.ID, cydcreator.JobSucceeded, cydcreator.JobFailed)
	if done.Status != cydcreator.JobSucceeded {
		t.Fatalf("job = %+v", done)
	}
	layout, _, _, _ := f.svc.GetLayout(ctx, p.ID)
	if _, _, ok := layout.FindWidget("lbl_time"); !ok {
		t.Fatal("the agent's layout change was not published")
	}
	src, _, err := f.svc.ReadFile(ctx, p.ID, "src/app/app_main.cpp")
	if err != nil || !strings.Contains(string(src), "ui_lbl_time") {
		t.Fatalf("published source = %q, %v", src, err)
	}
	messages, _ := f.svc.ListMessages(ctx, p.ID)
	if last := messages[len(messages)-1]; last.Role != cydcreator.MessageRoleAgent || last.Text != "Shows the time." {
		t.Fatalf("last message = %+v", last)
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	// No request carries a host path (the test's temporary directory) or the
	// provider key, neither in prompts nor in tool results.
	hostDir := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(f.dbPath))))
	for _, req := range provider.requests {
		data, _ := json.Marshal(req.Messages)
		if strings.Contains(string(data), hostDir) || strings.Contains(string(data), "local-test") {
			t.Fatalf("a request leaks a host path or the key: %s", data)
		}
	}
	var codingRequests int
	for _, req := range provider.requests {
		if slices.Contains(cydToolNames(req), cydcreator.ToolWriteFile) {
			codingRequests++
			if !slices.Equal(cydToolNames(req), cydcreator.PhaseTools(cydcreator.PhaseCoding)) {
				t.Fatalf("coding tools = %v", cydToolNames(req))
			}
		}
		for _, msg := range req.Messages {
			if msg.Role == openai.ChatMessageRoleTool && strings.Contains(msg.Content, "&lt;") {
				t.Fatalf("source tool result was escaped: %s", msg.Content)
			}
		}
	}
	if codingRequests != 3 {
		t.Fatalf("coding requests = %d (write, build, finish; finish ends the round)", codingRequests)
	}
	// The conversation stays private: nothing reaches the normal chat history
	// and the job's transient agent session is purged after every phase.
	if got := f.s.HistoryManager.GetAll(); len(got) != 0 {
		t.Fatalf("the normal chat history got %d messages", len(got))
	}
	for _, session := range []string{"default", "cyd-creator-" + job.ID} {
		if msgs, err := f.s.ShortTermMem.GetSessionMessagesForBridge(session); err != nil || len(msgs) != 0 {
			t.Fatalf("session %s kept %d messages (%v)", session, len(msgs), err)
		}
	}
}

// cyd_creator.model names a provider: the job talks to that provider's
// endpoint and model only; an unknown provider fails the job with CYD_DISABLED.
func TestCYDCreatorRunnerUsesTheConfiguredProvider(t *testing.T) {
	mainProvider, routed := &cydScriptedProvider{}, &cydScriptedProvider{}
	f, _ := newCYDRunnerFixture(t, mainProvider)
	// A provider with its own base URL is probed for model limits (GET
	// .../models): answer those with 404, chat requests with the script.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			http.NotFound(w, r)
			return
		}
		routed.ServeHTTP(w, r)
	}))
	t.Cleanup(upstream.Close)
	f.s.Cfg.Providers = append(f.s.Cfg.Providers, config.ProviderEntry{ID: "cyd-route", Name: "CYD route", Type: "openai",
		BaseURL: upstream.URL, APIKey: "local-test", Model: "cyd-route-model"})
	limits := f.svc.Limits()
	limits.Model = "cyd-route"
	f.svc.UpdateLimits(limits)
	p := f.createProject(t, "Routed")
	ctx := context.Background()
	job, err := f.svc.StartJob(ctx, p.ID, cydcreator.StartJobInput{Mode: cydcreator.JobModeNew, Prompt: "Build a clock"})
	if err != nil {
		t.Fatal(err)
	}
	if got := waitCYDServerJob(t, f, job.ID, cydcreator.JobAwaitingConfirmation, cydcreator.JobFailed); got.Status != cydcreator.JobAwaitingConfirmation {
		t.Fatalf("after planning: %+v", got)
	}
	routed.mu.Lock()
	routedRequests := len(routed.requests)
	model := ""
	if routedRequests > 0 {
		model = routed.requests[0].Model
	}
	routed.mu.Unlock()
	mainProvider.mu.Lock()
	mainRequests := len(mainProvider.requests)
	mainProvider.mu.Unlock()
	if routedRequests != 1 || model != "cyd-route-model" || mainRequests != 0 {
		t.Fatalf("routed requests = %d (model %q), main requests = %d", routedRequests, model, mainRequests)
	}
	if err := f.svc.CancelJob(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	waitCYDServerJob(t, f, job.ID, cydcreator.JobCancelled)

	limits.Model = "missing"
	f.svc.UpdateLimits(limits)
	job, err = f.svc.StartJob(ctx, p.ID, cydcreator.StartJobInput{Mode: cydcreator.JobModeNew, Prompt: "Build a clock"})
	if err != nil {
		t.Fatal(err)
	}
	failed := waitCYDServerJob(t, f, job.ID, cydcreator.JobFailed, cydcreator.JobAwaitingConfirmation)
	if failed.Status != cydcreator.JobFailed || failed.ErrorCode != cydcreator.CodeDisabled {
		t.Fatalf("job with an unknown provider = %+v", failed)
	}
}

// Cancelling a job ends the LLM request that is in flight.
func TestCYDCreatorRunnerCancelReachesTheLLMRequest(t *testing.T) {
	started, ended, stop := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var startOnce, endOnce sync.Once
	provider := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reading the body to EOF lets net/http notice the client hanging up.
		_, _ = io.Copy(io.Discard, r.Body)
		startOnce.Do(func() { close(started) })
		select {
		case <-r.Context().Done():
			endOnce.Do(func() { close(ended) })
		case <-stop:
		}
	})
	f, _ := newCYDRunnerFixture(t, provider)
	t.Cleanup(func() { close(stop) }) // runs before the upstream closes
	p := f.createProject(t, "Cancel")
	ctx := context.Background()
	job, err := f.svc.StartJob(ctx, p.ID, cydcreator.StartJobInput{Mode: cydcreator.JobModeChange, Prompt: "Show the time"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("the agent never called the model")
	}
	if err := f.svc.CancelJob(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		t.Fatal("the LLM request outlived the cancelled job")
	}
	if got := waitCYDServerJob(t, f, job.ID, cydcreator.JobCancelled, cydcreator.JobFailed); got.Status != cydcreator.JobCancelled {
		t.Fatalf("job = %+v", got)
	}
}

func TestCYDCreatorRunnerResumesTheSavedConversation(t *testing.T) {
	provider := &cydScriptedProvider{}
	// Coding request 1 writes a file, request 2 hits an outage: the job fails
	// and keeps a draft plus the private conversation.
	f, _ := newCYDRunnerFixture(t, &cydFailAfter{inner: provider, after: 1})
	p := f.createProject(t, "Resume")
	ctx := context.Background()
	job, err := f.svc.StartJob(ctx, p.ID, cydcreator.StartJobInput{Mode: cydcreator.JobModeChange, Prompt: "Show the time"})
	if err != nil {
		t.Fatal(err)
	}
	failed := waitCYDServerJob(t, f, job.ID, cydcreator.JobFailed, cydcreator.JobSucceeded)
	summary, _ := f.svc.GetProject(ctx, p.ID)
	if failed.Status != cydcreator.JobFailed || !summary.HasDraft {
		t.Fatalf("job = %+v, project = %+v", failed, summary)
	}
	resumed, err := f.svc.StartJob(ctx, p.ID, cydcreator.StartJobInput{Mode: cydcreator.JobModeChange, Resume: true})
	if err != nil {
		t.Fatal(err)
	}
	done := waitCYDServerJob(t, f, resumed.ID, cydcreator.JobSucceeded, cydcreator.JobFailed)
	if done.Status != cydcreator.JobSucceeded {
		t.Fatalf("resumed job = %+v", done)
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	first := provider.requests[len(provider.requests)-1]
	for _, req := range provider.requests {
		if strings.Contains(req.Messages[len(req.Messages)-1].Content, `"resumed":true`) {
			first = req
			break
		}
	}
	var sawEarlierCall bool
	for _, msg := range first.Messages {
		for _, call := range msg.ToolCalls {
			sawEarlierCall = sawEarlierCall || call.ID == "code-1"
		}
	}
	if !sawEarlierCall {
		t.Fatalf("the resumed request lacks the saved conversation: %+v", first.Messages)
	}
}

// cydFailAfter answers with an outage after `after` coding requests, once.
type cydFailAfter struct {
	inner *cydScriptedProvider
	after int
	mu    sync.Mutex
	seen  int
	done  bool
}

func (c *cydFailAfter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	c.seen++
	fail := !c.done && c.seen == c.after+1
	if fail {
		c.done = true
	}
	c.mu.Unlock()
	if fail {
		c.inner.mu.Lock()
		c.inner.failNext = true
		c.inner.mu.Unlock()
	}
	c.inner.ServeHTTP(w, r)
}

func waitCYDServerJob(t *testing.T, f *cydTestFixture, jobID string, statuses ...string) cydcreator.Job {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		j, err := f.svc.GetJob(context.Background(), jobID)
		for _, s := range statuses {
			if err == nil && j.Status == s {
				time.Sleep(20 * time.Millisecond)
				return j
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("job = %+v, %v", j, err)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
