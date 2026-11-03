package agent

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"aurago/internal/ecalc"
	"aurago/internal/tools"
)

// decodeECalcArgs reads an ecalc call from native params first and falls back to the generic ToolCall fields of
// text-mode calls. The variable values are called inputs on the wire because ToolCall already owns a "values" field
// for spreadsheets, which only decodes arrays of arrays.
func decodeECalcArgs(tc ToolCall) tools.ECalcRequest {
	return tools.ECalcRequest{
		Operation: strings.ToLower(strings.TrimSpace(firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")))),
		Query:     firstNonEmptyToolString(toolArgString(tc.Params, "query"), tc.Query),
		ID:        firstNonEmptyToolString(toolArgString(tc.Params, "id"), tc.ID),
		Inputs:    ecalcPairs(tc.Params["inputs"], "value"),
		Choices:   ecalcPairs(tc.Params["choices"], "option"),
		ESeries:   toolArgString(tc.Params, "eseries", "e_series", "series"),
		Limit:     firstNonEmptyInt(toolArgInt(tc.Params, 0, "limit"), tc.Limit),
	}
}

// ecalcPairs reads {name, <valueKey>} pairs. The schema asks for an array of objects; a JSON object of name to
// value pairs and a JSON text of either are accepted too, because models send them. Numbers are taken as typed.
func ecalcPairs(raw interface{}, valueKey string) []ecalc.NamedValue {
	if text, ok := raw.(string); ok {
		var decoded interface{}
		if json.Unmarshal([]byte(text), &decoded) != nil {
			return nil
		}
		raw = decoded
	}
	var out []ecalc.NamedValue
	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			name := strings.TrimSpace(toolArgString(m, "name", "id", "var"))
			if name == "" {
				continue
			}
			value, _ := ecalcScalarText(m[valueKey])
			if value == "" {
				value, _ = ecalcScalarText(m["value"])
			}
			out = append(out, ecalc.NamedValue{Name: name, Value: value})
		}
	case map[string]interface{}:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if value, ok := ecalcScalarText(v[k]); ok {
				out = append(out, ecalc.NamedValue{Name: k, Value: value})
			}
		}
	}
	return out
}

// ecalcScalarText is the text of a string or a number.
func ecalcScalarText(raw interface{}) (string, bool) {
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v), true
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64), true
	case json.Number:
		return v.String(), true
	case int:
		return strconv.Itoa(v), true
	case int64:
		return strconv.FormatInt(v, 10), true
	}
	return "", false
}
