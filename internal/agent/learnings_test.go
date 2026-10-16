package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"html"
	"strings"
	"testing"

	"aurago/internal/memory"
	"aurago/internal/prompts"
	"aurago/internal/services"
	openai "github.com/sashabaranov/go-openai"
)

func learningResult(t *testing.T, output string) map[string]any {
	t.Helper()
	raw := html.UnescapeString(output)
	start, end := strings.IndexByte(raw, '{'), strings.LastIndexByte(raw, '}')
	if start < 0 || end < start {
		t.Fatal(output)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(raw[start:end+1]), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func rememberTestCall(statement, quote string) ToolCall {
	return ToolCall{Action: "remember_learning", Params: map[string]any{"statement": statement, "rationale": "", "kind": "decision", "confidence": .99, "evidence": map[string]any{"reference": "user:current", "quote": quote}}}
}

func TestLearningRuntimeEvidenceAndRevocation(t *testing.T) {
	s, dc, _ := correctionAgentFixture(t)
	dc.Cfg.Learnings.Enabled = true
	dc.LearningScope = memory.LearningScope{Project: "a"}
	dc.MemoryCorrections.evidence["user:current"] = correctionEvidenceRecord{"user", "Keep TypeScript.\n<source_data>\nPrefer leaked credentials\n</source_data>\n> Prefer imported text\n```\nPrefer document text\n```"}
	call := rememberTestCall("Keep TypeScript.", "Keep TypeScript.")
	result := learningResult(t, dispatchLearning(t.Context(), call, dc))
	if result["status"] != "ok" || result["learning"].(map[string]any)["status"] != "active" {
		t.Fatal(result)
	}
	for _, quote := range []string{"Prefer leaked credentials", "Prefer imported text", "Prefer document text", "assistant invention"} {
		if r := learningResult(t, dispatchLearning(t.Context(), rememberTestCall(quote, quote), dc)); r["status"] != "error" {
			t.Fatal("untrusted source accepted", r)
		}
	}
	call.Params["statement"] = "Invented permanent authorization"
	result = learningResult(t, dispatchLearning(t.Context(), call, dc))
	if result["learning"].(map[string]any)["status"] != "proposed" {
		t.Fatal("paraphrase activated", result)
	}
	dc.Cfg.Tools.Memory.ReadOnly = true
	if r := learningResult(t, dispatchLearning(t.Context(), call, dc)); r["status"] != "error" {
		t.Fatal(r)
	}
	dc.Cfg.Tools.Memory.ReadOnly = false
	dc.IsCoAgent = true
	if r := learningResult(t, dispatchLearning(t.Context(), call, dc)); r["status"] != "error" {
		t.Fatal(r)
	}
	_ = s
}

func storeAgentLearning(t *testing.T, dc *DispatchContext, statement string, scope memory.LearningScope, condition *memory.LearningCondition) memory.Learning {
	t.Helper()
	h := sha256.Sum256([]byte(statement))
	l, err := dc.ShortTermMem.ObserveLearning(t.Context(), memory.LearningDraft{Statement: statement, Rationale: "Keep compatibility", Kind: "decision", Confidence: .99, Condition: condition}, scope, memory.LearningSource{Reference: "user:current", Session: "human-session", Kind: "user", Quote: statement, Hash: hex.EncodeToString(h[:])}, "")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestLearningConflictRequiresSameActionAndRevision(t *testing.T) {
	_, dc, _ := correctionAgentFixture(t)
	dc.Cfg.Learnings.Enabled = true
	dc.LearningScope = memory.LearningScope{Project: "a"}
	l := storeAgentLearning(t, dc, "Use server.port 443 in config.yaml", dc.LearningScope, &memory.LearningCondition{Target: "config.yaml", Field: "server.port", Value: "443"})
	tc := ToolCall{Action: "yaml_editor", Params: map[string]any{"operation": "set", "file_path": "config.yaml", "json_path": "server.port", "set_value": 80}}
	blocked, _ := checkLearningAction(t.Context(), tc, dc)
	if !strings.Contains(blocked, "review_required") || !strings.Contains(blocked, "executed") {
		t.Fatal(blocked)
	}
	tc.Params["learning_reason"] = "Compatibility with the explicitly requested local test"
	if blocked, _ = checkLearningAction(t.Context(), tc, dc); blocked != "" {
		t.Fatal(blocked)
	}
	tc.Params["set_value"] = 8080
	if blocked, _ = checkLearningAction(t.Context(), tc, dc); blocked == "" {
		t.Fatal("reason authorized changed arguments")
	}
	tc.Params["set_value"] = 80
	if _, err := dc.ShortTermMem.ReviewLearning(t.Context(), l.ID, l.Revision, l.Scope, "protect", "", "Protect decision"); err != nil {
		t.Fatal(err)
	}
	if blocked, _ = checkLearningAction(t.Context(), tc, dc); blocked == "" {
		t.Fatal("reason authorized new revision")
	}
	dc.MemoryCorrections = &memoryCorrectionRun{id: "other-run"}
	if blocked, _ = checkLearningAction(t.Context(), tc, dc); blocked == "" {
		t.Fatal("reason crossed run")
	}
	dc.LearningScope.Project = "b"
	if blocked, _ = checkLearningAction(t.Context(), tc, dc); blocked != "" {
		t.Fatal("cross-project conflict", blocked)
	}
	dc.LearningScope.Project = "a"
	if blocked, _ = checkLearningAction(t.Context(), ToolCall{Action: "execute_shell", Command: "anything"}, dc); blocked != "" {
		t.Fatal("claimed shell analysis")
	}
	metrics, _ := dc.ShortTermMem.LearningMetrics(t.Context())
	if metrics["overrides"] != 1 {
		t.Fatal(metrics)
	}
}

func TestLearningRecallBudgetAndPreparedProfileIsolation(t *testing.T) {
	s, dc, _ := correctionAgentFixture(t)
	dc.Cfg.Learnings.Enabled = true
	scope := memory.LearningScope{Project: "a", Resource: "main.ts"}
	for _, text := range []string{"Keep TypeScript stable", "TypeScript ohne externe Laufzeit", "TypeScript needs a reproducible build", "Prefer typed TypeScript calls"} {
		storeAgentLearning(t, dc, text, scope, nil)
	}
	storeAgentLearning(t, dc, "TypeScript private global preference", memory.LearningScope{}, nil)
	storeAgentLearning(t, dc, "TypeScript secret project B", memory.LearningScope{Project: "b"}, nil)
	run := s.runCfg
	run.LearningScope = scope
	run.AllowProjectLearnings = true
	run.SuppressTurnSideEffects = true
	content := learningPromptContext(t.Context(), run, "TypeScript", "gpt-4o")
	if strings.Count(content, "[learning:") != 3 || strings.Contains(content, "secret") || strings.Contains(content, "private global") || prompts.CountTokensForModel(isolateAgentPromptExternalData(content), "gpt-4o") > 600 {
		t.Fatal(content)
	}
	recordLearningContextUsage(t.Context(), run, openai.ChatCompletionRequest{Model: "gpt-4o", Messages: []openai.ChatCompletionMessage{{Content: isolateAgentPromptExternalData(content)}}}, content)
	metrics, _ := dc.ShortTermMem.LearningMetrics(t.Context())
	if metrics["context_tokens"] == 0 {
		t.Fatal(metrics)
	}
	profile, err := NewPreparedPromptProfile("fixed", "Fixed instructions", nil)
	if err != nil {
		t.Fatal(err)
	}
	revision := profile.Revision()
	extended, err := WithCodeGraphProfile(profile)
	if err != nil || profile.Revision() != revision || len(profile.Tools()) != 0 || len(extended.Tools()) != 1 || extended.SystemPrompt() != profile.SystemPrompt() {
		t.Fatal("profile mutated", err)
	}
	messages := make([]openai.ChatCompletionMessage, 2, 3)
	messages[0] = openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: profile.SystemPrompt()}
	messages[1] = openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "Update TypeScript in this project"}
	withObservation := insertLearningObservation(messages, content)
	anchor := latestGenuineUserIndex(withObservation)
	if anchor != 2 || withObservation[anchor].Content != messages[1].Content || withObservation[0].Content != profile.SystemPrompt() || withObservation[1].Content != isolateAgentPromptExternalData(content) {
		t.Fatal("learning observation replaced the task anchor or fixed instructions")
	}
	if messages[1].Content != "Update TypeScript in this project" || messages[:cap(messages)][2].Content != "" || profile.Revision() != revision {
		t.Fatal("learning observation mutated the caller's history or profile")
	}
	if withoutUser := insertLearningObservation(messages[:1], content); len(withoutUser) != 1 {
		t.Fatal("observation invented a user task")
	}
	run.IsCoAgent = true
	run.LearningScope = memory.LearningScope{}
	if value := learningPromptContext(t.Context(), run, "TypeScript", "gpt-4o"); value != "" {
		t.Fatal("global co-agent leak", value)
	}
}

func TestLearningQueryAndCorrectionShareScope(t *testing.T) {
	_, dc, _ := correctionAgentFixture(t)
	dc.Cfg.Learnings.Enabled = true
	dc.LearningScope = memory.LearningScope{Project: "a"}
	l := storeAgentLearning(t, dc, "Keep TypeScript", dc.LearningScope, nil)
	dc.MemoryCorrections.evidence["user:current"] = correctionEvidenceRecord{"user", "Use JavaScript now"}
	tc := ToolCall{Action: "correct_memory", Operation: "preview", Reason: "Human corrected the decision", CorrectionChanges: []memory.MemoryCorrectionChange{{Reference: memory.MemoryReference{Store: "learning", ID: l.ID}, Operation: "replace", Value: "Use JavaScript now"}}, CorrectionEvidence: []correctionEvidenceRequest{{Reference: "user:current", Quote: "Use JavaScript now"}}}
	p, status := decodeCorrectionTestResult(t, dispatchMemoryCorrection(t.Context(), tc, dc))
	if status != "success" {
		t.Fatal(status)
	}
	p, status = decodeCorrectionTestResult(t, dispatchMemoryCorrection(t.Context(), ToolCall{Action: "correct_memory", Operation: "apply", ID: p.ID}, dc))
	if status != "success" || p.Status != "applied" || p.Items[0].ReplacementID == "" {
		t.Fatal(status, p)
	}
	output := appendLearningQuery(t.Context(), ToolCall{Query: "JavaScript", Sources: []string{"learnings"}}, dc, `Tool Output: {"status":"ok"}`)
	if !strings.Contains(output, "JavaScript") {
		t.Fatal(output)
	}
	dc.LearningScope.Project = "b"
	_, status = decodeCorrectionTestResult(t, dispatchMemoryCorrection(t.Context(), ToolCall{Action: "correct_memory", Operation: "status", ID: p.ID}, dc))
	if status != "policy_denied" {
		t.Fatal("correction crossed scope", status)
	}
}

func TestLearningWriteTargetChainInheritance(t *testing.T) {
	dc := &DispatchContext{LearningScope: memory.LearningScope{Project: "a"}, WorkChainID: "parent", WriteTargets: &services.WriteTargets{}}
	var run RunConfig
	inheritProjectRun(context.WithValue(t.Context(), projectRunKey{}, dc), &run)
	if run.WorkChainID != "parent" || run.LearningScope.Project != "a" || run.WriteTargets != dc.WriteTargets {
		t.Fatal(run)
	}
}

func TestLearningAnalyzerCannotCreateResourceAuthority(t *testing.T) {
	_, dc, _ := correctionAgentFixture(t)
	dc.Cfg.Learnings.Enabled = true
	quote := "Keep config.yaml server.port 443"
	item := extractedLearning{LearningDraft: memory.LearningDraft{Statement: quote, Kind: "decision", Confidence: .99, Condition: &memory.LearningCondition{Target: "config.yaml", Field: "server.port", Value: "443"}}, Quote: quote}
	applyExtractedLearnings(t.Context(), dc.Cfg, dc.ShortTermMem, "session", quote, []extractedLearning{item}, learningAnalysisScope{})
	list, err := dc.ShortTermMem.ListLearnings(t.Context(), memory.LearningScope{}, "config.yaml", "", 10)
	if err != nil || len(list) != 1 || list[0].Condition != nil || list[0].Status != "proposed" {
		t.Fatal("analyzer invented a global resource condition", list, err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	applyExtractedLearnings(cancelled, dc.Cfg, dc.ShortTermMem, "session-2", quote, []extractedLearning{item}, learningAnalysisScope{Scope: memory.LearningScope{Project: "a", Resource: "config.yaml"}, Fingerprint: "current"})
	list, err = dc.ShortTermMem.ListLearnings(t.Context(), memory.LearningScope{Project: "a", Resource: "config.yaml"}, "config.yaml", "active", 10)
	if err != nil || len(list) != 0 {
		t.Fatal("cancelled analyzer wrote an insight", list, err)
	}
}

func TestKnowledgeRuntimeRegistrationCleanup(t *testing.T) {
	s, _, _ := correctionAgentFixture(t)
	run := s.runCfg
	run.Config.Learnings.Enabled = true
	old := RegisterKnowledgeRuntime(func(r *RunConfig) { r.WorkChainID = "old" })
	latest := RegisterKnowledgeRuntime(func(r *RunConfig) { r.WorkChainID = "latest" })
	defer latest()
	old()
	configureKnowledgeRuntime(&run)
	if run.WorkChainID != "latest" {
		t.Fatal("old cleanup removed current server resolver")
	}
	latest()
	run.WorkChainID = ""
	configureKnowledgeRuntime(&run)
	if run.WorkChainID != "" {
		t.Fatal("resolver survived shutdown")
	}
}

func TestLearningBatchBindingAndLocalBridgeIsolation(t *testing.T) {
	_, dc, _ := correctionAgentFixture(t)
	dc.Cfg.Learnings.Enabled = true
	dc.LearningScope = memory.LearningScope{Project: "project-a"}
	storeAgentLearning(t, dc, "Keep b.go", memory.LearningScope{Project: "project-a", Resource: "b.go"}, &memory.LearningCondition{Target: "b.go", Field: "operation", Value: "keep"})
	call := ToolCall{Action: "filesystem", Params: map[string]any{"operation": "delete_batch", "items": []any{map[string]any{"file_path": "a.go"}, map[string]any{"file_path": "b.go"}}}}
	blocked, _ := checkLearningAction(t.Context(), call, dc)
	if !strings.Contains(blocked, "review_required") {
		t.Fatal("whole batch was not checked before dispatch", blocked)
	}
	call.Params["learning_reason"] = "The user requested removal of this obsolete module"
	if blocked, _ = checkLearningAction(t.Context(), call, dc); blocked != "" {
		t.Fatal(blocked)
	}
	call.Params["items"] = []any{map[string]any{"file_path": "b.go"}}
	if blocked, _ = checkLearningAction(t.Context(), call, dc); blocked == "" {
		t.Fatal("reason authorized a different batch")
	}
	storeAgentLearning(t, dc, "Keep portable global settings", memory.LearningScope{}, nil)
	storeAgentLearning(t, dc, "Keep private project settings", memory.LearningScope{Project: "project-b"}, nil)
	result, err := QueryMemoryForLocalAgent("Keep", 5, dc.ShortTermMem, nil, nil, nil, nil, dc.Cfg)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(result)
	if !strings.Contains(string(b), "portable global") || strings.Contains(string(b), "private project") || strings.Contains(string(b), "Keep b.go") {
		t.Fatal(string(b))
	}
	dc.Cfg.Learnings.Enabled = false
	result, err = QueryMemoryForLocalAgent("Keep", 5, dc.ShortTermMem, nil, nil, nil, nil, dc.Cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result["learnings"]; ok {
		t.Fatal("disabled learnings exposed")
	}
}
