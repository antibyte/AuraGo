package agent

import (
	"encoding/json"
	"reflect"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

// code and value are text in ToolCall, but a model may send a number or an array (a colour code, a reading). The call must
// survive that and the typed field must hold the text, while the flat Params map keeps what was sent: the remote
// desktop input forwards Params as they are, and its key code is an integer.
func TestToolCallCodeAndValueTakeNumbersAndArraysAndParamsKeepTheirType(t *testing.T) {
	for name, c := range map[string]struct {
		json       string
		code, text string
		params     interface{}
		field      string
	}{
		"integer code":     {`{"action":"remote_control","operation":"desktop_input","kind":"key_down","key":"enter","code":13}`, "13", "13", float64(13), "code"},
		"decimal value":    {`{"action":"ecalc","operation":"encode","value":4.7}`, "4.7", "4.7", float64(4.7), "value"},
		"integer value":    {`{"action":"ecalc","operation":"encode","value":4700}`, "4700", "4700", float64(4700), "value"},
		"array code":       {`{"action":"ecalc","operation":"decode","code":["brown","black","red"]}`, "brown, black, red", "brown, black, red", []interface{}{"brown", "black", "red"}, "code"},
		"array of numbers": {`{"action":"ecalc","operation":"decode","code":[4,7,2]}`, "4, 7, 2", "4, 7, 2", []interface{}{float64(4), float64(7), float64(2)}, "code"},
		"boolean value":    {`{"action":"x","value":true}`, "true", "true", true, "value"},
		"text code":        {`{"action":"execute_python","code":"print(1)"}`, "print(1)", "print(1)", "print(1)", "code"},
		"text value":       {`{"action":"x","value":"4k7"}`, "4k7", "4k7", "4k7", "value"},
	} {
		var tc ToolCall
		if err := json.Unmarshal([]byte(c.json), &tc); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		field := tc.Code
		if c.field == "value" {
			field = tc.Value
		}
		if field != c.text {
			t.Errorf("%s: typed %s = %q, want %q", name, c.field, field, c.text)
		}
		if got := tc.Params[c.field]; !reflect.DeepEqual(got, c.params) {
			t.Errorf("%s: Params[%s] = %#v, want %#v", name, c.field, got, c.params)
		}
	}
	// The same through the native call path: the arguments are not malformed and the integer reaches the device as one.
	tc := NativeToolCallToToolCall(openai.ToolCall{ID: "call-1", Type: "function", Function: openai.FunctionCall{Name: "remote_control", Arguments: `{"operation":"desktop_input","kind":"key_down","key":"enter","code":13}`}}, nil)
	if tc.NativeArgsMalformed || tc.Code != "13" || tc.Params["code"] != float64(13) {
		t.Errorf("native desktop_input key code: malformed %v (%s), Code %q, Params code %#v", tc.NativeArgsMalformed, tc.NativeArgsError, tc.Code, tc.Params["code"])
	}
	if forwarded := copyRemoteDesktopParams(tc.Params); forwarded["code"] != float64(13) {
		t.Errorf("the device must receive the integer code: %#v", forwarded)
	}
}
