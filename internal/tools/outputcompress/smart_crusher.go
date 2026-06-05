package outputcompress

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const smartCrusherMinInputChars = 2000

// SmartCrusherConfig controls the generic JSON array-of-objects compressor.
type SmartCrusherConfig struct {
	Enabled  bool // master toggle
	MaxRows  int  // max rows to render before tail-truncation (default: 50)
	TailRows int  // rows to keep at tail when truncating (default: 5)
	MaxCols  int  // max columns to emit; wider tables are minified instead (default: 20)
}

// smartCrushJSON attempts to compress large JSON outputs into a more token-efficient
// representation. Arrays of uniform objects are rendered as compact tables;
// everything else falls back to minified JSON when that yields meaningful savings.
// The second return value is true only when compression was applied.
func smartCrushJSON(input string, cfg SmartCrusherConfig) (string, bool) {
	if len(input) < smartCrusherMinInputChars {
		return input, false
	}

	trimmed := strings.TrimSpace(input)
	if !(strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{")) {
		return input, false
	}

	// Fast path: validate JSON without the heavy cost of Unmarshal when possible.
	if !json.Valid([]byte(trimmed)) {
		return input, false
	}

	var data interface{}
	if err := json.Unmarshal([]byte(trimmed), &data); err != nil {
		return input, false
	}

	if arr, ok := data.([]interface{}); ok && len(arr) > 2 {
		if isArrayOfUniformObjects(arr) {
			crushed := crushArrayOfObjects(arr, cfg)
			if meetsSavingsThreshold(input, crushed, conservativeRollbackMinSavingsPercent) {
				return crushed, true
			}
			return input, false
		}
	}

	// Fallback: minified JSON if it is meaningfully shorter.
	minified, err := json.Marshal(data)
	if err != nil {
		return input, false
	}
	if len(minified) < len(input)*85/100 {
		return string(minified), true
	}

	return input, false
}

// isArrayOfUniformObjects returns true when every element is a map[string]interface{}.
func isArrayOfUniformObjects(arr []interface{}) bool {
	for _, item := range arr {
		if _, ok := item.(map[string]interface{}); !ok {
			return false
		}
	}
	return len(arr) > 0
}

// crushArrayOfObjects renders a JSON array of objects as a compact TSV-like table.
// If the objects are too heterogeneous or too wide, it falls back to minified JSON.
func crushArrayOfObjects(arr []interface{}, cfg SmartCrusherConfig) string {
	sharedKeys := findSharedKeys(arr, 0.6)
	if len(sharedKeys) == 0 || len(sharedKeys) > cfg.MaxCols {
		// Too heterogeneous or too wide → minify.
		b, _ := json.Marshal(arr)
		return string(b)
	}

	maxRows := cfg.MaxRows
	if maxRows <= 0 {
		maxRows = 50
	}
	tailRows := cfg.TailRows
	if tailRows <= 0 {
		tailRows = 5
	}

	var b strings.Builder
	b.WriteString("[JSON_ARRAY_COMPACT]\n")
	escapedKeys := make([]string, len(sharedKeys))
	for i, k := range sharedKeys {
		escapedKeys[i] = escapeForTSV(k)
	}
	b.WriteString(strings.Join(escapedKeys, "\t"))
	b.WriteByte('\n')

	limit := len(arr)
	showTail := false
	tailStart := -1
	if len(arr) > maxRows+tailRows {
		limit = maxRows
		showTail = true
		tailStart = len(arr) - tailRows
	}

	for i, item := range arr {
		if showTail && i >= limit && i < tailStart {
			if i == limit {
				b.WriteString(fmt.Sprintf("... (%d rows omitted) ...\n", tailStart-limit))
			}
			continue
		}
		if obj, ok := item.(map[string]interface{}); ok {
			vals := make([]string, len(sharedKeys))
			for j, key := range sharedKeys {
				vals[j] = escapeForTSV(compactValue(obj[key]))
			}
			b.WriteString(strings.Join(vals, "\t"))
			b.WriteByte('\n')
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

// findSharedKeys extracts keys that are present in at least `thresholdPct` of objects.
func findSharedKeys(arr []interface{}, thresholdPct float64) []string {
	if len(arr) == 0 {
		return nil
	}

	// Count key occurrences across all objects.
	keyCounts := make(map[string]int)
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		for key := range obj {
			keyCounts[key]++
		}
	}

	threshold := int(float64(len(arr)) * thresholdPct)
	if threshold < 1 {
		threshold = 1
	}

	shared := make([]string, 0, len(keyCounts))
	for key, count := range keyCounts {
		if count >= threshold {
			shared = append(shared, key)
		}
	}
	sort.Strings(shared)
	return shared
}

// compactValue turns a JSON value into a short string suitable for tabular display.
func compactValue(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		if len(val) > 80 {
			return val[:77] + "..."
		}
		return val
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	case []interface{}:
		return fmt.Sprintf("[...%d items]", len(val))
	case map[string]interface{}:
		return fmt.Sprintf("{...%d keys}", len(val))
	case json.Number:
		return val.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// escapeForTSV sanitises a string so it does not break the tabular format.
func escapeForTSV(s string) string {
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}
