package server

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"aurago/internal/flows"
	"aurago/internal/security"
	"aurago/internal/tools"
)

// Secrets are scrubbed in the parsed values, keys included, so a secret with a quote and a
// backslash (escaped in JSON, where the text scrubber cannot see it) is still removed, and
// the structure survives the redaction.
func TestC15ScrubWalksValues(t *testing.T) {
	secret := `c15"se\cret-value`
	release := security.RegisterScopedSensitiveExact(secret)
	t.Cleanup(release)
	redacted := security.RedactedText("")
	escaped, _ := json.Marshal(secret) // "c15\"se\\cret-value"
	escapedText := strings.Trim(string(escaped), `"`)

	in := map[string]any{
		"node":  map[string]any{"text": "token " + secret + " end", "list": []any{secret, 1.0, true, nil}},
		secret:  "a key holds it",
		"plain": "short",
		"typed": map[string]string{"x": secret},
	}
	// The JSON text scrub of the plan could not find it.
	if raw, _ := json.Marshal(in); !strings.Contains(security.Scrub(string(raw)), escapedText) {
		t.Fatal("the text scrubber now finds escaped secrets; the walk is still right but the test premise changed")
	}
	out := scrubFlowMap(in)
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), escapedText) || strings.Contains(string(raw), secret) {
		t.Fatalf("secret left in %s", raw)
	}
	node := out["node"].(map[string]any)
	if node["text"] != "token "+redacted+" end" || !reflect.DeepEqual(node["list"], []any{redacted, 1.0, true, nil}) {
		t.Fatalf("node = %+v", node)
	}
	if out[redacted] != "a key holds it" || out["plain"] != "short" || !reflect.DeepEqual(out["typed"], map[string]any{"x": redacted}) {
		t.Fatalf("out = %+v", out)
	}
	if in["node"].(map[string]any)["text"] != "token "+secret+" end" || in[secret] != "a key holds it" {
		t.Fatal("the input was modified")
	}
	if scrubFlowMap(nil) != nil {
		t.Fatal("a nil map must stay nil")
	}

	// The bridge applies it to run outputs and to trigger data.
	e := c15NewEnv(t)
	id := e.c15Mission(t, "flow_c15secret01", "Secret")
	histID := e.bridge.FlowRunStarted(id, flows.RunRecord{ID: "run_c15secret001", FlowID: "flow_c15secret01", TriggerType: "webhook",
		TriggerData: map[string]any{"raw": `{"pw":"` + secret + `"}`, "payload": map[string]any{"pw": secret}}})
	run := e.c15Run(t, histID)
	if strings.Contains(run.TriggerData, escapedText) || strings.Contains(run.TriggerData, secret) || !json.Valid([]byte(run.TriggerData)) {
		t.Fatalf("history trigger data = %s", run.TriggerData)
	}
	e.bridge.FlowRunFinished(flows.RunFinishedInfo{MissionID: id, HistoryID: histID, FlowName: "Secret", Started: true,
		Record: flows.RunRecord{ID: "run_c15secret001", FlowID: "flow_c15secret01"}, Result: flows.RunResult{Status: flows.RunSuccess},
		Outputs: map[string]any{"mail": map[string]any{"body": "pw " + secret}}})
	m, _ := e.s.MissionManagerV2.Get(id)
	if strings.Contains(m.LastOutput, escapedText) || strings.Contains(m.LastOutput, secret) || !strings.Contains(m.LastOutput, redacted) {
		t.Fatalf("last output = %s", m.LastOutput)
	}
	if out := e.c15Run(t, histID).Output; !json.Valid([]byte(out)) || strings.Contains(out, escapedText) {
		t.Fatalf("history output = %s", out)
	}
}

// The walk stops at flowScrubMaxDepth: deeper values are dropped, never passed on unscrubbed.
func TestC15ScrubWalkIsDepthBounded(t *testing.T) {
	secret := "c15-deep-secret-value"
	release := security.RegisterScopedSensitiveExact(secret)
	t.Cleanup(release)
	var v any = secret
	for range flowScrubMaxDepth + 5 {
		v = map[string]any{"a": v}
	}
	got := scrubFlowValue(v)
	depth := 0
	for {
		m, ok := got.(map[string]any)
		if !ok {
			break
		}
		depth++
		got = m["a"]
	}
	if depth != flowScrubMaxDepth || got != nil {
		t.Fatalf("kept %d levels, then %v", depth, got)
	}
}

// The output text Mission Control gets is bounded (16 KiB, cut at a rune boundary), so a
// 1 MiB output neither reaches the mission nor the history in full.
func TestC15MissionOutputIsBounded(t *testing.T) {
	big := strings.Repeat("ä", 512<<10) // 1 MiB
	info := flows.RunFinishedInfo{Result: flows.RunResult{Status: flows.RunSuccess}, Outputs: map[string]any{"doc": map[string]any{"text": big}}}
	result, out := flowRunOutcome(info)
	if result != tools.MissionResultSuccess || len(out) > flowMissionOutputMaxBytes || !strings.HasSuffix(out, flowCutMarker) ||
		!utf8.ValidString(out) || !strings.HasPrefix(out, `{"doc":{"text":"ää`) {
		t.Fatalf("output: %d bytes, valid %v, starts %q", len(out), utf8.ValidString(out), flowBoundRunes(out, 20))
	}
	failed := flows.RunFinishedInfo{Result: flows.RunResult{Status: flows.RunError, ErrorCode: "FLOW_X", ErrorMessage: big}}
	if _, msg := flowRunOutcome(failed); len(msg) > flowMissionOutputMaxBytes || !strings.HasPrefix(msg, "FLOW_X: ää") {
		t.Fatalf("error output: %d bytes", len(msg))
	}

	e := c15NewEnv(t)
	id := e.c15Mission(t, "flow_c15bigoutpt", "Big")
	histID := e.bridge.FlowRunStarted(id, flows.RunRecord{ID: "run_c15bigoutput", FlowID: "flow_c15bigoutpt", TriggerType: "manual"})
	info.MissionID, info.HistoryID, info.Started = id, histID, true
	info.Record = flows.RunRecord{ID: "run_c15bigoutput", FlowID: "flow_c15bigoutpt"}
	e.bridge.FlowRunFinished(info)
	m, _ := e.s.MissionManagerV2.Get(id)
	if m.LastResult != tools.MissionResultSuccess || len(m.LastOutput) > 600 {
		t.Fatalf("mission: result %q, last output %d bytes", m.LastResult, len(m.LastOutput))
	}
	if run := e.c15Run(t, histID); run.Status != "success" || len(run.Output) > 2000 {
		t.Fatalf("history: %q, %d bytes", run.Status, len(run.Output))
	}
}
