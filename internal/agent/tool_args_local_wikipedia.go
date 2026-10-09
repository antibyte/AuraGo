package agent

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"aurago/internal/tools"
)

// decodeLocalWikipediaArgs reads a local_wikipedia call from native params
// first and falls back to the generic ToolCall fields of text-mode calls.
func decodeLocalWikipediaArgs(tc ToolCall) tools.LocalWikipediaRequest {
	offset, ok := localWikipediaWholeNumberArg(tc.Params, "offset")
	if !ok {
		offset = tc.Offset
	}
	return tools.LocalWikipediaRequest{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Query:     firstNonEmptyToolString(toolArgString(tc.Params, "query"), tc.Query),
		Limit:     firstNonEmptyInt(toolArgInt(tc.Params, 0, "limit"), tc.Limit),
		Title:     firstNonEmptyToolString(toolArgString(tc.Params, "title"), tc.Title),
		Path:      firstNonEmptyToolString(toolArgString(tc.Params, "path"), tc.Path),
		Section:   localWikipediaSectionArg(tc.Params),
		Offset:    offset,
	}
}

// localWikipediaMaxIndex bounds section indexes and offsets taken from JSON
// numbers; it fits int on every platform.
const localWikipediaMaxIndex = math.MaxInt32

// localWikipediaSectionArg accepts a heading or an index, also when a model
// sends the index as a JSON number. A number that is not a whole number in
// 0..localWikipediaMaxIndex (2.5, -1, 1e300) is passed on as heading text,
// so the library answers with the article's sections instead of reading a
// wrapped-around or silently rounded index. Isolation tags and entities a
// model echoes back are removed by the library, as for title and path.
func localWikipediaSectionArg(params map[string]interface{}) string {
	switch v := params["section"].(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		if i, ok := localWikipediaWholeNumber(v); ok {
			return strconv.Itoa(i)
		}
		return strconv.FormatFloat(v, 'g', -1, 64)
	case int:
		return strconv.Itoa(v) // a negative index reads as heading text
	case int64:
		return strconv.FormatInt(v, 10)
	case json.Number:
		return strings.TrimSpace(v.String())
	}
	return ""
}

// localWikipediaWholeNumberArg reads a non-negative whole number; ok is false
// when the key is absent or not a number. A number outside
// 0..localWikipediaMaxIndex or with a fraction reads as -1, which the tool
// rejects as an invalid request instead of rounding or wrapping it.
func localWikipediaWholeNumberArg(params map[string]interface{}, key string) (int, bool) {
	var f float64
	switch v := params[key].(type) {
	case float64:
		f = v
	case int:
		f = float64(v)
	case int64:
		f = float64(v)
	case json.Number:
		parsed, err := v.Float64()
		if err != nil {
			return -1, true
		}
		f = parsed
	default:
		return 0, false
	}
	if i, ok := localWikipediaWholeNumber(f); ok {
		return i, true
	}
	return -1, true
}

func localWikipediaWholeNumber(v float64) (int, bool) {
	if v >= 0 && v <= localWikipediaMaxIndex && v == math.Trunc(v) {
		return int(v), true
	}
	return 0, false
}
