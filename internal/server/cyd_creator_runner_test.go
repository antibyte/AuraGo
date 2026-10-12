package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/cydcreator"
	"aurago/internal/i18n"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/tools"
	"aurago/ui"

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
		for _, rule := range []string{"never send passwords", "LVGL only on the loop task", "omit job_id", "never by firmware hashes",
			"topic overview first", "board and orientation are fixed", "only from the user's uploads", "at least one button"} {
			if !strings.Contains(profile.SystemPrompt(), rule) {
				t.Errorf("%s profile lacks %q", phase, rule)
			}
		}
		// A profile names only tools its phase offers.
		for _, name := range cydToolNamePattern.FindAllString(profile.SystemPrompt(), -1) {
			if !slices.Contains(names, name) {
				t.Errorf("%s profile names %s, which the phase does not offer", phase, name)
			}
		}
	}
	coding, _ := cydCreatorPromptProfile(cydcreator.PhaseCoding)
	for _, rule := range []string{"start_line/end_line", `op set_settings "libraries"`} {
		if !strings.Contains(coding.SystemPrompt(), rule) {
			t.Errorf("coding profile lacks %q", rule)
		}
	}
	repair, _ := cydCreatorPromptProfile(cydcreator.PhaseRepairing)
	if coding.Revision() != repair.Revision() {
		t.Fatal("repair must share the coding profile")
	}
	if _, err := cydCreatorPromptProfile(cydcreator.PhaseBuilding); err == nil {
		t.Fatal("building has no agent profile")
	}
}

var cydToolNamePattern = regexp.MustCompile(`cyd_creator_[a-z_]+`)

// The private checkpoint keeps the newest messages within its byte cap and
// never starts with a tool result whose call was dropped.
func TestCYDCreatorConversationTrim(t *testing.T) {
	var messages []openai.ChatCompletionMessage
	for i := 0; i < 60; i++ {
		id := fmt.Sprintf("call-%d", i)
		messages = append(messages,
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{{ID: id, Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{Name: cydcreator.ToolReadFile, Arguments: "{}"}}}},
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, ToolCallID: id, Content: strings.Repeat("x", 64<<10)})
	}
	if got := cydCreatorTrimConversation(messages[:4], 1<<20); len(got) != 4 {
		t.Fatalf("a small archive was trimmed to %d messages", len(got))
	}
	trimmed := cydCreatorTrimConversation(messages, 1<<20)
	data, _ := json.Marshal(trimmed)
	if len(data) > 1<<20 || len(trimmed) < 2 || trimmed[0].Role != openai.ChatMessageRoleAssistant ||
		trimmed[len(trimmed)-1].ToolCallID != "call-59" {
		t.Fatalf("trimmed archive: %d bytes, %d messages, first role %q", len(data), len(trimmed), trimmed[0].Role)
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Messages) == 0 {
		// Model-limit probes (GET .../models of a provider with its own base
		// URL) carry no chat request.
		http.NotFound(w, r)
		return
	}
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
	if !strings.HasPrefix(user, "Build a clock") || !strings.Contains(user, "<external_data>") || !strings.Contains(user, `"job_id":"`+job.ID+`"`) ||
		!strings.Contains(user, `"seconds_remaining":`) {
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
	waitCYD(t, "the agent's summary", func() bool {
		messages, _ := f.svc.ListMessages(ctx, p.ID)
		return len(messages) > 0 && messages[len(messages)-1].Role == cydcreator.MessageRoleAgent && messages[len(messages)-1].Text == "Shows the time."
	})
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
	for _, session := range []string{"default", cydCreatorAgentSessionPrefix + job.ID} {
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
	// A provider with its own base URL is also probed for model limits (GET
	// .../models); the scripted provider answers those with 404.
	upstream := httptest.NewServer(routed)
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
	job = startCYDJob(t, f, p.ID, cydcreator.StartJobInput{Mode: cydcreator.JobModeNew, Prompt: "Build a clock"})
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
	if failed.Status != cydcreator.JobFailed {
		t.Fatalf("job = %+v", failed)
	}
	waitCYD(t, "the draft", func() bool {
		summary, err := f.svc.GetProject(ctx, p.ID)
		return err == nil && summary.HasDraft
	})
	resumed := startCYDJob(t, f, p.ID, cydcreator.StartJobInput{Mode: cydcreator.JobModeChange, Resume: true})
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
				return j
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("job = %+v, %v", j, err)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitCYD polls cond until it holds or 10 s pass. A job's final status is
// stored before its chat message, its draft and the release of the agent slot.
func waitCYD(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// startCYDJob starts a job, retrying while the previous job still holds the
// agent slot or the project lock (CYD_BUSY).
func startCYDJob(t *testing.T, f *cydTestFixture, projectID string, in cydcreator.StartJobInput) cydcreator.Job {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		job, err := f.svc.StartJob(context.Background(), projectID, in)
		if err == nil {
			return job
		}
		var typed *cydcreator.Error
		if !errors.As(err, &typed) || typed.Code != cydcreator.CodeBusy || time.Now().After(deadline) {
			t.Fatalf("start job: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A provider failure reaches the job chat only as a generic reason: the
// provider's address, account path and response body stay in the server log.
func TestCYDCreatorRunnerKeepsProviderErrorsOutOfTheChat(t *testing.T) {
	i18n.Load(ui.Content, slog.Default())
	provider := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"Incorrect API key provided: loc***est at http://10.1.2.3:8080/v1/accounts/acct-4242/chat","type":"invalid_request_error"}}`)
	})
	f, _ := newCYDRunnerFixture(t, provider)
	p := f.createProject(t, "Leak")
	job := startCYDJob(t, f, p.ID, cydcreator.StartJobInput{Mode: cydcreator.JobModeChange, Prompt: "Show the time"})
	failed := waitCYDServerJob(t, f, job.ID, cydcreator.JobFailed, cydcreator.JobSucceeded)
	want := "The agent stopped: " + i18n.T("en", "backend.handler_llm_auth_error")
	if failed.Status != cydcreator.JobFailed || failed.ErrorMessage != want {
		t.Fatalf("job = %+v", failed)
	}
	waitCYD(t, "the failure in the chat", func() bool {
		messages, _ := f.svc.ListMessages(context.Background(), p.ID)
		return slices.ContainsFunc(messages, func(m cydcreator.Message) bool { return strings.Contains(m.Text, want) })
	})
	messages, _ := f.svc.ListMessages(context.Background(), p.ID)
	data, _ := json.Marshal(messages)
	for _, leak := range []string{"10.1.2.3", "acct-4242", "loc***est", "Incorrect API key"} {
		if strings.Contains(string(data), leak) || strings.Contains(failed.ErrorMessage, leak) {
			t.Fatalf("%q reached the job chat: %s", leak, data)
		}
	}
}

// A damaged checkpoint is logged and skipped: the resumed job still runs.
func TestCYDCreatorRunnerSkipsADamagedCheckpoint(t *testing.T) {
	provider := &cydScriptedProvider{}
	f, _ := newCYDRunnerFixture(t, &cydFailAfter{inner: provider, after: 1})
	p := f.createProject(t, "Damaged")
	job := startCYDJob(t, f, p.ID, cydcreator.StartJobInput{Mode: cydcreator.JobModeChange, Prompt: "Show the time"})
	if got := waitCYDServerJob(t, f, job.ID, cydcreator.JobFailed, cydcreator.JobSucceeded); got.Status != cydcreator.JobFailed {
		t.Fatalf("job = %+v", got)
	}
	waitCYD(t, "the draft", func() bool {
		summary, err := f.svc.GetProject(context.Background(), p.ID)
		return err == nil && summary.HasDraft
	})
	db, err := sql.Open("sqlite", f.dbPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Valid JSON of the wrong shape: the service stores it, the runner cannot decode it.
	res, err := db.Exec(`UPDATE cc_agent_context SET data=? WHERE job_id=?`, `{"messages":"damaged"}`, job.ID)
	if n, _ := res.RowsAffected(); err != nil || n != 1 {
		t.Fatalf("damage the checkpoint: %d rows, %v", n, err)
	}
	resumed := startCYDJob(t, f, p.ID, cydcreator.StartJobInput{Mode: cydcreator.JobModeChange, Resume: true})
	if done := waitCYDServerJob(t, f, resumed.ID, cydcreator.JobSucceeded, cydcreator.JobFailed); done.Status != cydcreator.JobSucceeded {
		t.Fatalf("resumed job = %+v", done)
	}
}

// Startup purges agent sessions a crash left behind, but never the session of
// a job that is still running, and no other session.
func TestCYDCreatorStartupSweepPurgesLeftoverAgentSessions(t *testing.T) {
	started, stop := make(chan struct{}), make(chan struct{})
	var once sync.Once
	provider := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		once.Do(func() { close(started) })
		select {
		case <-r.Context().Done():
		case <-stop:
		}
	})
	f, _ := newCYDRunnerFixture(t, provider)
	t.Cleanup(func() { close(stop) }) // runs before the upstream closes
	p := f.createProject(t, "Sweep")
	job := startCYDJob(t, f, p.ID, cydcreator.StartJobInput{Mode: cydcreator.JobModeChange, Prompt: "Show the time"})
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("the agent never called the model")
	}
	running, leftover := cydCreatorAgentSessionPrefix+job.ID, cydCreatorAgentSessionPrefix+"ccj_gone"
	for _, id := range []string{running, leftover, "default"} {
		if _, err := f.s.ShortTermMem.InsertMessage(id, openai.ChatMessageRoleUser, "private", false, false); err != nil {
			t.Fatal(err)
		}
	}
	f.s.sweepCYDCreatorAgentSessions(f.svc)
	if ids, err := f.s.ShortTermMem.TransientSessionIDs(cydCreatorAgentSessionPrefix); err != nil || !slices.Equal(ids, []string{running}) {
		t.Fatalf("sessions after the sweep = %v, %v", ids, err)
	}
	if msgs, err := f.s.ShortTermMem.GetSessionMessagesForBridge("default"); err != nil || len(msgs) != 1 {
		t.Fatalf("the default session = %d messages, %v", len(msgs), err)
	}
	if err := f.svc.CancelJob(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	waitCYDServerJob(t, f, job.ID, cydcreator.JobCancelled, cydcreator.JobFailed)
}

// A call to a tool the phase does not offer, built-in or from another CYD
// Creator phase, is refused without running; the job goes on.
func TestCYDCreatorRunnerRefusesToolsOutsideThePhase(t *testing.T) {
	scripted := &cydScriptedProvider{}
	var mu sync.Mutex
	planning := 0
	results := map[string]string{}
	provider := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req openai.ChatCompletionRequest
		if json.Unmarshal(body, &req) == nil && slices.Contains(cydToolNames(req), cydcreator.ToolSubmitPlan) {
			mu.Lock()
			for _, msg := range req.Messages {
				if msg.Role == openai.ChatMessageRoleTool {
					results[msg.ToolCallID] = msg.Content
				}
			}
			planning++
			n := planning
			mu.Unlock()
			switch n {
			case 1:
				w.Header().Set("Content-Type", "text/event-stream")
				writeToolCall(w, "rogue-shell", "execute_shell", map[string]any{"command": "echo outside"})
				return
			case 2:
				w.Header().Set("Content-Type", "text/event-stream")
				writeToolCall(w, "rogue-write", cydcreator.ToolWriteFile, map[string]any{"path": "src/app/app_main.cpp", "content": "// outside\n"})
				return
			}
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		scripted.ServeHTTP(w, r)
	})
	f, _ := newCYDRunnerFixture(t, provider)
	p := f.createProject(t, "Scope")
	job := startCYDJob(t, f, p.ID, cydcreator.StartJobInput{Mode: cydcreator.JobModeNew, Prompt: "Build a clock"})
	if got := waitCYDServerJob(t, f, job.ID, cydcreator.JobAwaitingConfirmation, cydcreator.JobFailed); got.Status != cydcreator.JobAwaitingConfirmation {
		t.Fatalf("after planning: %+v", got)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, id := range []string{"rogue-shell", "rogue-write"} {
		if !strings.Contains(results[id], "tool_scope_denied") {
			t.Fatalf("result of %s = %q", id, results[id])
		}
	}
	if err := f.svc.CancelJob(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
}
