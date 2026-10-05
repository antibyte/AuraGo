package server

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync/atomic"
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
	// Background only: the JSON text scrub of the plan could not find it.
	if raw, _ := json.Marshal(in); !strings.Contains(security.Scrub(string(raw)), escapedText) {
		t.Log("security.Scrub now finds JSON-escaped secrets in text; the walk does not depend on it")
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

// c15CountScrub counts the bytes the bridge hands to the scrubber until the test ends.
func c15CountScrub(t *testing.T) *atomic.Int64 {
	t.Helper()
	var n atomic.Int64
	old := flowScrub
	t.Cleanup(func() { flowScrub = old })
	flowScrub = func(s string) string {
		n.Add(int64(len(s)))
		return old(s)
	}
	return &n
}

// c15BigOutputs returns the outputs of nodes final nodes, each a list of perNode 8-byte
// strings (the shape of the reviewer's probe).
func c15BigOutputs(nodes, perNode int) map[string]any {
	out := map[string]any{}
	for n := range nodes {
		items := make([]any, perNode)
		for i := range items {
			items[i] = fmt.Sprintf("%08d", i)
		}
		out[fmt.Sprintf("node%d", n)] = map[string]any{"items": items}
	}
	return out
}

// The scrub work of a run is bounded by the budgets, not by the size of its outputs or
// trigger data, and Mission Control gets the preview shape of the tools package for
// outputs that did not fit.
func TestC15ScrubCostIsBounded(t *testing.T) {
	big := c15BigOutputs(7, 100000) // about 7 MiB of JSON in 700000 strings
	scrubbed := c15CountScrub(t)

	bounded := boundFlowOutputs(big)
	if n := scrubbed.Load(); n > flowOutputsScrubBudget+flowScrubOverlapBytes {
		t.Fatalf("scrubbed %d bytes for the outputs", n)
	}
	preview, _ := bounded.mission["_preview"].(string)
	if bounded.mission["_truncated"] != true || len(bounded.mission) != 2 || len(preview) > flowOutputsPreviewBytes ||
		!strings.HasPrefix(preview, `{"node0":{"items":["00000000","00000001"`) {
		t.Fatalf("mission outputs = %v", bounded.mission)
	}
	if enc, _ := json.Marshal(bounded.mission); len(enc) > flowOutputsScrubBudget {
		t.Fatalf("the preview encodes to %d bytes; the tools package would cut it again", len(enc))
	}
	if len(bounded.text) > flowMissionOutputMaxBytes || !strings.HasSuffix(bounded.text, flowCutMarker) ||
		!strings.HasPrefix(bounded.text, `{"node0":{"items":["00000000"`) {
		t.Fatalf("text: %d bytes, %q", len(bounded.text), flowBoundRunes(bounded.text, 40))
	}
	if again := boundFlowOutputs(big); again.text != bounded.text || !reflect.DeepEqual(again.mission, bounded.mission) {
		t.Fatal("the bounded copy depends on map order")
	}

	// Through the bridge: one run's start and end.
	e := c15NewEnv(t)
	id := e.c15Mission(t, "flow_c15costly01", "Costly")
	scrubbed.Store(0)
	histID := e.bridge.FlowRunStarted(id, flows.RunRecord{ID: "run_c15costly001", FlowID: "flow_c15costly01", TriggerType: "webhook",
		TriggerData: map[string]any{"payload": c15BigOutputs(1, 25000)}}) // about 250 KiB
	if n := scrubbed.Load(); n > flowTriggerDataScrubBudget+flowScrubOverlapBytes {
		t.Fatalf("scrubbed %d bytes for the trigger data", n)
	}
	td := e.c15Run(t, histID).TriggerData
	var stored map[string]any
	_ = json.Unmarshal([]byte(td), &stored)
	if preview, _ := stored["_preview"].(string); len(td) > flowTriggerDataScrubBudget || stored["_truncated"] != true ||
		!strings.HasPrefix(preview, `{"payload":{"node0":{"items":["00000000"`) {
		t.Fatalf("the cut trigger data is not a valid preview: %d bytes, %q", len(td), td[max(len(td)-40, 0):])
	}
	scrubbed.Store(0)
	e.bridge.FlowRunFinished(flows.RunFinishedInfo{MissionID: id, HistoryID: histID, FlowName: "Costly", Started: true,
		Record: flows.RunRecord{ID: "run_c15costly001", FlowID: "flow_c15costly01"}, Result: flows.RunResult{Status: flows.RunSuccess},
		Outputs: big})
	if n := scrubbed.Load(); n > flowOutputsScrubBudget+flowScrubOverlapBytes {
		t.Fatalf("scrubbed %d bytes for the run's end", n)
	}
	if m, _ := e.s.MissionManagerV2.Get(id); m.LastResult != tools.MissionResultSuccess || !strings.HasPrefix(m.LastOutput, `{"node0":`) {
		t.Fatalf("mission = %+v", m)
	}
}

// Values that fit stay whole; a tight budget keeps keys in sorted order and a scrubbed
// prefix of the string it cuts.
func TestC15BoundedScrubKeepsWhatFits(t *testing.T) {
	secret := "c15-bounded-secret"
	release := security.RegisterScopedSensitiveExact(secret)
	t.Cleanup(release)
	in := map[string]any{"b": []any{"x", 1.0, map[string]any{"k": secret}}, "a": "text"}
	copied, truncated := scrubFlowMapBounded(in, flowOutputsScrubBudget)
	if truncated || !reflect.DeepEqual(copied, scrubFlowMap(in)) {
		t.Fatalf("copy = %v, truncated %v", copied, truncated)
	}
	// "{}" takes 2 bytes, the key "a" 5, so 3 bytes of "text" are left.
	if copied, truncated := scrubFlowMapBounded(in, 12); !truncated || !reflect.DeepEqual(copied, map[string]any{"a": "tex"}) {
		t.Fatalf("tight copy = %v, truncated %v", copied, truncated)
	}
	// A secret across the cut is scrubbed as a whole before the cut.
	long := map[string]any{"t": strings.Repeat("y", 30) + secret}
	copied, _ = scrubFlowMapBounded(long, 2+5+2+35)
	if text, _ := copied["t"].(string); strings.Contains(text, secret[:5]) || len(text) != 35 {
		t.Fatalf("cut text = %q", text)
	}
	if copied, truncated := scrubFlowMapBounded(nil, 10); copied != nil || truncated {
		t.Fatal("a nil map must stay nil")
	}
}

// Numbers are scrubbed as their JSON text, like scrubJSONStrings does, so a numeric secret
// does not pass as a number.
func TestC15ScrubWalkScrubsNumbers(t *testing.T) {
	release := security.RegisterScopedSensitiveExact("12345678901")
	t.Cleanup(release)
	out := scrubFlowMap(map[string]any{"id": 12345678901.0, "n": 42.0, "nan": math.NaN()})
	if out["id"] != security.RedactedText("") || out["n"] != 42.0 || out["nan"] != nil {
		t.Fatalf("out = %v", out)
	}
}

// flowScrubMinBytes rests on internal/security registering no value shorter than 8 bytes.
func TestC15ScrubIgnoresShortValues(t *testing.T) {
	short := "c15abcd" // 7 bytes
	security.RegisterSensitive(short)
	t.Cleanup(security.RegisterScopedSensitiveExact(short))
	if got := security.Scrub("x " + short + " y"); got != "x "+short+" y" {
		t.Fatalf("security scrubs %d-byte values now (%q); lower flowScrubMinBytes", len(short), got)
	}
	eight := "c15abcde"
	t.Cleanup(security.RegisterScopedSensitiveExact(eight))
	if len(eight) != flowScrubMinBytes || scrubFlowText(eight) != security.RedactedText("") {
		t.Fatalf("an %d-byte value is not scrubbed", len(eight))
	}
}

// c15Dense returns pad dots followed by secret+"--" repeated until n bytes are reached.
func c15Dense(secret string, pad, n int) string {
	var sb strings.Builder
	sb.WriteString(strings.Repeat(".", pad))
	for sb.Len() < n {
		sb.WriteString(secret)
		sb.WriteString("--")
	}
	return sb.String()
}

// A densely repeated secret shrinks the scrubbed window by far more than the overlap
// (64 bytes become the 10-byte placeholder); the kept text must still end before the
// unredacted prefix of the secret the window's end splits, at every alignment, in the
// prefix cut, the error text, the trigger data and the outputs.
func TestC15ScrubWindowNeverKeepsASecretPrefix(t *testing.T) {
	secret := "C15-TOKEN-" + strings.Repeat("Z", 54) // 64 bytes
	t.Cleanup(security.RegisterScopedSensitiveExact(secret))
	prefix := secret[:8]
	check := func(what string, pad int, text string) {
		t.Helper()
		if strings.Contains(text, prefix) {
			i := strings.Index(text, prefix)
			t.Fatalf("%s, pad %d: secret prefix at %d of %d: %q", what, pad, i, len(text), text[i:min(len(text), i+40)])
		}
	}
	for pad := range 66 {
		check("scrubFlowPrefix", pad, scrubFlowPrefix(c15Dense(secret, pad, 4096+flowScrubOverlapBytes+4096), 4096))
		check("flowBoundedText", pad, flowBoundedText(c15Dense(secret, pad, flowMissionOutputMaxBytes+flowScrubOverlapBytes+4096)))
		data := boundFlowTriggerData(map[string]any{"raw": c15Dense(secret, pad, 200<<10)})
		check("boundFlowTriggerData", pad, data)
		if !json.Valid([]byte(data)) || len(data) > flowTriggerDataScrubBudget {
			t.Fatalf("trigger data, pad %d: %d bytes, valid JSON %v", pad, len(data), json.Valid([]byte(data)))
		}
		if pad%11 == 0 {
			b := boundFlowOutputs(map[string]any{"doc": c15Dense(secret, pad, flowOutputsScrubBudget+flowScrubOverlapBytes+4096)})
			enc, _ := json.Marshal(b.mission)
			check("boundFlowOutputs text", pad, b.text)
			check("boundFlowOutputs mission", pad, string(enc))
		}
	}
	// Without redactions nothing more than the overlap is dropped: the full limit is kept.
	plain := strings.Repeat("p", 4096+flowScrubOverlapBytes+100)
	if got := scrubFlowPrefix(plain, 4096); len(got) != 4096 {
		t.Fatalf("plain prefix = %d bytes", len(got))
	}
	if got := flowBoundedText(plain + strings.Repeat("p", flowMissionOutputMaxBytes)); len(got) != flowMissionOutputMaxBytes ||
		!strings.HasSuffix(got, flowCutMarker) {
		t.Fatalf("plain bounded text = %d bytes", len(got))
	}
}

// Trigger data that fits stays the plain copy; a copy that was cut becomes the preview
// object, valid JSON within what the history keeps, so the history stores it unchanged.
func TestC15TriggerDataStaysValidJSON(t *testing.T) {
	if got := boundFlowTriggerData(map[string]any{"a": "b"}); got != `{"a":"b"}` {
		t.Fatalf("small = %s", got)
	}
	if got := boundFlowTriggerData(nil); got != "null" {
		t.Fatalf("nil = %s", got)
	}
	// Quotes, backslashes and HTML characters double or worse when escaped in the preview.
	heavy := strings.Repeat(`"\<>&`, 20000)
	got := boundFlowTriggerData(map[string]any{"raw": heavy})
	var obj map[string]any
	if err := json.Unmarshal([]byte(got), &obj); err != nil || obj["_truncated"] != true || len(got) > flowTriggerDataScrubBudget {
		t.Fatalf("preview: %d bytes, %v, %v", len(got), err, obj["_truncated"])
	}
	if tools.CutWithMarker(got, flowTriggerDataScrubBudget, "...[truncated]") != got {
		t.Fatal("the history would cut the preview")
	}
}
