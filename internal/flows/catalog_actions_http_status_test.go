package flows

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// The body is real JSON or text, bounded, and never silently wrong.
func TestHTTPRequestBody(t *testing.T) {
	ok := answerWith(apiAnswer(200, ""))
	sent := func(params map[string]any) (any, bool, error) {
		params["url"] = "https://api"
		_, tools, err := runHTTP(t, params, ok, nil)
		if err != nil {
			if tools.count() != 0 {
				t.Errorf("%v: %d calls", params, tools.count())
			}
			return nil, false, err
		}
		body, has := tools.last(t).Args["body"]
		return body, has, nil
	}
	for name, c := range map[string]struct {
		body any
		want any // nil: no body
	}{
		"text":             {`a=1&b=2`, `a=1&b=2`},
		"text as it is":    {"  spaced  \n", "  spaced  \n"},
		"object":           {map[string]any{"a": 1.0, "b": []any{"x", nil}}, `{"a":1,"b":["x",null]}`},
		"no HTML escaping": {map[string]any{"a": "<b>&"}, `{"a":"<b>&"}`},
		"list":             {[]any{1.0, 2.0}, `[1,2]`},
		"number":           {5.0, `5`},
		"flag":             {false, `false`},
		"null":             {nil, nil},
		"blank":            {"  ", nil},
		"empty object":     {map[string]any{}, nil},
		"empty list":       {[]any{}, nil},
		"list of null":     {[]any{nil}, `[null]`},
	} {
		got, has, err := sent(map[string]any{"method": "POST", "body": c.body})
		if err != nil || (c.want == nil) == has || (has && got != c.want) {
			t.Errorf("%s: body %#v (%v, sent %v), want %#v", name, got, err, has, c.want)
		}
	}
	// A GET has no body, so nothing about it is judged either.
	if _, has, err := sent(map[string]any{"body": "x"}); err != nil || has {
		t.Errorf("GET body: %v %v", has, err)
	}
	if _, has, err := sent(map[string]any{"body": make(chan int)}); err != nil || has {
		t.Errorf("GET with a stray body: %v %v", has, err)
	}
	for _, method := range []string{"PUT", "patch", "Delete"} {
		if got, _, err := sent(map[string]any{"method": method, "body": "x"}); err != nil || got != "x" {
			t.Errorf("%s: %v %v", method, got, err)
		}
	}

	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	for name, body := range map[string]any{
		"NaN": map[string]any{"a": math.NaN()}, "Inf": math.Inf(1), "func": func() {}, "channel": make(chan int), "cycle": cyclic,
		"nested NaN": []any{map[string]any{"a": []any{math.NaN()}}}, "invalid UTF-8": "ok\xff", "over the limit": strings.Repeat("b", maxHTTPBodyBytes+1),
		"object over the limit": map[string]any{"a": strings.Repeat("b", maxHTTPBodyBytes)},
	} {
		_, tools, err := runHTTP(t, map[string]any{"url": "https://api", "method": "POST", "body": body}, ok, nil)
		wantParamInvalid(t, name, tools, err, "bbbb")
	}
	if got, _, err := sent(map[string]any{"method": "POST", "body": strings.Repeat("b", maxHTTPBodyBytes)}); err != nil || len(got.(string)) != maxHTTPBodyBytes {
		t.Errorf("a body of exactly the limit: %v", err)
	}
}

// A status of 400 or more is the server's answer: FLOW_HTTP_STATUS, or the output
// when fail_on_error is off. Everything else the tool reports is a tool failure
// and never an HTTP status, whatever fail_on_error says, and whatever the answer
// claims to be must come with a status the node can read.
func TestHTTPStatusIsNotAToolFailure(t *testing.T) {
	text := func(s string) ToolResponse { return ToolResponse{Output: s, Status: "success"} }
	type result struct {
		code string // "": success
		ok   any
		sc   any
	}
	cases := []struct {
		name   string
		resp   ToolResponse
		params map[string]any
		want   result
	}{
		{"404", apiAnswer(404, "nope"), nil, result{code: "FLOW_HTTP_STATUS"}},
		{"500 tolerated", apiAnswer(500, "oops"), map[string]any{"fail_on_error": false}, result{ok: false, sc: 500.0}},
		{"399 is fine", apiAnswer(399, ""), nil, result{ok: true, sc: 399.0}},
		{"302 not followed", apiAnswer(302, ""), nil, result{ok: true, sc: 302.0}},
		{"204", apiAnswer(204, ""), nil, result{ok: true, sc: 204.0}},
		{"599", apiAnswer(599, ""), nil, result{code: "FLOW_HTTP_STATUS"}},
		{"null fail_on_error keeps the default", apiAnswer(503, ""), map[string]any{"fail_on_error": nil}, result{code: "FLOW_HTTP_STATUS"}},
		{"text false", apiAnswer(503, ""), map[string]any{"fail_on_error": "false"}, result{ok: false, sc: 503.0}},
		{"flag as text yes", apiAnswer(503, ""), map[string]any{"fail_on_error": "yes"}, result{code: "FLOW_HTTP_STATUS"}},
		{"error status with IsError", ToolResponse{Output: apiAnswer(404, "x").Output, IsError: true, Status: "failed"}, nil, result{code: "FLOW_HTTP_STATUS"}},
		{"error status tolerated with IsError", ToolResponse{Output: apiAnswer(404, "x").Output, IsError: true, Status: "failed"}, map[string]any{"fail_on_error": false}, result{ok: false, sc: 404.0}},
		{"SSRF block", text(`{"status":"error","message":"URL validation failed: access to internal address 127.0.0.1 is blocked (SSRF protection)"}`), map[string]any{"fail_on_error": false}, result{code: "FLOW_TOOL_ERROR"}},
		{"request failed", text(`{"status":"error","message":"Request failed: dial tcp: i/o timeout"}`), map[string]any{"fail_on_error": false}, result{code: "FLOW_TOOL_ERROR"}},
		{"error status 200", text(`{"status":"error","status_code":200,"message":"odd"}`), map[string]any{"fail_on_error": false}, result{code: "FLOW_TOOL_ERROR"}},
		{"status 99", text(`{"status":"error","status_code":99}`), map[string]any{"fail_on_error": false}, result{code: "FLOW_TOOL_ERROR"}},
		{"status 600", text(`{"status":"error","status_code":600}`), map[string]any{"fail_on_error": false}, result{code: "FLOW_TOOL_ERROR"}},
		{"status 404.5", text(`{"status":"error","status_code":404.5}`), map[string]any{"fail_on_error": false}, result{code: "FLOW_TOOL_ERROR"}},
		{"status text", text(`{"status":"error","status_code":"not found"}`), map[string]any{"fail_on_error": false}, result{code: "FLOW_TOOL_ERROR"}},
		{"success without a status code", text(`{"status":"success","body":"x"}`), nil, result{code: "FLOW_TOOL_ERROR"}},
		{"success with status 0", text(`{"status":"success","status_code":0,"body":"x"}`), nil, result{code: "FLOW_TOOL_ERROR"}},
		{"plain refusal", text("Tool Output: [PERMISSION DENIED] api_request is disabled in Danger Zone settings (agent.allow_network_requests: false)."), nil, result{code: "FLOW_TOOL_ERROR"}},
		{"plain refusal as error", ToolResponse{Output: "[PERMISSION DENIED] no", IsError: true, Status: "failed"}, map[string]any{"fail_on_error": false}, result{code: "FLOW_TOOL_ERROR"}},
		{"empty answer", text(""), nil, result{code: "FLOW_TOOL_ERROR"}},
		{"unknown status", text(`{"status":"pending","status_code":200}`), nil, result{code: "FLOW_TOOL_ERROR"}},
		{"denied wins over a 404", ToolResponse{Output: apiAnswer(404, "x").Output, Status: "denied"}, map[string]any{"fail_on_error": false}, result{code: "FLOW_TOOL_DENIED"}},
		{"policy denied", text(`{"status":"policy_denied","status_code":403,"message":"no"}`), map[string]any{"fail_on_error": false}, result{code: "FLOW_TOOL_DENIED"}},
		{"needs setup", ToolResponse{Output: `{"message":"x"}`, Status: "needs_setup"}, nil, result{code: "FLOW_NODE_UNAVAILABLE"}},
		{"upper case status", text(`{"status":"SUCCESS","status_code":201,"body":"{}"}`), nil, result{ok: true, sc: 201.0}},
	}
	for _, c := range cases {
		params := map[string]any{"url": "https://api"}
		for k, v := range c.params {
			params[k] = v
		}
		res, _, err := runHTTP(t, params, answerWith(c.resp), nil)
		ne := asNodeError(err)
		switch {
		case c.want.code != "":
			if ne == nil || ne.Code != c.want.code {
				t.Errorf("%s: error = %v, want %s", c.name, err, c.want.code)
			}
		case err != nil:
			t.Errorf("%s: %v", c.name, err)
		case res.Output["ok"] != c.want.ok || res.Output["status_code"] != c.want.sc:
			t.Errorf("%s: output = %#v", c.name, res.Output)
		}
		if ne != nil && c.want.code == "FLOW_HTTP_STATUS" && (strings.Contains(ne.Message, "oops") || !strings.Contains(ne.Message, "HTTP")) {
			t.Errorf("%s: message = %q", c.name, ne.Message)
		}
	}
}

// The parsed body is bounded in depth (a server must not be able to fail a node
// by nesting), a body without JSON has no json field, and the headers are an object.
func TestHTTPResponseShape(t *testing.T) {
	run := func(body string) map[string]any {
		res, _, err := runHTTP(t, map[string]any{"url": "https://api"}, answerWith(apiAnswer(200, body)), nil)
		if err != nil {
			t.Fatalf("%.30q: %v", body, err)
		}
		return res.Output
	}
	deep := func(n int) string { return strings.Repeat("[", n) + strings.Repeat("]", n) }
	for name, c := range map[string]struct {
		body string
		json bool
	}{
		"object": {`{"a":1}`, true}, "list": {`[1,2]`, true}, "null": {`null`, true}, "number": {`5`, true}, "text": {`not json`, false},
		"empty": {``, false}, "trailing text": {`{"a":1} x`, false}, "at the limit": {deep(maxJSONDepth), true},
		"too deep": {deep(maxJSONDepth + 2), false}, "very deep": {deep(9000), false},
	} {
		out := run(c.body)
		if _, has := out["json"]; has != c.json || out["body"] != c.body {
			t.Errorf("%s: json present = %v, body kept = %v", name, has, out["body"] == c.body)
		}
		if raw, err := json.Marshal(out); err != nil || len(raw) == 0 {
			t.Errorf("%s: output does not encode: %v", name, err)
		}
	}
	res, _, err := runHTTP(t, map[string]any{"url": "https://api"}, answerWith(ToolResponse{Output: `{"status":"success","status_code":200,"body":"x"}`, Status: "success"}), nil)
	if h, isObject := res.Output["headers"].(map[string]any); err != nil || !isObject || len(h) != 0 {
		t.Errorf("headers = %#v, %v", res.Output["headers"], err)
	}
}

// Parameters and inputs may share memory with run data: Execute reads them only,
// and the headers it sends are a map of its own.
func TestHTTPDoesNotChangeItsParameters(t *testing.T) {
	headers := map[string]any{"X-A": "1", "authorization": "Basic abc"}
	body := map[string]any{"list": []any{1.0, map[string]any{"k": "v"}}}
	params := map[string]any{"url": "https://api", "method": "POST", "headers": headers, "body": body, "auth_secret": "tok"}
	before, _ := json.Marshal(params)
	_, tools, err := runHTTP(t, params, answerWith(apiAnswer(200, "")), fakeSecrets{"tok": "s3cret"})
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(params)
	if string(before) != string(after) {
		t.Errorf("parameters changed:\n%s\n%s", before, after)
	}
	sent := tools.last(t).Args["headers"].(map[string]any)
	sent["X-A"] = "changed"
	if headers["X-A"] != "1" || len(headers) != 2 {
		t.Errorf("the request shares its headers with the parameters: %#v", headers)
	}
}
