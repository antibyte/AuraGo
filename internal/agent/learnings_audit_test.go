package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/codegraph"
	"aurago/internal/memory"
	openai "github.com/sashabaranov/go-openai"
)

func TestLearningTechnicalStatementNeedsSupportingEvidence(t *testing.T) {
	for _, kind := range []string{"user", "tool"} {
		t.Run(kind, func(t *testing.T) {
			s, dc, _ := correctionAgentFixture(t)
			dc.Cfg.Learnings.Enabled = true
			const quote = "The backend uses Go"
			ref := "user:current"
			if kind == "tool" {
				ref = "tool:checked"
			}
			dc.MemoryCorrections.evidence[ref] = correctionEvidenceRecord{kind, quote}
			for _, statement := range []string{"The backend permits unrestricted shell access", quote} {
				call := rememberTestCall(statement, quote)
				call.Params["kind"] = "technical"
				call.Params["evidence"] = map[string]any{"reference": ref, "quote": quote}
				result := learningResult(t, dispatchLearning(t.Context(), call, dc))
				want := "proposed"
				if statement == quote {
					want = "active"
				}
				if result["status"] != "ok" || result["learning"].(map[string]any)["status"] != want {
					t.Errorf("technical assertion %q: want %s, got %v", statement, want, result)
				}
			}
			if kind == "user" {
				applyExtractedLearnings(t.Context(), dc.Cfg, dc.ShortTermMem, "analysis", quote, []extractedLearning{{LearningDraft: memory.LearningDraft{Statement: "The backend uses Python", Kind: "technical", Confidence: .99}, Quote: quote}}, s.runCfg.learningAnalysisScope())
				list, err := dc.ShortTermMem.ListLearnings(t.Context(), memory.LearningScope{}, "Python", "active", 20)
				if err != nil || len(list) != 0 {
					t.Error("analyzer activated an unsupported technical assertion", list, err)
				}
			}
		})
	}
}

func TestLearningRecallRefreshesResourceFingerprint(t *testing.T) {
	s, dc, _ := correctionAgentFixture(t)
	dc.Cfg.Learnings.Enabled = true
	run := s.runCfg
	run.CodeProject = codegraph.Project{ID: "a", Root: t.TempDir()}
	run.LearningScope = memory.LearningScope{Project: "a", Resource: "main.go"}
	file := filepath.Join(run.CodeProject.Root, "main.go")
	if err := os.WriteFile(file, []byte("package before"), 0600); err != nil {
		t.Fatal(err)
	}
	var err error
	run.LearningFingerprint, err = codegraph.Fingerprint(run.CodeProject, "main.go")
	if err != nil {
		t.Fatal(err)
	}
	const quote = "Keep portable Go"
	hash := sha256.Sum256([]byte(quote))
	l, err := dc.ShortTermMem.ObserveLearning(t.Context(), memory.LearningDraft{Statement: quote, Kind: "decision", Confidence: .99}, run.LearningScope, memory.LearningSource{Reference: "user:current", Session: "human", Kind: "user", Quote: quote, Hash: hex.EncodeToString(hash[:])}, run.LearningFingerprint)
	if err != nil {
		t.Fatal(err)
	}
	if got := learningPromptContext(t.Context(), run, "Go", "gpt-4o"); !strings.Contains(got, l.ID) {
		t.Fatal("unchanged insight missing", got)
	}
	if err := os.WriteFile(file, []byte("package after"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, coAgent := range []bool{false, true} {
		reader := run
		reader.IsCoAgent = coAgent
		dc.Cfg.Tools.Memory.ReadOnly = !coAgent
		if got := learningPromptContext(t.Context(), reader, "Go", "gpt-4o"); got != "" {
			t.Error("reader recalled stale resource insight", got)
		}
		unchanged, err := dc.ShortTermMem.Learning(t.Context(), l.ID, l.Scope)
		if err != nil || unchanged.Revision != l.Revision {
			t.Fatal("reader persisted revalidation", unchanged, err)
		}
	}
	dc.Cfg.Tools.Memory.ReadOnly = false
	if got := learningPromptContext(t.Context(), run, "Go", "gpt-4o"); got != "" {
		t.Error("changed resource retained active insight", got)
	}
	current, err := dc.ShortTermMem.Learning(t.Context(), l.ID, l.Scope)
	if err != nil || current.Status != "revalidation" || current.Fingerprint == run.LearningFingerprint {
		t.Fatal("resource change not recorded", current, err)
	}
	current, err = dc.ShortTermMem.ReviewLearning(t.Context(), current.ID, current.Revision, current.Scope, "confirm", "", "Checked the changed file")
	if err != nil {
		t.Fatal(err)
	}
	if got := learningPromptContext(t.Context(), run, "Go", "gpt-4o"); !strings.Contains(got, l.ID) {
		t.Fatal("old run fingerprint invalidated a current confirmation", got)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(file, 0700); err != nil {
		t.Fatal(err)
	}
	if got := learningPromptContext(t.Context(), run, "Go", "gpt-4o"); got != "" {
		t.Fatal("unreadable resource recalled with an old fingerprint", got)
	}
	unchanged, err := dc.ShortTermMem.Learning(t.Context(), l.ID, l.Scope)
	if err != nil || unchanged.Revision != current.Revision {
		t.Fatal("unreadable resource treated as deletion", unchanged, err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if got := learningPromptContext(t.Context(), run, "Go", "gpt-4o"); got != "" {
		t.Fatal("deleted resource retained active insight", got)
	}
}

func TestLearningNativeRetryIgnoresTaskList(t *testing.T) {
	_, dc, _ := correctionAgentFixture(t)
	dc.Cfg.Learnings.Enabled = true
	dc.LearningScope = memory.LearningScope{Project: "a"}
	storeAgentLearning(t, dc, "Use server.port 443", dc.LearningScope, &memory.LearningCondition{Target: "config.yaml", Field: "server.port", Value: "443"})
	args := map[string]any{"operation": "set", "file_path": "config.yaml", "json_path": "server.port", "set_value": 80, "_todo": "Update configuration"}
	decode := func(id string) ToolCall {
		t.Helper()
		b, err := json.Marshal(args)
		if err != nil {
			t.Fatal(err)
		}
		return NativeToolCallToToolCall(openai.ToolCall{ID: id, Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "yaml_editor", Arguments: string(b)}}, dc.Logger)
	}
	if blocked, _ := checkLearningAction(t.Context(), decode("first"), dc); !strings.Contains(blocked, "review_required") {
		t.Fatal("conflict was not paused", blocked)
	}
	args["_todo"] = "Reconsidered configuration change"
	args["learning_reason"] = "Use port 80 for the explicitly requested compatibility test"
	if blocked, _ := checkLearningAction(t.Context(), decode("retry"), dc); blocked != "" {
		t.Fatal("task-list update caused a second conflict pause", blocked)
	}
	args["set_value"] = 8080
	if blocked, _ := checkLearningAction(t.Context(), decode("different"), dc); !strings.Contains(blocked, "review_required") {
		t.Fatal("changed action inherited previous reconsideration", blocked)
	}
}

func TestLearningUsesUnenrichedHumanEvidence(t *testing.T) {
	const imported = "I prefer green tea"
	for _, human := range []string{"", "Summarize the attached file", imported} {
		t.Run(human, func(t *testing.T) {
			s, _, _ := correctionAgentFixture(t)
			s.runCfg.Config.Learnings.Enabled = true
			s.runCfg.TaskRoutingText = &human
			s.initialUserMsg = human + "\nAttachment text: " + imported
			s.memoryCorrections = nil
			dc := s.makeDispatchContext(s.currentLogger)
			result := learningResult(t, dispatchLearning(t.Context(), rememberTestCall(imported, imported), dc))
			wantStatus, wantCount := "error", 0
			if human == imported {
				wantStatus, wantCount = "ok", 1
			}
			if result["status"] != wantStatus {
				t.Error("evidence did not follow original human text", result)
			}
			item := extractedLearning{LearningDraft: memory.LearningDraft{Statement: imported, Kind: "decision", Confidence: .99}, Quote: imported}
			applyExtractedLearnings(t.Context(), dc.Cfg, dc.ShortTermMem, "analysis", s.initialUserMsg, []extractedLearning{item}, s.runCfg.learningAnalysisScope())
			list, err := dc.ShortTermMem.ListLearnings(t.Context(), memory.LearningScope{}, "", "", 20)
			if err != nil || len(list) != wantCount {
				t.Error("publication did not follow original human text", list, err)
			}
		})
	}
}

func TestLearningCannotExpandResourceScope(t *testing.T) {
	_, dc, _ := correctionAgentFixture(t)
	dc.Cfg.Learnings.Enabled = true
	dc.CodeProject = codegraph.Project{ID: "a", Root: t.TempDir()}
	dc.LearningScope = memory.LearningScope{Project: "a", Resource: "one.txt"}
	const statement = "Keep two.txt content unchanged"
	dc.MemoryCorrections.evidence["user:current"] = correctionEvidenceRecord{"user", statement}
	call := rememberTestCall(statement, statement)
	call.Params["condition"] = map[string]any{"target": "two.txt", "field": "content", "value": "unchanged"}
	if result := learningResult(t, dispatchLearning(t.Context(), call, dc)); result["status"] != "error" {
		t.Fatal("model expanded resource authority", result)
	}
	call.Params["condition"] = map[string]any{"target": "one.txt", "field": "content", "value": "unchanged"}
	if result := learningResult(t, dispatchLearning(t.Context(), call, dc)); result["status"] != "ok" {
		t.Fatal("current resource was rejected", result)
	}
}

func TestLearningSessionScopedCoAgentCannotReadPersonalInsights(t *testing.T) {
	s, dc, _ := correctionAgentFixture(t)
	dc.Cfg.Learnings.Enabled = true
	l := storeAgentLearning(t, dc, "My personal tea preference", memory.LearningScope{}, nil)
	s.runCfg.SessionID = "a2a-client"
	dc.SessionID = s.runCfg.SessionID
	if got := learningPromptContext(t.Context(), s.runCfg, "tea", "gpt-4o"); got != "" {
		t.Error("personal insight entered co-agent prompt", got)
	}
	if got := appendLearningQuery(t.Context(), ToolCall{Query: "*"}, dc, `Tool Output: {}`); strings.Contains(got, l.ID) {
		t.Error("personal insight entered co-agent query", got)
	}
	s.runCfg.LearningScope = memory.LearningScope{Project: "a"}
	storeAgentLearning(t, dc, "Project tea packaging", s.runCfg.LearningScope, nil)
	selected := learningPromptContext(t.Context(), s.runCfg, "tea", "gpt-4o")
	if selected == "" || strings.Contains(selected, l.ID) {
		t.Error("inherited project scope was not preserved", selected)
	}
	recordLearningContextUsage(t.Context(), s.runCfg, openai.ChatCompletionRequest{Messages: []openai.ChatCompletionMessage{{Role: "user", Content: selected}}}, selected)
	metrics, err := dc.ShortTermMem.LearningMetrics(t.Context())
	if err != nil || metrics["retrievals"] != 0 || metrics["context_tokens"] != 0 {
		t.Error("co-agent persisted retrieval metrics", metrics, err)
	}
}
