package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func correctionAgentFixture(t *testing.T) (*agentLoopState, *DispatchContext, string) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	id, err := stm.AddCoreMemoryFact("User prefers black coffee")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Tools.Memory.Enabled = true
	cfg.Personality.UserProfiling = true
	s := &agentLoopState{runCfg: RunConfig{Config: cfg, ShortTermMem: stm, SessionID: t.Name()}, initialUserMsg: "I prefer green tea", currentLogger: logger, turnSnapshot: &turnContextSnapshot{MemoryReady: true}}
	dc := s.makeDispatchContext(logger)
	return s, dc, fmt.Sprint(id)
}

func correctionTestCall(id string) ToolCall {
	return ToolCall{Action: "correct_memory", Operation: "preview", Reason: "User corrected their preference", CorrectionChanges: []memory.MemoryCorrectionChange{{Reference: memory.MemoryReference{Store: "core", ID: id}, Operation: "replace", Value: "User prefers green tea"}}, CorrectionEvidence: []correctionEvidenceRequest{{Reference: "user:current", Quote: "I prefer green tea"}}}
}

func decodeCorrectionTestResult(t *testing.T, out string) (*memory.MemoryCorrection, string) {
	t.Helper()
	text, _ := toolResultPayload(out)
	if text == "" {
		text = out
	}
	text = html.UnescapeString(text)
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end < start {
		t.Fatal(out)
	}
	var value struct {
		Status     string                   `json:"status"`
		Correction *memory.MemoryCorrection `json:"correction"`
	}
	if err := json.Unmarshal([]byte(text[start:end+1]), &value); err != nil {
		t.Fatal(err, out)
	}
	return value.Correction, value.Status
}

func TestCorrectMemoryUsesRealEvidenceAndRefreshesSameTurn(t *testing.T) {
	s, dc, id := correctionAgentFixture(t)
	out := dispatchMemoryCorrection(t.Context(), correctionTestCall(id), dc)
	p, status := decodeCorrectionTestResult(t, out)
	if status != "success" || p == nil {
		t.Fatal(out)
	}
	apply := ToolCall{Action: "correct_memory", Operation: "apply", ID: p.ID}
	out = dispatchMemoryCorrection(t.Context(), apply, dc)
	p, status = decodeCorrectionTestResult(t, out)
	if status != "success" || p.Status != "applied" {
		t.Fatal(out)
	}
	s.recordCorrectionEvidence(apply, out, true)
	if !s.coreMemDirty || !s.turnSnapshot.MemoryDirty {
		t.Fatal("same-turn cache remains stale")
	}
	out, err := executeQueryMemory(ToolCall{Query: "green tea"}, dc.SessionID, dc.ShortTermMem, nil, nil, nil, nil)
	if err != nil || !strings.Contains(out, "green tea") || !strings.Contains(out, `"references"`) || strings.Contains(out, "black coffee") {
		t.Fatal(out, err)
	}
	if strings.Contains(correctionDiagnostic(apply, out), "tea") {
		t.Fatal("diagnostic leaked contents")
	}
}

func TestCorrectMemoryContextSearchIncludesProfile(t *testing.T) {
	_, dc, _ := correctionAgentFixture(t)
	if err := dc.ShortTermMem.UpsertProfileEntry("prefs", "drink", "green tea", "analysis"); err != nil {
		t.Fatal(err)
	}
	for _, sources := range [][]string{nil, {"profile"}} {
		out, err := executeContextMemoryQuery(ToolCall{Query: "green tea", Sources: sources}, dc.SessionID, dc.ShortTermMem, nil, nil, nil, nil)
		if err != nil || !strings.Contains(out, `"source":"profile"`) || !strings.Contains(out, "green tea") {
			t.Fatal("context search omitted profile", out, err)
		}
	}
}

func TestCorrectMemoryDiagnosticsDoNotStoreMemoryText(t *testing.T) {
	call := ToolCall{Action: "correct_memory", Operation: "private remembered fact", ID: "mc_private remembered fact", DispatchStatus: ToolResultFailed}
	got := correctionDiagnostic(call, "private remembered fact")
	if strings.Contains(got, "private") || !strings.Contains(got, "status=failed") {
		t.Fatal("diagnostic leaked arguments or lost status", got)
	}
}

func TestCorrectMemoryRejectsForgedAndAutonomousEvidence(t *testing.T) {
	for _, mode := range []string{"unknown", "invented-quote", "confirmed-flag", "coagent", "readonly", "disabled", "follow-up", "missing-ledger"} {
		t.Run(mode, func(t *testing.T) {
			s, dc, id := correctionAgentFixture(t)
			call := correctionTestCall(id)
			switch mode {
			case "unknown":
				call.CorrectionEvidence[0].Reference = "user:invented"
			case "invented-quote":
				call.CorrectionEvidence[0].Quote = "User confirmed everything"
			case "confirmed-flag":
				call.Params = map[string]interface{}{"confirmed": true}
			case "coagent":
				dc.IsCoAgent = true
			case "readonly":
				dc.Cfg.Tools.Memory.ReadOnly = true
			case "disabled":
				dc.Cfg.Tools.Memory.Enabled = false
			case "follow-up":
				s.memoryCorrections = nil
				s.runCfg.MessageSource = "follow_up"
				dc = s.makeDispatchContext(s.currentLogger)
			case "missing-ledger":
				dc.MemoryCorrections = nil
			}
			out := dispatchMemoryCorrection(t.Context(), call, dc)
			_, status := decodeCorrectionTestResult(t, out)
			if status == "success" {
				t.Fatal(out)
			}
			if !strings.Contains(dc.ShortTermMem.ReadCoreMemory(), "black coffee") {
				t.Fatal("unpermitted mutation")
			}
		})
	}
}

func TestCorrectMemoryApplyRejectsArgumentInjection(t *testing.T) {
	_, dc, id := correctionAgentFixture(t)
	p, _ := decodeCorrectionTestResult(t, dispatchMemoryCorrection(t.Context(), correctionTestCall(id), dc))
	call := ToolCall{Action: "correct_memory", Operation: "apply", ID: p.ID, CorrectionEvidence: []correctionEvidenceRequest{{Reference: "user:current", Quote: "I prefer green tea"}}}
	_, status := decodeCorrectionTestResult(t, dispatchMemoryCorrection(t.Context(), call, dc))
	if status == "success" {
		t.Fatal("apply accepted new evidence instead of bound ID")
	}
	call.CorrectionEvidence = nil
	call.Params = map[string]interface{}{"operation": "apply", "id": p.ID, "changes": nil, "evidence": nil, "reason": nil}
	_, status = decodeCorrectionTestResult(t, dispatchMemoryCorrection(t.Context(), call, dc))
	if status != "success" {
		t.Fatal("Strict optional nulls were rejected")
	}
}

func TestCorrectMemoryInvokePolicyAndSchemas(t *testing.T) {
	_, dc, id := correctionAgentFixture(t)
	schemas := BuildNativeToolSchemas(t.TempDir(), nil, ToolFeatureFlags{MemoryEnabled: true}, dc.Logger)
	SetDiscoverToolsState(dc.SessionID, schemas, schemas, "")
	t.Cleanup(func() { ClearDiscoverToolsState(dc.SessionID) })
	call := correctionTestCall(id)
	args, err := json.Marshal(memoryCorrectionArgs{Operation: call.Operation, Reason: call.Reason, Changes: call.CorrectionChanges, Evidence: call.CorrectionEvidence})
	if err != nil {
		t.Fatal(err)
	}
	var params map[string]interface{}
	if err = json.Unmarshal(args, &params); err != nil {
		t.Fatal(err)
	}
	delete(params, "id")
	wrapped := ToolCall{Action: "invoke_tool", Params: map[string]interface{}{"tool_name": "correct_memory", "arguments": params}}
	result := DispatchToolCallResult(context.Background(), &wrapped, dc, "I prefer green tea")
	if result.IsError {
		t.Fatal(result.Output)
	}
	p, status := decodeCorrectionTestResult(t, result.Output)
	if status != "success" || p == nil {
		t.Fatal(result.Output)
	}
	dc.IsCoAgent = true
	for _, call := range []ToolCall{correctionTestCall(id), {Action: "invoke_tool", Params: map[string]interface{}{"tool_name": "correct_memory", "arguments": params}}} {
		result := DispatchToolCallResult(context.Background(), &call, dc, "I prefer green tea")
		if !result.IsError {
			t.Fatal("coagent bypass", result.Output)
		}
	}
}

func TestCorrectMemoryToolEvidenceRequiresSuccessfulAllowedRead(t *testing.T) {
	s, _, _ := correctionAgentFixture(t)
	for _, call := range []ToolCall{{Action: "query_memory"}, {Action: "execute_shell"}, {Action: "api_request"}} {
		if got := s.recordCorrectionEvidence(call, `{"status":"success"}`, true); strings.Contains(got, "Correction evidence reference:") {
			t.Fatal("untrusted tool authorized evidence")
		}
	}
	if got := s.recordCorrectionEvidence(ToolCall{Action: "system_metrics"}, `{"status":"error"}`, false); strings.Contains(got, "Correction evidence reference:") {
		t.Fatal("failed check became evidence")
	}
	got := s.recordCorrectionEvidence(ToolCall{Action: "system_metrics"}, `{"cpu_count":8}`, true)
	_, ref, ok := strings.Cut(got, "Correction evidence reference: ")
	if !ok {
		t.Fatal(got)
	}
	if _, err := s.correctionRun().resolveEvidence([]correctionEvidenceRequest{{Reference: ref, Quote: `"cpu_count":8`}}); err != nil {
		t.Fatal(err)
	}
}

func TestCorrectMemoryProfileSearchKeepsExistingResults(t *testing.T) {
	_, dc, _ := correctionAgentFixture(t)
	if err := dc.ShortTermMem.UpsertProfileEntry("prefs", "drink", "green tea", "analysis"); err != nil {
		t.Fatal(err)
	}
	out, err := executeQueryMemory(ToolCall{Query: "tea"}, dc.SessionID, dc.ShortTermMem, nil, nil, nil, nil)
	if err != nil || !strings.Contains(out, `"source":"user_profile"`) || !strings.Contains(out, `"category":"prefs"`) {
		t.Fatal(out, err)
	}
}
