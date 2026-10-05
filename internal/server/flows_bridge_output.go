package server

import (
	"encoding/json"
	"sort"
	"strings"
	"unicode/utf8"

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
	// flowCutMarker ends a text that flowCutText cut.
	flowCutMarker = "…"
	// flowScrubMaxDepth bounds the nesting scrubFlowValue walks. Flow values are
	// JSON-normalised (the engine encodes and decodes every node output, and trigger data
	// comes from json.Unmarshal in NormalizeTriggerData or the store), and encoding/json does
	// not decode nesting deeper than 10000 levels (its maxNestingDepth), which is the bound
	// flows.ParseToolOutput and the engine inherit. A deeper value is dropped (nil) rather
	// than passed on unscrubbed.
	flowScrubMaxDepth = 10000
)

// flowRunOutcome maps a finished run to a Mission Control result and output text. The text
// is at most flowMissionOutputMaxBytes long (flowCutText). info.Outputs is encoded once, as
// given: FlowRunFinished scrubs it first (scrubFlowMap).
func flowRunOutcome(info flows.RunFinishedInfo) (string, string) {
	result, text := flowRunText(info)
	return result, flowCutText(text, flowMissionOutputMaxBytes)
}

func flowRunText(info flows.RunFinishedInfo) (string, string) {
	switch info.Result.Status {
	case flows.RunSuccess:
		if len(info.Outputs) == 0 {
			return tools.MissionResultSuccess, ""
		}
		data, _ := json.Marshal(info.Outputs)
		return tools.MissionResultSuccess, string(data)
	case flows.RunCancelled:
		msg := strings.TrimSpace(info.Result.ErrorMessage)
		if msg == "" || info.Result.ErrorCode == "" {
			msg = tools.MissionCancelledOutput
		}
		return tools.MissionResultError, msg
	default:
		msg := strings.TrimSpace(info.Result.ErrorMessage)
		if msg == "" {
			msg = "the flow failed"
		}
		if info.Result.ErrorCode != "" {
			msg = info.Result.ErrorCode + ": " + msg
		}
		return tools.MissionResultError, msg
	}
}

// flowCutText returns s when it has at most limit bytes, else its longest prefix that
// leaves room for flowCutMarker without splitting a UTF-8 sequence, followed by the marker.
func flowCutText(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit - len(flowCutMarker)
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + flowCutMarker
}

// scrubFlowMap is scrubFlowValue for a map; a nil map stays nil.
func scrubFlowMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out, _ := scrubFlowValue(m).(map[string]any)
	return out
}

// scrubFlowValue returns a copy of v in which security.Scrub has replaced the registered
// secrets in every string value and every map key, with the structure kept. Use it for any
// flow value (run outputs, trigger data, step data) before it leaves the flow service
// towards Mission Control, the API or a log.
//
// It scrubs the values, not their JSON text: a secret that holds a quote, a backslash or a
// control character appears escaped in JSON, where the scrubber does not find it, and a
// redaction inside the text can break the JSON. v is not modified (run outputs and trigger
// data are shared and read-only). Strings, maps and lists are walked; bools and numbers are
// kept; any other Go type is normalised through JSON first and becomes nil when it cannot
// be encoded. Nesting deeper than flowScrubMaxDepth becomes nil. Strings shorter than
// flowCredentialMinBytes are kept as they are: the scrubber registers no shorter value, and
// its encoded forms are longer still. Keys that scrub to the same text keep one value, the
// one of the first such key in sorted order.
func scrubFlowValue(v any) any { return scrubFlowValueAt(v, 0) }

func scrubFlowValueAt(v any, depth int) any {
	switch x := v.(type) {
	case nil, bool, float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return x
	case string:
		return scrubFlowText(x)
	case map[string]any:
		if x == nil {
			return x
		}
		if depth >= flowScrubMaxDepth {
			return nil
		}
		out := make(map[string]any, len(x))
		var renamed []string
		for k, item := range x {
			if scrubFlowText(k) != k {
				renamed = append(renamed, k)
				continue
			}
			out[k] = scrubFlowValueAt(item, depth+1)
		}
		sort.Strings(renamed)
		for _, k := range renamed {
			key := scrubFlowText(k)
			if _, taken := out[key]; !taken {
				out[key] = scrubFlowValueAt(x[k], depth+1)
			}
		}
		return out
	case []any:
		if x == nil {
			return x
		}
		if depth >= flowScrubMaxDepth {
			return nil
		}
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = scrubFlowValueAt(item, depth+1)
		}
		return out
	default:
		data, err := json.Marshal(x)
		if err != nil {
			return nil
		}
		var normalised any
		if json.Unmarshal(data, &normalised) != nil {
			return nil
		}
		return scrubFlowValueAt(normalised, depth)
	}
}

func scrubFlowText(s string) string {
	if len(s) < flowCredentialMinBytes {
		return s
	}
	return security.Scrub(s)
}
