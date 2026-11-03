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
// text-mode calls (code, value, offset and limit are ToolCall fields of the same type). The variable values are called
// inputs on the wire because ToolCall already owns a "values" field for spreadsheets, which only decodes arrays of
// arrays.
//
// A model may send a number where the schema asks for text. Whether that reaches this decoder depends on the decoding of
// ToolCall, which fails on a type mismatch of the whole call: code and value are in toolCallStringFields (a number
// becomes its text, a list of words is joined with ", ", which the band parser accepts; Params keeps what was sent), and
// offset and limit are in numericStringToolFields (a numeric text becomes a number). The other texts (filter, tolerance,
// tempco, voltage, class) and the whole numbers (bands) are no ToolCall fields and are read from Params, a number as typed.
func decodeECalcArgs(tc ToolCall) tools.ECalcRequest {
	return tools.ECalcRequest{
		Operation: toolCallSnapshotOperation(tc), // the precedence of the action policy, so that both see the same operation
		Query:     firstNonEmptyToolString(toolArgString(tc.Params, "query"), tc.Query),
		ID:        firstNonEmptyToolString(toolArgString(tc.Params, "id"), tc.ID),
		Inputs:    ecalcPairs(tc.Params["inputs"], "value"),
		Choices:   ecalcPairs(tc.Params["choices"], "option"),
		ESeries:   toolArgString(tc.Params, "eseries", "e_series", "series"),
		Limit:     firstNonEmptyInt(toolArgInt(tc.Params, 0, "limit"), tc.Limit),

		Filter: ecalcParamText(tc.Params, "filter"),
		Offset: firstNonEmptyInt(toolArgInt(tc.Params, 0, "offset"), tc.Offset),
		Lookup: ecalcLookup(tc.Params["lookup"]),

		Kind:      strings.ToLower(strings.TrimSpace(toolArgString(tc.Params, "kind", "code_kind"))),
		Code:      firstNonEmptyToolString(ecalcParamText(tc.Params, "code"), tc.Code),
		Value:     firstNonEmptyToolString(ecalcParamText(tc.Params, "value"), tc.Value),
		Bands:     ecalcParamInt(tc.Params, "bands"),
		Tolerance: ecalcParamText(tc.Params, "tolerance"),
		TempCo:    ecalcParamText(tc.Params, "tempco", "temp_co"),
		Voltage:   ecalcParamText(tc.Params, "voltage"),
		Class:     ecalcParamText(tc.Params, "class"),
	}
}

// ecalcParamText is the text of the first of the keys that holds a string or a number.
func ecalcParamText(params map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if text, ok := ecalcScalarText(params[key]); ok && text != "" {
			return text
		}
	}
	return ""
}

// ecalcParamInt reads a whole number that may arrive as a number or as text ("5"); anything else is 0.
func ecalcParamInt(params map[string]interface{}, key string) int {
	if n := toolArgInt(params, 0, key); n != 0 {
		return n
	}
	if text, ok := ecalcScalarText(params[key]); ok {
		if n, err := strconv.Atoi(text); err == nil {
			return n
		}
	}
	return 0
}

// ecalcLookup reads the lookup object of a table call: an object, or the JSON text of one. A lookup that is present but
// has no usable shape becomes an empty lookup, so that the tool answers that it needs a value instead of ignoring it.
func ecalcLookup(raw interface{}) *ecalc.TableLookup {
	if raw == nil {
		return nil
	}
	if text, ok := raw.(string); ok {
		var decoded interface{}
		if json.Unmarshal([]byte(text), &decoded) == nil {
			raw = decoded
		}
	}
	m, _ := raw.(map[string]interface{})
	value, _ := ecalcScalarText(m["value"])
	return &ecalc.TableLookup{
		Column: strings.TrimSpace(toolArgString(m, "column")),
		Value:  value,
		Mode:   strings.ToLower(strings.TrimSpace(toolArgString(m, "mode"))),
		Output: strings.TrimSpace(toolArgString(m, "output")),
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
