package flows

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

// The enumerated parameters of doc.pdf_create are checked like if_exists: an unknown
// value is an error in Validate and in Execute, spelling variants are normalised,
// and the output name is bounded.
func TestPDFCreateChoicesAndName(t *testing.T) {
	env := StaticEnv{"document_creator": {}, GotenbergTool: {}}
	def := lookupDef(t, docRegistry(t, env), TypePDFCreate)
	run := func(params map[string]any) (*fakeTools, error) {
		tools := &fakeTools{respond: docOK}
		params["content"] = "c"
		_, err := execDef(def, params, &Services{Tools: tools})
		return tools, err
	}
	for param, values := range map[string][]any{
		"format":     {"pdf", "rtf", "markdown!", 5.0, true, []any{"html"}},
		"paper_size": {"A6", "big", 4.0, true, []any{"A4"}},
	} {
		for _, v := range values {
			tools, err := run(map[string]any{param: v})
			if ne := asNodeError(err); ne == nil || ne.Code != "FLOW_PARAM_INVALID" || tools.count() != 0 || !strings.HasPrefix(ne.Message, param) {
				t.Errorf("%s=%#v: error %v after %d calls", param, v, err, tools.count())
			}
			node := &Node{ID: testNodeID(1), Params: map[string]any{param: v}}
			if issues := def.Validate(node, ValidateContext{}); len(issues) != 1 || issues[0].Code != IssueParamInvalid || issues[0].Param != param {
				t.Errorf("%s=%#v: issues = %+v", param, v, issues)
			}
		}
	}
	tools, err := run(map[string]any{"format": " Markdown ", "paper_size": "letter"})
	if args := tools.last(t).Args; err != nil || args["operation"] != "markdown_to_pdf" || args["paper_size"] != "Letter" {
		t.Errorf("spelling variants: %v, args %#v", err, args)
	}
	tools, err = run(map[string]any{"paper_size": nil, "format": nil})
	if args := tools.last(t).Args; err != nil || args["operation"] != "create_pdf" || args["paper_size"] != nil {
		t.Errorf("null choices: %v, args %#v", err, args)
	}
	// A template cannot be judged before the run.
	node := &Node{ID: testNodeID(1), Params: map[string]any{"format": "{{trigger.data.f}}", "paper_size": "{{x.y}}"}}
	if issues := def.Validate(node, ValidateContext{}); len(issues) != 0 {
		t.Errorf("templates: %+v", issues)
	}
	// Markdown without Gotenberg stays one issue, whatever the spelling.
	noGotenberg := lookupDef(t, docRegistry(t, StaticEnv{"document_creator": {}}), TypePDFCreate)
	node = &Node{ID: testNodeID(1), Params: map[string]any{"format": "HTML "}}
	if issues := noGotenberg.Validate(node, ValidateContext{}); len(issues) != 1 || issues[0].Param != "format" {
		t.Errorf("html without Gotenberg: %+v", issues)
	}

	long := strings.Repeat("n", maxFileNameBytes)
	for name, c := range map[string]struct {
		v    any
		want any // the "filename" argument; nil: not sent
		fail bool
	}{
		"plain": {"report", "report", false}, "pdf suffix": {"report.pdf", "report", false}, "blanks": {" r.pdf ", "r", false},
		"number": {2024.0, "2024", false}, "only suffix": {".pdf", nil, false}, "null": {nil, nil, false}, "blank": {"  ", nil, false},
		"limit": {long, long, false}, "too long": {long + "n", nil, true}, "huge": {strings.Repeat("n", 1<<20), nil, true},
		"NUL": {"a\x00b", nil, true}, "invalid UTF-8": {"a\xff", nil, true},
		"object": {map[string]any{"a": 1.0}, nil, true}, "list": {[]any{"a"}, nil, true},
	} {
		tools, err := run(map[string]any{"filename": c.v})
		ne := asNodeError(err)
		if c.fail {
			if ne == nil || ne.Code != "FLOW_PARAM_INVALID" || tools.count() != 0 || len(ne.Message) > 120 || strings.Contains(ne.Message, "nnnn") {
				t.Errorf("filename %s: error %+v after %d calls", name, ne, tools.count())
			}
		} else if err != nil || tools.last(t).Args["filename"] != c.want {
			t.Errorf("filename %s: %v, sent %#v, want %#v", name, err, tools.last(t).Args["filename"], c.want)
		}
	}
}

// A successful read always carries text: a tool answer without it is an error, not an
// empty document that a flow would go on with. The extractor's summary mode answers in
// the same {"status":"success","content":…} envelope as a plain extraction.
func TestPDFAndFileReadNeedText(t *testing.T) {
	reg := docRegistry(t, nil)
	pdf, file := lookupDef(t, reg, TypePDFRead), lookupDef(t, reg, TypeFileRead)
	read := func(def *NodeDef, resp ToolResponse) (ExecResult, error) {
		tools := &fakeTools{respond: func(ToolRequest) (ToolResponse, error) { return resp, nil }}
		return execDef(def, map[string]any{"file": "a.pdf", "path": "a.txt"}, &Services{Tools: tools})
	}
	ok := func(out string) ToolResponse { return ToolResponse{Output: out, Status: "success"} }
	for _, c := range []struct {
		name string
		def  *NodeDef
		resp ToolResponse
		key  string
		want string // "": must fail
	}{
		{"pdf content", pdf, ok(`{"status":"success","content":"Seite 1"}`), "text", "Seite 1"},
		{"pdf empty content", pdf, ok(`{"status":"success","content":""}`), "text", ""},
		{"pdf summary mode", pdf, ok("{\"status\":\"success\",\"content\":\"<external_data>\\nA short summary.\\n</external_data>\"}"), "text", "A short summary."},
		{"pdf plain text is no summary", pdf, ok("A short summary."), "text", "\x00"},
		{"pdf text field is no content", pdf, ok(`{"status":"success","text":"A short summary."}`), "text", "\x00"},
		{"pdf no text", pdf, ok(`{"status":"success"}`), "text", "\x00"},
		{"pdf content object", pdf, ok(`{"status":"success","content":{"a":1}}`), "text", "\x00"},
		{"file text", file, ok(`{"status":"success","data":"Hallo"}`), "content", "Hallo"},
		{"file empty", file, ok(`{"status":"success","data":""}`), "content", ""},
		{"file object", file, ok(`{"status":"success","data":{"content":"Inhalt"}}`), "content", "Inhalt"},
		{"file no data", file, ok(`{"status":"success"}`), "content", "\x00"},
		{"file data without content", file, ok(`{"status":"success","data":{"lines":[]}}`), "content", "\x00"},
		{"file number", file, ok(`{"status":"success","data":5}`), "content", "\x00"},
	} {
		res, err := read(c.def, c.resp)
		if c.want == "\x00" {
			if ne := asNodeError(err); ne == nil || ne.Code != "FLOW_TOOL_ERROR" {
				t.Errorf("%s: %#v, %v; want FLOW_TOOL_ERROR", c.name, res.Output, err)
			}
		} else if err != nil || res.Output[c.key] != c.want {
			t.Errorf("%s: %#v, %v; want %q", c.name, res.Output, err, c.want)
		}
	}
	// A binary file is the tool's error, kept as it is.
	bin := ToolResponse{Output: `{"status":"error","message":"'x.png' appears to be a binary file","data":{"kind":"binary"}}`, IsError: true, Status: "failed"}
	if _, err := read(file, bin); asNodeError(err).Code != "FLOW_TOOL_ERROR" || !strings.Contains(err.Error(), "binary file") {
		t.Errorf("binary file: %v", err)
	}
}

// The lint follows only flags: every parameter that names a file or becomes the
// content of one is a sink, and what reads files is untrusted.
func TestDocNodeTaintFlags(t *testing.T) {
	reg := docRegistry(t, nil)
	sinks := map[string]map[string]bool{
		TypeFileWrite: {"path": true, "content": true}, TypeFileRead: {"path": true},
		TypePDFRead: {"file": true}, TypePDFCreate: {"filename": true},
	}
	untrusted := map[string]bool{TypeFileRead: true, TypePDFRead: true}
	effects := map[string][]Effect{TypeFileWrite: {EffectWritesFiles}, TypePDFCreate: {EffectWritesFiles}}
	defs := reg.All()
	if len(defs) != 4 {
		t.Fatalf("%d document node types", len(defs))
	}
	for _, def := range defs {
		for _, p := range def.Params {
			if p.SensitiveSink != sinks[def.Type][p.Name] {
				t.Errorf("%s.%s: SensitiveSink = %v", def.Type, p.Name, p.SensitiveSink)
			}
		}
		for name := range sinks[def.Type] {
			found := false
			for _, p := range def.Params {
				found = found || p.Name == name
			}
			if !found {
				t.Errorf("%s has no parameter %s", def.Type, name)
			}
		}
		if def.UntrustedOutput != untrusted[def.Type] {
			t.Errorf("%s: UntrustedOutput = %v", def.Type, def.UntrustedOutput)
		}
		if !reflect.DeepEqual(def.Effects, effects[def.Type]) {
			t.Errorf("%s: effects = %v", def.Type, def.Effects)
		}
		if def.Category != "documents" || def.Tool == "" || def.LabelKey == "" || def.AvailabilityFunc == nil || def.Execute == nil {
			t.Errorf("%s: incomplete definition %+v", def.Type, def)
		}
	}
}

// End to end: data from an untrusted trigger or from a node that reads files warns
// where it reaches a file path or file content.
func TestLintFlagsUntrustedDataInFileNodes(t *testing.T) {
	reg := triggerRegistry(t)
	if err := registerDocNodes(reg, nil); err != nil {
		t.Fatal(err)
	}
	params := func(f *Flow, nodeID string) string {
		var got []string
		for _, is := range LintUntrustedData(f, reg) {
			if is.NodeID == nodeID {
				got = append(got, is.Param)
			}
		}
		sort.Strings(got)
		return strings.Join(got, ",")
	}
	for _, c := range []struct {
		name  string
		build func(b *flowBuilder) string // returns the id of the node under test
		want  string
	}{
		{"webhook into write", func(b *flowBuilder) string {
			hook := b.node("hook", TypeTriggerWebhook, nil)
			w := b.node("write", TypeFileWrite, map[string]any{"path": "{{trigger.data.payload.name}}", "content": "{{trigger.data.raw}}"})
			b.edge(hook, PortOut, w)
			return w
		}, "content,path"},
		{"webhook into read", func(b *flowBuilder) string {
			hook := b.node("hook", TypeTriggerWebhook, nil)
			r := b.node("read", TypeFileRead, map[string]any{"path": "{{trigger.data.payload.p}}"})
			b.edge(hook, PortOut, r)
			return r
		}, "path"},
		{"file content into write", func(b *flowBuilder) string {
			m := b.node("start", TypeTriggerManual, nil)
			r := b.node("read", TypeFileRead, map[string]any{"path": "in.txt"})
			w := b.node("write", TypeFileWrite, map[string]any{"path": "out.txt", "content": "{{read.content}}"})
			b.edge(m, PortOut, r)
			b.edge(r, PortOut, w)
			return w
		}, "content"},
		{"pdf text into a pdf name", func(b *flowBuilder) string {
			m := b.node("start", TypeTriggerManual, nil)
			p := b.node("pdf", TypePDFRead, map[string]any{"file": "a.pdf"})
			c := b.node("make", TypePDFCreate, map[string]any{"content": "{{pdf.text}}", "filename": "{{pdf.text}}"})
			b.edge(m, PortOut, p)
			b.edge(p, PortOut, c)
			return c
		}, "filename"},
		{"pdf text into a pdf file", func(b *flowBuilder) string {
			m := b.node("start", TypeTriggerManual, nil)
			p := b.node("pdf", TypePDFRead, map[string]any{"file": "a.pdf"})
			c := b.node("make", TypePDFCreate, map[string]any{"content": "{{pdf.text}}"})
			b.edge(m, PortOut, p)
			b.edge(p, PortOut, c)
			return c
		}, ""},
		{"manual trigger is trusted", func(b *flowBuilder) string {
			m := b.node("start", TypeTriggerManual, nil)
			w := b.node("write", TypeFileWrite, map[string]any{"path": "{{trigger.data.p}}", "content": "x"})
			b.edge(m, PortOut, w)
			return w
		}, ""},
	} {
		b := newFlow(c.name)
		id := c.build(b)
		if got := params(b.build(), id); got != c.want {
			t.Errorf("%s: warned parameters %q, want %q", c.name, got, c.want)
		}
	}
}

// Validate and Execute see raw parameters of any type. No hook may panic or produce
// unbounded text, Validate and Execute must agree on the enumerated parameters, and
// no combination may reach a tool with a malformed request.
func TestDocHooksSurviveOddParams(t *testing.T) {
	env := StaticEnv{"document_creator": {}, "filesystem": {}, PDFExtractorTool: {}, GotenbergTool: {}}
	reg := docRegistry(t, env)
	vc := ValidateContext{Mode: ModePublish}
	choiceParams := map[string]bool{"format": true, "paper_size": true, "if_exists": true}
	bases := map[string]map[string]any{
		TypePDFCreate: {"content": "x"}, TypePDFRead: {"file": "a.pdf"},
		TypeFileRead: {"path": "a.txt"}, TypeFileWrite: {"path": "a.txt", "content": "x"},
	}
	var failures []string
	fail := func(format string, args ...any) { failures = append(failures, fmt.Sprintf(format, args...)) }
	runs := 0

	check := func(def *NodeDef, params map[string]any, label string) {
		runs++
		node := &Node{ID: testNodeID(1), Key: "doc", Type: def.Type, Params: params}
		tools := &fakeTools{respond: docOK}
		var (
			issues  []Issue
			res     ExecResult
			execErr error
		)
		for _, step := range []struct {
			name string
			fn   func()
		}{
			{"Validate", func() {
				if def.Validate != nil {
					issues = def.Validate(node, vc)
				}
			}},
			{"Availability", func() { _ = def.Availability() }},
			{"FieldsOf", func() { _ = def.FieldsOf(node) }},
			{"OutputPorts", func() { _ = def.OutputPorts(node) }},
			{"EffectsOf", func() { _ = def.EffectsOf(node) }},
			{"Execute", func() { res, execErr = execDef(def, params, &Services{Tools: tools}) }},
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
		flagged := map[string]bool{}
		if len(issues) > 4 {
			fail("%s: %d issues", label, len(issues))
		}
		for _, is := range issues {
			flagged[is.Param] = true
			if is.Code != IssueParamInvalid || is.Severity != SeverityError || is.NodeID != node.ID || !declared[is.Param] ||
				len(is.Message) > maxEchoMessageBytes || !utf8.ValidString(is.Message) || strings.ContainsAny(is.Message, "\n\r\x00") {
				fail("%s: bad issue %.200q", label, fmt.Sprintf("%+v", is))
			}
		}
		if execErr != nil {
			var ne *NodeError
			if !errors.As(execErr, &ne) || ne == nil || ne.Code == "" || len(ne.Message) > maxEchoMessageBytes ||
				!utf8.ValidString(ne.Message) || strings.ContainsAny(ne.Message, "\n\r\x00") {
				fail("%s: bad Execute error %T %.200q", label, execErr, execErr.Error())
				return
			}
			if ne.Code == "FLOW_PARAM_INVALID" && tools.count() != 0 {
				fail("%s: rejected the parameters after %d tool calls", label, tools.count())
			}
			if ne.Code == "FLOW_PARAM_INVALID" {
				for name := range choiceParams {
					if strings.HasPrefix(ne.Message, name+" must be") && !flagged[name] {
						fail("%s: Execute rejected %s (%s) but Validate reported nothing", label, name, ne.Message)
					}
				}
			}
		} else if raw, err := json.Marshal(res.Output); err != nil || !utf8.Valid(raw) || len(res.Output) == 0 {
			fail("%s: output does not encode: %v", label, err)
		}
		for name := range choiceParams {
			if flagged[name] && execErr == nil {
				fail("%s: Validate rejected %s but Execute succeeded", label, name)
			}
		}
		for _, c := range tools.allCalls() {
			if _, err := json.Marshal(c.Args); err != nil || c.Tool != def.Tool || len(c.AllowedTools) != 1 || c.AllowedTools[0] != def.Tool {
				fail("%s: malformed tool request %v: %+v", label, err, c.Tool)
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
	if runs < 800 {
		t.Fatalf("only %d combinations ran", runs)
	}
	if len(failures) > 0 {
		if len(failures) > 15 {
			failures = append(failures[:15], fmt.Sprintf("... and %d more", len(failures)-15))
		}
		t.Fatalf("%d of %d combinations failed:\n%s", len(failures), runs, strings.Join(failures, "\n"))
	}
}
