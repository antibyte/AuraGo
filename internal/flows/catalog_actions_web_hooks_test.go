package flows

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// webAnswers answers each web tool with a success of its own shape.
func webAnswers(req ToolRequest) (ToolResponse, error) {
	switch req.Tool {
	case DDGSearchTool, BraveSearchTool:
		return ToolResponse{Output: searchOK, Status: "success"}, nil
	case "web_scraper":
		return ToolResponse{Output: `{"status":"success","title":"T","content":"c","items":[{"title":"a"}]}`, Status: "success"}, nil
	}
	return apiAnswer(200, `{"a":1}`), nil
}

// Only a GET is free of outward effects. A method that is not known yet (a
// template) or has no meaning counts as one that may change something, and the hook
// copes with a nil node and any parameter.
func TestHTTPRequestEffects(t *testing.T) {
	def := httpNodeDef(t)
	sends := []Effect{EffectSendsMessage}
	for name, c := range map[string]struct {
		method any
		want   []Effect
	}{
		"absent": {nil, nil}, "empty": {"", nil}, "blank": {"  ", nil}, "GET": {"GET", nil}, "lower": {"get", nil}, "padded": {" Get\n", nil},
		"POST": {"POST", sends}, "post": {"post", sends}, "PUT": {"PUT", sends}, "PATCH": {"PATCH", sends}, "DELETE": {"delete", sends},
		"template": {"{{trigger.data.method}}", sends}, "unknown": {"HEAD", sends}, "number": {5.0, sends}, "flag": {true, sends},
		"list": {[]any{"GET"}, sends}, "object": {map[string]any{}, sends}, "huge": {strings.Repeat("G", 1<<20), sends},
	} {
		if got := def.EffectsOf(&Node{Params: map[string]any{"method": c.method}}); !reflect.DeepEqual(got, c.want) {
			t.Errorf("method %s: effects %v, want %v", name, got, c.want)
		}
	}
	if got := def.EffectsOf(&Node{}); got != nil {
		t.Errorf("a node without parameters: %v", got)
	}
	if got := def.EffectsOf(nil); got != nil {
		t.Errorf("a nil node: %v", got)
	}
	// The other web nodes have none.
	for _, typ := range []string{TypeWebSearch, TypeWebRead} {
		if got := lookupDef(t, webRegistry(t, nil), typ).EffectsOf(&Node{Params: map[string]any{"method": "POST"}}); len(got) != 0 {
			t.Errorf("%s: effects %v", typ, got)
		}
	}
}

// Validate reports what Execute would reject, and only what is known before the run:
// a template is not judged, and auth_header only counts when a secret is used.
func TestHTTPRequestValidate(t *testing.T) {
	def := httpNodeDef(t)
	check := func(params map[string]any, want ...string) {
		t.Helper()
		if _, has := params["url"]; !has {
			params["url"] = "https://api"
		}
		issues := def.Validate(&Node{ID: testNodeID(1), Params: params}, ValidateContext{Mode: ModePublish})
		var got []string
		for _, is := range issues {
			got = append(got, is.Param)
			if is.Code != IssueParamInvalid || is.Severity != SeverityError || strings.Contains(is.Message, "Evil") {
				t.Errorf("%v: bad issue %+v", params, is)
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%v: issues on %v, want %v", params, got, want)
		}
	}
	check(map[string]any{})
	check(map[string]any{"method": "FETCH"}, "method")
	check(map[string]any{"method": "{{trigger.data.m}}"})
	check(map[string]any{"method": " post "})
	check(map[string]any{"headers": map[string]any{"X-A": "a\r\nX-Evil: 1"}}, "headers")
	check(map[string]any{"headers": map[string]any{"X-A": "{{trigger.data.a}}", "X-B": "ok"}})
	check(map[string]any{"headers": "X-A: 1"}, "headers")
	check(map[string]any{"auth_secret": "tok", "auth_header": "Bad Name"}, "auth_header")
	check(map[string]any{"auth_header": "Bad Name"})
	check(map[string]any{"auth_secret": "tok", "auth_header": "{{trigger.data.h}}"})
	check(map[string]any{"auth_secret": "{{trigger.data.s}}", "auth_header": "X\r\nEvil"}, "auth_header")
	check(map[string]any{"auth_secret": "tok", "auth_header": " X-Key "})
	check(map[string]any{"auth_secret": "tok"})
	// The headers as a whole can be a template that resolves to the object.
	check(map[string]any{"headers": "{{trigger.data.h}}"})
	check(map[string]any{"headers": " {{trigger.data.h}} "})
	// A literal address that Execute always rejects; an absent one is the required check's.
	check(map[string]any{"url": "ftp://x"}, "url")
	check(map[string]any{"url": "example.com"}, "url")
	check(map[string]any{"url": "https://x/\r\nEvil"}, "url")
	check(map[string]any{"url": 5.0}, "url")
	check(map[string]any{"url": []any{"https://x"}}, "url")
	check(map[string]any{"url": "{{trigger.data.u}}"})
	check(map[string]any{"url": "https://x/{{trigger.data.p}}"})
	check(map[string]any{"url": ""})
	check(map[string]any{"url": nil})
	// auth_prefix counts when a secret is used, like auth_header.
	check(map[string]any{"auth_secret": "tok", "auth_prefix": "Bearer\r\nEvil: 1"}, "auth_prefix")
	check(map[string]any{"auth_secret": "tok", "auth_prefix": []any{"Bearer "}}, "auth_prefix")
	check(map[string]any{"auth_secret": "tok", "auth_prefix": "Token "})
	check(map[string]any{"auth_secret": "tok", "auth_prefix": ""})
	check(map[string]any{"auth_secret": "tok", "auth_prefix": "{{trigger.data.p}}"})
	check(map[string]any{"auth_prefix": "Bearer\r\nEvil: 1"})
	check(map[string]any{"fail_on_error": "maybe"}, "fail_on_error")
	check(map[string]any{"method": "FETCH", "headers": []any{"x"}, "auth_secret": "tok", "auth_header": ":"}, "method", "headers", "auth_header")
	if issues := def.Validate(nil, ValidateContext{}); issues != nil {
		t.Errorf("nil node: %+v", issues)
	}
}

// Validate and Execute see raw parameters of any type. No hook may panic or produce
// unbounded text, everything Validate rejects Execute must reject too, a parameter
// problem is found before the tool is called, and what reaches a tool is well
// formed: in particular no header with a line break.
func TestWebHooksSurviveOddParams(t *testing.T) {
	env := StaticEnv{DDGSearchTool: {}, "web_scraper": {}, "api_request": {}}
	reg := webRegistry(t, env)
	vc := ValidateContext{Mode: ModePublish}
	bases := map[string]map[string]any{
		TypeWebSearch:   {"query": "q"},
		TypeWebRead:     {"url": "https://x"},
		TypeHTTPRequest: {"url": "https://x", "method": "POST", "body": "b"},
	}
	var failures []string
	fail := func(format string, args ...any) { failures = append(failures, fmt.Sprintf(format, args...)) }
	runs := 0

	check := func(def *NodeDef, params map[string]any, label string) {
		runs++
		node := &Node{ID: testNodeID(1), Key: "web", Type: def.Type, Params: params}
		tools := &fakeTools{respond: webAnswers}
		svc := &Services{Tools: tools, Secrets: fakeSecrets{"x": "s3cr3t-x", "7": "s3cr3t-7"}}
		var (
			issues  []Issue
			res     ExecResult
			execErr error
		)
		for _, step := range []struct {
			name string
			fn   func()
		}{
			{"Validate", func() { issues = def.Validate(node, vc) }},
			{"Availability", func() { _ = def.Availability() }},
			{"FieldsOf", func() { _ = def.FieldsOf(node) }},
			{"OutputPorts", func() { _ = def.OutputPorts(node) }},
			{"EffectsOf", func() { _ = def.EffectsOf(node) }},
			{"Execute", func() { res, execErr = execDef(def, params, svc) }},
		} {
			if p := catchPanic(step.fn); p != nil {
				fail("%s: %s panicked: %v", label, step.name, p)
				return
			}
		}
		declared := map[string]bool{}
		for _, p := range def.Params {
			declared[p.Name] = true
		}
		clean := func(msg string) bool {
			return len(msg) <= maxEchoMessageBytes && utf8.ValidString(msg) && !strings.ContainsAny(msg, "\n\r\x00")
		}
		if len(issues) > 6 {
			fail("%s: %d issues", label, len(issues))
		}
		for _, is := range issues {
			if is.Code != IssueParamInvalid || is.Severity != SeverityError || is.NodeID != node.ID || !declared[is.Param] || !clean(is.Message) {
				fail("%s: bad issue %.200q", label, fmt.Sprintf("%+v", is))
			}
			// fail_on_error is the one flag Validate is stricter about than Execute: an
			// unreadable value keeps the default (fail) at run time, it is not rejected.
			if is.Param != "fail_on_error" && execErr == nil {
				fail("%s: Validate rejected %s but Execute succeeded", label, is.Param)
			}
		}
		if execErr != nil {
			ne := asNodeError(execErr)
			if ne == nil || ne.Code == "" || !clean(ne.Message) {
				fail("%s: bad Execute error %T %.200q", label, execErr, execErr.Error())
				return
			}
			if ne.Code == "FLOW_PARAM_INVALID" && tools.count() != 0 {
				fail("%s: rejected the parameters after %d tool calls", label, tools.count())
			}
		} else if raw, err := json.Marshal(res.Output); err != nil || !utf8.Valid(raw) || len(res.Output) == 0 || strings.Contains(string(raw), "s3cr3t") {
			fail("%s: bad output: %v %.100s", label, err, raw)
		}
		for _, c := range tools.allCalls() {
			if _, err := json.Marshal(c.Args); err != nil || len(c.AllowedTools) != 1 || c.AllowedTools[0] != c.Tool || c.Tool != def.Tool {
				fail("%s: malformed tool request %v: %+v", label, err, c.Tool)
			}
			headers, _ := c.Args["headers"].(map[string]any)
			for name, v := range headers {
				if value, isText := v.(string); !validHeaderName(name) || !isText || !validHeaderValue(value) {
					fail("%s: a malformed header reached the tool: %.60q", label, name)
				}
			}
			if u, isText := c.Args["url"].(string); c.Args["url"] != nil && (!isText || !hasWebScheme(u) || !validHeaderValue(u)) {
				fail("%s: a malformed address reached the tool: %.60q", label, u)
			}
		}
	}

	values := oddParamValues()
	for _, def := range reg.All() {
		base := bases[def.Type]
		names := []string{"unknown_param"}
		for _, p := range def.Params {
			names = append(names, p.Name)
		}
		for _, ov := range values {
			for _, name := range names {
				params := make(map[string]any, len(base)+1)
				for k, v := range base {
					params[k] = v
				}
				params[name] = ov.v
				check(def, params, fmt.Sprintf("%s %s=%s", def.Type, name, ov.name))
			}
			all := make(map[string]any, len(names))
			for _, name := range names {
				all[name] = ov.v
			}
			check(def, all, fmt.Sprintf("%s all=%s", def.Type, ov.name))
			// Odd header rows next to a credential: the vault secret is read ("x").
			if def.Type == TypeHTTPRequest {
				check(def, map[string]any{"url": "https://x", "auth_secret": "x", "headers": map[string]any{"X-A": ov.v, "": ov.v}}, fmt.Sprintf("%s headers row=%s", def.Type, ov.name))
				check(def, map[string]any{"url": "https://x", "auth_secret": ov.v, "auth_header": ov.v, "auth_prefix": ov.v}, fmt.Sprintf("%s credential=%s", def.Type, ov.name))
			}
		}
		check(def, nil, def.Type+" nil params")
		check(def, map[string]any{}, def.Type+" empty params")
		check(def, base, def.Type+" plain params")

		// A nil node is no reason to panic.
		if p := catchPanic(func() {
			if def.Validate != nil {
				_ = def.Validate(nil, vc)
			}
			_ = def.FieldsOf(nil)
			_ = def.OutputPorts(nil)
			_ = def.EffectsOf(nil)
		}); p != nil {
			fail("%s: a hook panicked on a nil node: %v", def.Type, p)
		}
	}
	if runs < 600 {
		t.Fatalf("only %d combinations ran", runs)
	}
	if len(failures) > 0 {
		if len(failures) > 15 {
			failures = append(failures[:15], fmt.Sprintf("... and %d more", len(failures)-15))
		}
		t.Fatalf("%d of %d combinations failed:\n%s", len(failures), runs, strings.Join(failures, "\n"))
	}
}

// A flow whose headers are one template can be published: the validator cannot
// judge the object before the run, and the run accepts it once it is resolved.
func TestHTTPTemplatedHeadersCanBePublished(t *testing.T) {
	reg := triggerRegistry(t)
	if err := registerWebNodes(reg, StaticEnv{"api_request": {}}); err != nil {
		t.Fatal(err)
	}
	b := newFlow("Templated headers")
	tr := b.node("start", TypeTriggerManual, nil)
	call := b.node("call", TypeHTTPRequest, map[string]any{"url": "https://api", "headers": "{{trigger.data.h}}"})
	b.edge(tr, PortOut, call)
	f := b.build()
	issues := Validate(f, reg, ValidateContext{Mode: ModePublish, Now: time.Now()})
	if HasErrors(issues) {
		t.Fatalf("publishing is blocked: %+v", issues)
	}
	tools := &fakeTools{respond: answerWith(apiAnswer(200, "ok"))}
	res, _ := runWith(context.Background(), newTestEngine(reg, &Services{Tools: tools, Location: time.UTC}, 2), f,
		RunRequest{TriggerNode: tr, TriggerData: map[string]any{"h": map[string]any{"X-A": "1"}}})
	if res.Status != RunSuccess {
		t.Fatalf("status %s: %s", res.Status, res.ErrorMessage)
	}
	if got := tools.last(t).Args["headers"]; !reflect.DeepEqual(got, map[string]any{"X-A": "1"}) {
		t.Errorf("headers sent = %#v", got)
	}
	// What the template resolves to is still checked at run time.
	tools = &fakeTools{respond: answerWith(apiAnswer(200, "ok"))}
	res, _ = runWith(context.Background(), newTestEngine(reg, &Services{Tools: tools, Location: time.UTC}, 2), f,
		RunRequest{TriggerNode: tr, TriggerData: map[string]any{"h": map[string]any{"X-A": "1\r\nX-Evil: 1"}}})
	if res.Status == RunSuccess || res.ErrorCode != "FLOW_PARAM_INVALID" || tools.count() != 0 {
		t.Errorf("a bad resolved header: status %s code %s after %d calls", res.Status, res.ErrorCode, tools.count())
	}
}
