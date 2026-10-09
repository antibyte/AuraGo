package agent

import (
	"math"
	"strconv"
	"strings"

	"aurago/internal/tools"
)

// decodeLocalWikipediaArgs reads a local_wikipedia call from native params
// first and falls back to the generic ToolCall fields of text-mode calls.
func decodeLocalWikipediaArgs(tc ToolCall) tools.LocalWikipediaRequest {
	return tools.LocalWikipediaRequest{
		Operation: firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Query:     firstNonEmptyToolString(toolArgString(tc.Params, "query"), tc.Query),
		Limit:     firstNonEmptyInt(toolArgInt(tc.Params, 0, "limit"), tc.Limit),
		Title:     firstNonEmptyToolString(toolArgString(tc.Params, "title"), tc.Title),
		Path:      firstNonEmptyToolString(toolArgString(tc.Params, "path"), tc.Path),
		Section:   localWikipediaSectionArg(tc.Params),
		Offset:    firstNonEmptyInt(toolArgInt(tc.Params, 0, "offset"), tc.Offset),
	}
}

// localWikipediaSectionArg accepts a heading or an index, also when a model
// sends the index as a JSON number.
func localWikipediaSectionArg(params map[string]interface{}) string {
	switch v := params["section"].(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		if v >= 0 && v == math.Trunc(v) {
			return strconv.Itoa(int(v))
		}
	case int:
		if v >= 0 {
			return strconv.Itoa(v)
		}
	}
	return ""
}
