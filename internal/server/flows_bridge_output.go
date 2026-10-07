package server

import (
	"bytes"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"

	"aurago/internal/flows"
	"aurago/internal/security"
	"aurago/internal/tools"
)

// Bounds of what the bridge hands on to Mission Control.
const (
	// flowMissionOutputMaxBytes caps the output text of a finished live run that Mission
	// Control gets. On success it is the JSON of the run's final outputs, which can reach
	// flows.MaxRunOutputBytes (32 MiB). The agent path caps nothing before
	// MissionManagerV2.SetResult, and Mission Control keeps at most 2000 bytes of the text
	// (history output, the "output" of mission_completed dependents, the audit detail;
	// LastOutput 500), but the audit recorder scrubs the whole text before it cuts it. 16 KiB
	// leaves every consumer its full share. A longer text is cut at a rune boundary and ends
	// with flowCutMarker.
	flowMissionOutputMaxBytes = 16 << 10
	// flowCutMarker ends a text the bridge cut.
	flowCutMarker = "…"
	// flowOutputsScrubBudget bounds the copy of a run's final outputs that the bridge scrubs
	// and hands on, in bytes of JSON (scrubFlowMapBounded). It is the size up to which
	// Mission Control hands outputs to mission_completed dependents as they are
	// (tools.flowCompletionOutputsMaxBytes, 64 KiB); larger outputs only ever reach them as a
	// preview. So the work per run no longer grows with the outputs, which can reach 32 MiB.
	flowOutputsScrubBudget = 64 << 10
	// flowOutputsPreviewBytes is the preview of cut outputs, as
	// tools.flowCompletionOutputsPreviewBytes makes it.
	flowOutputsPreviewBytes = 4 << 10
	// flowTriggerDataScrubBudget bounds the copy of a run's trigger data for the mission
	// history, which keeps 16 KiB of it (tools.flowHistoryTriggerDataMaxBytes).
	flowTriggerDataScrubBudget = 16 << 10
	// flowTriggerDataPreviewBytes is the preview of cut trigger data (boundFlowTriggerData).
	flowTriggerDataPreviewBytes = 7 << 10
	// flowScrubOverlapBytes is how much more of a text the bridge scrubs than it keeps when it
	// cuts the text (scrubFlowPrefix), so that a registered secret of up to this size that
	// crosses the cut is still found as a whole and no prefix of it stays.
	flowScrubOverlapBytes = 16 << 10
	// flowScrubMinBytes is the shortest text the walk scrubs. internal/security registers no
	// value shorter than 8 bytes (minimumGlobalSensitiveLiteralBytes in scrubber.go, also for
	// scoped values), and the encoded forms Scrub removes as well (fragmented, hex, base64)
	// are at least as long as the value, so a shorter text cannot hold any of them.
	// TestC15ScrubIgnoresShortValues pins the security side.
	flowScrubMinBytes = 8
	// flowScrubMaxDepth bounds the nesting the walk follows. Flow values are JSON-normalised
	// (the engine encodes and decodes every node output, and trigger data comes from
	// json.Unmarshal in NormalizeTriggerData or the store), and encoding/json does not decode
	// nesting deeper than 10000 levels (its maxNestingDepth), which is the bound
	// flows.ParseToolOutput and the engine inherit. A deeper value is dropped (nil) rather
	// than passed on unscrubbed.
	flowScrubMaxDepth = 10000
	// flowScrubSortedKeys is the largest map whose keys the walk visits in sorted order, so
	// that what a cut copy keeps and which of two keys that scrub alike survives do not
	// depend on map order. It is about the most keys a 64 KiB budget can hold; a larger map
	// is visited in map order, since sorting all its keys would cost more than the budget.
	flowScrubSortedKeys = 1 << 14
)

// flowScrub is security.Scrub; tests count the bytes it is given.
var flowScrub = security.Scrub

// flowRunOutputs is what a finished run hands to Mission Control, built by boundFlowOutputs
// from one bounded, scrubbed copy of the run's final outputs that is encoded once.
type flowRunOutputs struct {
	// mission goes to MissionManagerV2.FlowRunFinishedAtDepth for mission_completed dependents. It
	// is the copy, or, when the outputs did not fit in flowOutputsScrubBudget, the shape
	// tools.boundedCompletionOutputs gives larger outputs, {"_truncated": true, "_preview":
	// "<first flowOutputsPreviewBytes of the JSON>"}. That map is far below the tools limit,
	// so boundedCompletionOutputs encodes it as it is and never sees the full outputs. nil
	// when the run has no outputs map.
	mission map[string]any
	// text is the success output text: the copy's JSON, at most flowMissionOutputMaxBytes,
	// ending with flowCutMarker when anything was left out.
	text string
}

// boundFlowOutputs scrubs and bounds a run's final outputs for Mission Control.
func boundFlowOutputs(outputs map[string]any) flowRunOutputs {
	if outputs == nil {
		return flowRunOutputs{}
	}
	copied, truncated := scrubFlowMapBounded(outputs, flowOutputsScrubBudget)
	if len(copied) == 0 && !truncated {
		return flowRunOutputs{mission: copied}
	}
	enc, err := json.Marshal(copied)
	if err != nil { // the copy holds JSON values only
		return flowRunOutputs{mission: map[string]any{}}
	}
	text := string(enc)
	if !truncated && len(enc) <= flowOutputsScrubBudget {
		return flowRunOutputs{mission: copied, text: tools.CutWithMarker(text, flowMissionOutputMaxBytes, flowCutMarker)}
	}
	return flowRunOutputs{
		mission: map[string]any{"_truncated": true, "_preview": tools.CutAtRuneBoundary(text, flowOutputsPreviewBytes)},
		text:    tools.CutWithMarker(text+flowCutMarker, flowMissionOutputMaxBytes, flowCutMarker),
	}
}

// boundFlowTriggerData returns the JSON of a scrubbed copy of a run's trigger data for the
// mission history, bounded by flowTriggerDataScrubBudget, always valid JSON: the history
// (tools.flowHistoryTriggerData) cuts text over 16 KiB as text, so a copy that was cut, or
// whose encoding still exceeds that, becomes {"_truncated": true, "_preview": "<start of
// the JSON>"}, the shape cut outputs have, sized to stay within the 16 KiB. A nil map gives
// "null".
func boundFlowTriggerData(data map[string]any) string {
	copied, truncated := scrubFlowMapBounded(data, flowTriggerDataScrubBudget)
	enc, err := json.Marshal(copied)
	if err != nil {
		return "{}"
	}
	if !truncated && len(enc) <= flowTriggerDataScrubBudget {
		return string(enc)
	}
	// Inside the preview string JSON text at most doubles (quotes and backslashes; HTML
	// escaping is off), so half the budget fits; the loop is a guard.
	for preview := flowTriggerDataPreviewBytes; ; preview /= 2 {
		var buf bytes.Buffer
		e := json.NewEncoder(&buf)
		e.SetEscapeHTML(false)
		_ = e.Encode(map[string]any{"_truncated": true, "_preview": tools.CutAtRuneBoundary(string(enc), preview)})
		if out := strings.TrimSuffix(buf.String(), "\n"); len(out) <= flowTriggerDataScrubBudget || preview < 64 {
			return out
		}
	}
}

// flowRunOutcome maps a finished run to a Mission Control result and output text (see
// flowOutcome); it bounds and scrubs info.Outputs itself.
func flowRunOutcome(info flows.RunFinishedInfo) (string, string) {
	return flowOutcome(info, boundFlowOutputs(info.Outputs))
}

// flowOutcome maps a finished run to a Mission Control result and output text, at most
// flowMissionOutputMaxBytes long: on success the text of the bounded outputs, else the
// run's error text, scrubbed and cut at a rune boundary.
func flowOutcome(info flows.RunFinishedInfo, outputs flowRunOutputs) (string, string) {
	switch info.Result.Status {
	case flows.RunSuccess:
		return tools.MissionResultSuccess, outputs.text
	case flows.RunCancelled:
		msg := strings.TrimSpace(info.Result.ErrorMessage)
		if msg == "" || info.Result.ErrorCode == "" {
			msg = tools.MissionCancelledOutput
		}
		return tools.MissionResultError, flowBoundedText(msg)
	default:
		msg := strings.TrimSpace(info.Result.ErrorMessage)
		if msg == "" {
			msg = "the flow failed"
		}
		if info.Result.ErrorCode != "" {
			msg = info.Result.ErrorCode + ": " + msg
		}
		return tools.MissionResultError, flowBoundedText(msg)
	}
}

// flowBoundedText scrubs s and cuts it to flowMissionOutputMaxBytes, ending a cut text with
// flowCutMarker. It scrubs at most flowScrubOverlapBytes more than it keeps (scrubFlowWindow).
func flowBoundedText(s string) string {
	text, cut := scrubFlowWindow(s, flowMissionOutputMaxBytes)
	if cut {
		text += flowCutMarker
	}
	return tools.CutWithMarker(text, flowMissionOutputMaxBytes, flowCutMarker)
}

// scrubFlowPrefix returns the first limit bytes (at most, cut at a rune boundary) of s
// scrubbed (scrubFlowWindow).
func scrubFlowPrefix(s string, limit int) string {
	text, _ := scrubFlowWindow(s, limit)
	return tools.CutAtRuneBoundary(text, limit)
}

// scrubFlowWindow scrubs s for a caller that keeps at most limit bytes of it, without
// scrubbing much more than that. A text of up to limit+flowScrubOverlapBytes bytes is
// scrubbed whole (cut is false). Of a longer one only that window is scrubbed, and the
// window's cut can split a secret, whose prefix then stays unredacted at the very end of
// the scrubbed window. Redactions shrink the text before it (a 64-byte secret becomes the
// 10-byte placeholder), so that end can move into the first limit bytes. The last
// flowScrubOverlapBytes bytes of the scrubbed window are therefore dropped (cut is true):
// they hold any such prefix of a secret whose longest form (hex, twice the value) is at
// most flowScrubOverlapBytes long. When redactions shrank the window by more than limit,
// nothing is left; the caller keeps less rather than leak.
func scrubFlowWindow(s string, limit int) (text string, cut bool) {
	window := limit + flowScrubOverlapBytes
	if len(s) <= window {
		return scrubFlowText(s), false
	}
	scrubbed := scrubFlowText(tools.CutAtRuneBoundary(s, window))
	keep := len(scrubbed) - flowScrubOverlapBytes
	if keep <= 0 {
		return "", true
	}
	return tools.CutAtRuneBoundary(scrubbed, keep), true
}

// scrubFlowMap is scrubFlowValue for a map; a nil map stays nil.
func scrubFlowMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out, _ := scrubFlowValue(m).(map[string]any)
	return out
}

// scrubFlowMapBounded is scrubFlowMap with a budget: the copy takes at most about budget
// bytes of JSON, and only that much text is scrubbed (plus flowScrubOverlapBytes for the
// text the budget cuts). truncated reports that something was left out: entries are
// visited in sorted key order (see flowScrubSortedKeys) and list order, and the walk stops
// at the first value that does not fit; a string that does not fit keeps a scrubbed prefix
// of the bytes left. A nil map stays nil.
func scrubFlowMapBounded(m map[string]any, budget int) (copied map[string]any, truncated bool) {
	if m == nil {
		return nil, false
	}
	w := flowScrubWalk{left: budget}
	out, _ := w.value(m, 0)
	copied, _ = out.(map[string]any)
	if copied == nil { // not even "{}" fitted
		copied = map[string]any{}
	}
	return copied, w.truncated
}

// scrubFlowValue returns a copy of v in which security.Scrub has replaced the registered
// secrets in every string value, every map key and every number, with the structure kept.
// Use it for any flow value (run outputs, trigger data, step data) before it leaves the
// flow service towards Mission Control, the API or a log; use scrubFlowMapBounded where the
// value can be large.
//
// It scrubs the values, not their JSON text: a secret that holds a quote, a backslash or a
// control character appears escaped in JSON, where the scrubber does not find it, and a
// redaction inside the text can break the JSON. Strings, maps and lists are walked; a
// number is scrubbed as its JSON text and becomes the redacted string when that changes
// it; bools and nil are kept; NaN and infinities (not JSON) become nil; any other Go type is
// normalised through JSON first and becomes nil when it cannot be encoded. Nesting deeper
// than flowScrubMaxDepth becomes nil. Texts shorter than flowScrubMinBytes are kept as
// they are. Keys that scrub to the same text keep one value, that of the key visited
// first (in sorted order for maps up to flowScrubSortedKeys keys).
//
// It differs from scrubJSONStrings (sse.go), which scrubs decoded SSE payloads: it never
// modifies its input (scrubJSONStrings rewrites lists in place; run outputs and trigger
// data are shared and read-only), it bounds the nesting depth (and, as
// scrubFlowMapBounded, the bytes it copies and scrubs), and it resolves keys that scrub
// alike deterministically where scrubJSONStrings keeps whichever map order writes last.
func scrubFlowValue(v any) any {
	w := flowScrubWalk{left: math.MaxInt}
	out, _ := w.value(v, 0)
	return out
}

// flowScrubWalk copies and scrubs a flow value within a budget of JSON bytes.
type flowScrubWalk struct {
	left      int  // bytes of JSON the copy may still take
	truncated bool // something was left out
}

// take reserves n bytes of the budget. When they are not left, the walk is over: nothing
// more is taken and the copy counts as cut.
func (w *flowScrubWalk) take(n int) bool {
	if n > w.left {
		w.left, w.truncated = 0, true
		return false
	}
	w.left -= n
	return true
}

// value returns the scrubbed copy of v at nesting level depth, and false when v did not
// fit (it is then left out, and the walk is over).
func (w *flowScrubWalk) value(v any, depth int) (any, bool) {
	switch x := v.(type) {
	case nil:
		return nil, w.take(len("null"))
	case bool:
		return x, w.take(len("false"))
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil, w.take(len("null"))
		}
		text := flowJSONNumber(x)
		if !w.take(len(text)) {
			return nil, false
		}
		if scrubbed := scrubFlowText(text); scrubbed != text {
			return scrubbed, true
		}
		return x, true
	case string:
		return w.text(x)
	case map[string]any:
		if x == nil || depth >= flowScrubMaxDepth {
			return nil, w.take(len("null"))
		}
		return w.object(x, depth)
	case []any:
		if x == nil || depth >= flowScrubMaxDepth {
			return nil, w.take(len("null"))
		}
		if !w.take(len("[]")) {
			return nil, false
		}
		out := make([]any, 0, min(len(x), 1024))
		for _, item := range x {
			if !w.take(len(",")) {
				break
			}
			copied, ok := w.value(item, depth+1)
			if !ok {
				break
			}
			out = append(out, copied)
		}
		return out, true
	default:
		data, err := json.Marshal(x)
		if err != nil {
			return nil, w.take(len("null"))
		}
		var normalised any
		if json.Unmarshal(data, &normalised) != nil {
			return nil, w.take(len("null"))
		}
		return w.value(normalised, depth)
	}
}

// object copies a map: its keys in sorted order up to flowScrubSortedKeys keys (else in map
// order), each key scrubbed; a key that scrubs to one already copied is skipped.
func (w *flowScrubWalk) object(x map[string]any, depth int) (any, bool) {
	if !w.take(len("{}")) {
		return nil, false
	}
	out := make(map[string]any, min(len(x), 1024))
	visit := func(k string) bool {
		if !w.take(len(k) + len(`"":,`)) {
			return false
		}
		key := scrubFlowText(k)
		if _, taken := out[key]; taken {
			return true
		}
		copied, ok := w.value(x[k], depth+1)
		if !ok {
			return false
		}
		out[key] = copied
		return true
	}
	if len(x) > flowScrubSortedKeys {
		for k := range x {
			if !visit(k) {
				break
			}
		}
		return out, true
	}
	keys := make([]string, 0, len(x))
	for k := range x {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !visit(k) {
			break
		}
	}
	return out, true
}

// text copies a string; one that does not fit keeps a scrubbed prefix of the bytes left.
func (w *flowScrubWalk) text(s string) (any, bool) {
	if len(s)+len(`""`) <= w.left {
		out := scrubFlowText(s)
		w.left = max(w.left-len(out)-len(`""`), 0) // a redaction can make the text longer
		return out, true
	}
	room := w.left - len(`""`)
	w.left, w.truncated = 0, true
	if room <= 0 {
		return nil, false
	}
	return scrubFlowPrefix(s, room), true
}

// flowJSONNumber formats f as encoding/json does (it then also shortens exponents such as
// e-07 to e-7, which no secret relies on).
func flowJSONNumber(f float64) string {
	format := byte('f')
	if abs := math.Abs(f); abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		format = 'e'
	}
	return strconv.FormatFloat(f, format, -1, 64)
}

func scrubFlowText(s string) string {
	if len(s) < flowScrubMinBytes {
		return s
	}
	return flowScrub(s)
}
