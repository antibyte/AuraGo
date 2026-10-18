package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"aurago/internal/codegraph"
	"aurago/internal/memory"
	openai "github.com/sashabaranov/go-openai"
)

func TestLearningConflictChecksAcceptedOperationForms(t *testing.T) {
	for _, tc := range []ToolCall{
		{Action: "filesystem", Operation: "write", FilePath: "main.txt", Content: "changed"},
		{Action: "filesystem", Operation: " WRITE_FILE ", FilePath: "main.txt", Content: "changed"},
		{Action: "filesystem", Params: map[string]any{"operation": "write_file", "destination": "main.txt", "content": "changed"}},
		{Action: "yaml_editor", Params: map[string]any{"operation": " SET ", "file_path": "main.txt", "json_path": "content", "set_value": "changed"}},
		{Action: "json_editor", Params: map[string]any{"operation": " SET ", "file_path": "main.txt", "json_path": "content", "set_value": "changed"}},
		{Action: "toml_editor", Params: map[string]any{"operation": " SET ", "file_path": "main.txt", "toml_path": "content", "set_value": "changed"}},
		{Action: "filesystem", Params: map[string]any{"operation": " DELETE_BATCH ", "items": []any{map[string]any{"file_path": "main.txt"}}}},
	} {
		t.Run(tc.Action+tc.Operation, func(t *testing.T) {
			_, dc, _ := correctionAgentFixture(t)
			dc.Cfg.Learnings.Enabled = true
			dc.LearningScope = memory.LearningScope{Project: "test"}
			storeAgentLearning(t, dc, "Keep main.txt unchanged", dc.LearningScope, &memory.LearningCondition{Target: "main.txt", Field: "content", Value: "original"})
			storeAgentLearning(t, dc, "Only read main.txt", dc.LearningScope, &memory.LearningCondition{Target: "main.txt", Field: "operation", Value: "read_file"})
			if blocked, _ := checkLearningAction(t.Context(), tc, dc); !strings.Contains(blocked, "review_required") {
				t.Fatalf("accepted operation bypassed conflict check: %s %s %v: %s", tc.Action, tc.Operation, tc.Params, blocked)
			}
		})
	}
}

func TestLearningChecksRespectMemoryWriteRestrictions(t *testing.T) {
	for _, mode := range []string{"readonly", "co-agent"} {
		t.Run(mode, func(t *testing.T) {
			_, dc, _ := correctionAgentFixture(t)
			dc.Cfg.Learnings.Enabled = true
			dc.LearningScope = memory.LearningScope{Project: "a"}
			storeAgentLearning(t, dc, "Keep main.txt", dc.LearningScope, &memory.LearningCondition{Target: "main.txt", Field: "content", Value: "old"})
			dc.Cfg.Tools.Memory.ReadOnly = mode == "readonly"
			dc.IsCoAgent = mode == "co-agent"
			var audit bytes.Buffer
			dc.Logger = slog.New(slog.NewTextHandler(&audit, nil))
			call := ToolCall{Action: "filesystem", Operation: "write", FilePath: "main.txt", Content: "new"}
			if blocked, _ := checkLearningAction(t.Context(), call, dc); blocked == "" {
				t.Fatal("conflict missing")
			}
			call.Params = map[string]any{"learning_reason": "Requested temporary compatibility test"}
			if blocked, _ := checkLearningAction(t.Context(), call, dc); blocked != "" {
				t.Fatal(blocked)
			}
			metrics, err := dc.ShortTermMem.LearningMetrics(t.Context())
			if err != nil || metrics["conflicts"] != 0 || metrics["overrides"] != 0 {
				t.Fatal("memory write restriction bypassed", metrics, err)
			}
			if !strings.Contains(audit.String(), "Requested temporary compatibility test") {
				t.Fatal("deviation was not audited")
			}
		})
	}
}

func TestLearningProjectRevocationCoversRecallCorrectionsAndAnalysis(t *testing.T) {
	s, dc, _ := correctionAgentFixture(t)
	dc.Cfg.Learnings.Enabled = true
	s.runCfg.LearningScope = memory.LearningScope{Project: "a"}
	s.runCfg.CodeProject = codegraph.Project{ID: "a", Root: t.TempDir()}
	allowed := true
	s.runCfg.LearningProjectValid = func(p codegraph.Project) bool { return allowed && p.ID == "a" }
	dc = s.makeDispatchContext(s.currentLogger)
	l := storeAgentLearning(t, dc, "Keep TypeScript", dc.LearningScope, nil)
	if learningPromptContext(t.Context(), s.runCfg, "TypeScript", "gpt-4o") == "" {
		t.Fatal("authorized recall missing")
	}
	preview := correctionTestCall(l.ID)
	preview.CorrectionChanges[0].Reference.Store = "learning"
	plan, status := decodeCorrectionTestResult(t, dispatchMemoryCorrection(t.Context(), preview, dc))
	if status != "success" || plan == nil {
		t.Fatal(status, plan)
	}
	allowed = false
	if got := learningPromptContext(t.Context(), s.runCfg, "TypeScript", "gpt-4o"); got != "" {
		t.Fatal("revoked automatic recall", got)
	}
	const previous = "previous project observation"
	messages := []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: "Immutable profile"}, {Role: openai.ChatMessageRoleUser, Content: "Current task"}}
	withLearning := insertLearningObservation(messages, previous)
	refreshed := replaceLearningObservation(withLearning, previous, "")
	if len(refreshed) != 2 || refreshed[0].Content != messages[0].Content || refreshed[1].Content != messages[1].Content || len(withLearning) != 3 {
		t.Fatal("prepared observation survived revocation or mutated history")
	}
	if got := appendLearningQuery(t.Context(), ToolCall{Query: "*"}, dc, `Tool Output: {}`); strings.Contains(got, l.ID) {
		t.Fatal("revoked explicit recall", got)
	}
	if r := learningResult(t, dispatchLearning(t.Context(), rememberTestCall("I prefer green tea", "I prefer green tea"), dc)); r["status"] != "error" {
		t.Fatal("revoked remember", r)
	}
	for _, op := range []string{"status", "apply"} {
		_, status = decodeCorrectionTestResult(t, dispatchMemoryCorrection(t.Context(), ToolCall{Action: "correct_memory", Operation: op, ID: plan.ID}, dc))
		if status == "success" {
			t.Fatal("revoked correction", op)
		}
	}
	item := extractedLearning{LearningDraft: memory.LearningDraft{Statement: "I prefer green tea", Kind: "decision", Confidence: .99}, Quote: "I prefer green tea"}
	applyExtractedLearnings(t.Context(), dc.Cfg, dc.ShortTermMem, "analysis", item.Quote, []extractedLearning{item}, s.runCfg.learningAnalysisScope())
	list, err := dc.ShortTermMem.ListLearnings(t.Context(), dc.LearningScope, "green tea", "", 10)
	if err != nil || len(list) != 0 {
		t.Fatal("revoked background publication", list, err)
	}
	var child RunConfig
	inheritProjectRun(context.WithValue(t.Context(), projectRunKey{}, dc), &child)
	if child.LearningProjectValid == nil || child.LearningProjectValid(child.CodeProject) {
		t.Fatal("co-agent lost live revocation")
	}
}

func TestLearningHelperBatchCacheSeparatesActivation(t *testing.T) {
	client := &mockChatClient{response: `{"memory_analysis":{"facts":[],"preferences":[],"corrections":[],"pending_actions":[],"learnings":[]},"activity_digest":{}}`}
	m := &helperLLMManager{client: client, model: "helper-model", responseCache: map[string]string{}}
	for _, enabled := range []bool{false, true, true, false} {
		_, err := m.AnalyzeTurn(t.Context(), "Keep the Go backend portable", "Understood", nil, nil, nil, newHelperTurnBudget(t), enabled)
		if err != nil {
			t.Fatal(err)
		}
		if client.calls <= 2 && enabled && !strings.Contains(client.lastReq.Messages[0].Content, learningExtractionInstructions) {
			t.Fatal("enabled helper omitted evidence instructions")
		}
	}
	if client.calls != 2 {
		t.Fatal("helper cache crossed activation boundary", client.calls)
	}
}

func TestLearningHelperBatchPublishesThroughAgentLoop(t *testing.T) {
	run, client, cleanup := newPromptPipelineTestRunConfig(t, "learning-batch", "web_chat")
	defer cleanup()
	ResetGlobalHelperLLMManager()
	defer ResetGlobalHelperLLMManager()
	run.Config.Learnings.Enabled = true
	run.BudgetTracker = newHelperTurnBudget(t)
	run.Config.Tools.Memory.Enabled = true
	run.Config.LLM.HelperEnabled = true
	run.Config.LLM.HelperProviderType = "openai"
	run.Config.LLM.HelperBaseURL = "http://127.0.0.1:1/v1"
	run.Config.LLM.HelperResolvedModel = "gpt-4o-mini"
	const quote = "Keep the Go backend portable"
	helper := &mockChatClient{response: `{"memory_analysis":{"facts":[],"preferences":[],"corrections":[],"pending_actions":[],"learnings":[{"statement":"Keep the Go backend portable","rationale":"","kind":"decision","confidence":0.99,"quote":"Keep the Go backend portable"}]},"activity_digest":{}}`}
	m := newHelperLLMManager(run.Config, run.Logger)
	if m == nil {
		t.Fatal("helper unavailable")
	}
	m.client = helper
	client.response = "Understood, I will keep the backend portable."
	run.SideEffects = NewAsyncTaskGroup(t.Context())
	defer run.SideEffects.Shutdown(time.Second)
	_, err := ExecuteAgentLoop(t.Context(), openai.ChatCompletionRequest{Model: run.Config.LLM.Model, Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: quote}}}, run, false, NoopBroker{})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { run.SideEffects.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("analysis did not drain")
	}
	list, err := run.ShortTermMem.ListLearnings(t.Context(), memory.LearningScope{}, "portable", "active", 5)
	if err != nil || len(list) != 1 {
		t.Fatal("helper learning was not applied", list, err)
	}
	data, _ := json.Marshal(list[0].Sources)
	if !strings.Contains(string(data), quote) || helper.calls != 1 {
		t.Fatalf("wrong source or second analyzer: %s, calls=%d", data, helper.calls)
	}
}
