package flows

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"unicode/utf8"
)

// docOK answers every tool call of the document nodes with a success that carries
// what each of them reads.
func docOK(ToolRequest) (ToolResponse, error) {
	return ToolResponse{Output: `{"status":"success","data":"x","content":"x","file_path":"/d/x.pdf","web_path":"/files/x.pdf","filename":"x.pdf"}`, Status: "success"}, nil
}

// fsError is a failed filesystem tool answer with the error code the tool sets.
func fsError(message, code string) ToolResponse {
	body := map[string]any{"status": "error", "message": message}
	if code != "" {
		body["data"] = map[string]any{"error_code": code}
	}
	raw, _ := json.Marshal(body)
	return ToolResponse{Output: string(raw), IsError: true, Status: "failed"}
}

// existsFake is a filesystem tool: stat finds the paths taken reports, write succeeds.
func existsFake(taken func(string) bool) *fakeTools {
	return &fakeTools{respond: func(req ToolRequest) (ToolResponse, error) {
		p := Stringify(req.Args["file_path"])
		switch req.Args["operation"] {
		case "stat":
			if taken(p) {
				return ToolResponse{Output: `{"status":"success","data":{"name":"x","size":1}}`, Status: "success"}, nil
			}
			return fsError("Failed to stat: statat "+p+": no such file or directory", "io_error"), nil
		case "write_file":
			return ToolResponse{Output: `{"status":"success","message":"Wrote 1 bytes"}`, Status: "success"}, nil
		}
		return ToolResponse{Output: "unexpected", IsError: true}, nil
	}}
}

// execDefCtx is execDef with the caller's context.
func execDefCtx(ctx context.Context, def *NodeDef, params map[string]any, svc *Services) (ExecResult, error) {
	node := &Node{ID: testNodeID(1), Key: "node", Type: def.Type, Params: params}
	return def.Execute(ctx, ExecInput{
		Node: node, Params: withDefaults(def, params), Services: svc,
		Run: RunInfo{ID: "run_test", FlowID: "flow_test", Mode: ModeTest},
	})
}

func opsOf(calls []ToolRequest, op string) []string {
	var paths []string
	for _, c := range calls {
		if c.Args["operation"] == op {
			paths = append(paths, Stringify(c.Args["file_path"]))
		}
	}
	return paths
}

// A file object is only a hint at a path: FilePath reads a text "path" and nothing
// else, whatever "$type" says, and never panics or renders another type as a path.
func TestFilePathHandlesOddValues(t *testing.T) {
	for _, ov := range oddParamValues() {
		var direct, inMap string
		if p := catchPanic(func() { direct = FilePath(ov.v); inMap = FilePath(map[string]any{"path": ov.v}) }); p != nil {
			t.Errorf("%s: panicked: %v", ov.name, p)
			continue
		}
		want := ""
		if s, isText := ov.v.(string); isText {
			want = strings.TrimSpace(s)
		}
		if direct != want || inMap != want {
			t.Errorf("%s: FilePath = %.30q / in a file object %.30q, want %.30q", ov.name, direct, inMap, want)
		}
	}
	for name, c := range map[string]struct {
		v    any
		want string
	}{
		"file object":          {map[string]any{"$type": "file", "path": " a/b.txt "}, "a/b.txt"},
		"foreign type":         {map[string]any{"$type": "upload", "path": "/etc/passwd"}, "/etc/passwd"},
		"no type":              {map[string]any{"path": "x"}, "x"},
		"no path":              {map[string]any{"$type": "file", "name": "x"}, ""},
		"numeric path":         {map[string]any{"$type": "file", "path": 5.0}, ""},
		"nested object":        {map[string]any{"path": map[string]any{"path": "x"}}, ""},
		"list path":            {map[string]any{"path": []any{"x"}}, ""},
		"upper case key":       {map[string]any{"PATH": "x"}, ""},
		"typed nil object":     {map[string]any(nil), ""},
		"blank":                {"  \t", ""},
		"list":                 {[]any{"x"}, ""},
		"json number":          {json.Number("1"), ""},
		"web path is not path": {map[string]any{"web_path": "/files/x.pdf"}, ""},
	} {
		if got := FilePath(c.v); got != c.want {
			t.Errorf("%s: FilePath = %q, want %q", name, got, c.want)
		}
	}
}

// The nodes pass a path on exactly as it is: the sandbox is the tool's job, and a
// forged file object gets no more and no less than a plain string would.
func TestSpoofedFileObjectsAreOnlyPathHints(t *testing.T) {
	reg := docRegistry(t, nil)
	forged := map[string]any{"$type": "something_else", "path": "../../etc/passwd", "name": "harmless.txt", "mime": "text/plain"}
	for _, c := range []struct {
		typ, param, tool, arg string
	}{
		{TypeFileRead, "path", "filesystem", "file_path"},
		{TypePDFRead, "file", PDFExtractorTool, "filepath"},
	} {
		tools := &fakeTools{respond: docOK}
		res, err := execDef(lookupDef(t, reg, c.typ), map[string]any{c.param: forged}, &Services{Tools: tools})
		if err != nil {
			t.Fatalf("%s: %v", c.typ, err)
		}
		if req := tools.last(t); tools.count() != 1 || req.Tool != c.tool || req.Args[c.arg] != "../../etc/passwd" {
			t.Errorf("%s: request = %+v after %d calls", c.typ, req, tools.count())
		}
		// The result describes the path, not the forged name or type.
		if file := res.Output["file"].(map[string]any); file["path"] != "../../etc/passwd" || file["name"] != "passwd" || file["$type"] != "file" {
			t.Errorf("%s: file = %#v", c.typ, file)
		}
	}
}

// Every node that takes a path bounds it: too long, a NUL byte and invalid UTF-8
// fail before any tool runs, and the message never echoes the path.
func TestDocPathParamsAreBounded(t *testing.T) {
	reg := docRegistry(t, nil)
	long := strings.Repeat("a", maxFilePathBytes)
	for _, c := range []struct{ typ, param string }{{TypeFileRead, "path"}, {TypeFileWrite, "path"}, {TypePDFRead, "file"}} {
		def := lookupDef(t, reg, c.typ)
		run := func(v any) (int, error) {
			tools := &fakeTools{respond: docOK}
			params := map[string]any{c.param: v}
			if c.typ == TypeFileWrite {
				params["content"] = "c"
			}
			_, err := execDef(def, params, &Services{Tools: tools})
			return tools.count(), err
		}
		if n, err := run(long); err != nil || n != 1 {
			t.Errorf("%s: a path of %d bytes: %d calls, %v", c.typ, len(long), n, err)
		}
		if n, err := run(map[string]any{"$type": "file", "path": long}); err != nil || n != 1 {
			t.Errorf("%s: a file object with a path of %d bytes: %d calls, %v", c.typ, len(long), n, err)
		}
		for name, v := range map[string]any{
			"too long": long + "a", "huge": strings.Repeat("a", 1<<20), "NUL": "a\x00b", "invalid UTF-8": "ab\xff",
			"long in object": map[string]any{"path": long + "a"},
			"nil":            nil, "empty": "", "blank": " \t", "number": 5.0, "list": []any{"a"},
			"object without path": map[string]any{"$type": "file"}, "numeric path": map[string]any{"path": 5.0},
			"nested": map[string]any{"path": map[string]any{"path": "a"}},
		} {
			n, err := run(v)
			ne := asNodeError(err)
			if ne == nil || ne.Code != "FLOW_PARAM_INVALID" || n != 0 || len(ne.Message) > 120 || strings.Contains(ne.Message, "aaaa") || !utf8.ValidString(ne.Message) {
				t.Errorf("%s: %s: %d calls, error %+v", c.typ, name, n, ne)
			}
		}
	}
}

// An unknown if_exists must never turn into an overwrite: a mistyped "fail" would
// destroy the file it was meant to protect. Validate and Execute agree.
func TestFileWriteRejectsUnknownIfExists(t *testing.T) {
	def := lookupDef(t, docRegistry(t, nil), TypeFileWrite)
	for _, v := range []any{"skip", "overwrite!", "never", 5.0, true, []any{"fail"}, map[string]any{}} {
		tools := existsFake(func(string) bool { return true })
		_, err := execDef(def, map[string]any{"path": "a.txt", "content": "x", "if_exists": v}, &Services{Tools: tools})
		if ne := asNodeError(err); ne == nil || ne.Code != "FLOW_PARAM_INVALID" || tools.count() != 0 {
			t.Errorf("%#v: error %v after %d calls", v, err, tools.count())
		}
		node := &Node{ID: testNodeID(1), Params: map[string]any{"if_exists": v}}
		if issues := def.Validate(node, ValidateContext{}); len(issues) != 1 || issues[0].Code != IssueParamInvalid || issues[0].Param != "if_exists" || issues[0].Severity != SeverityError {
			t.Errorf("%#v: issues = %+v", v, issues)
		}
	}
	// Case and blanks do not matter; a blank value means the default.
	for v, want := range map[any]string{"FAIL ": "FLOW_FILE_EXISTS", " Unique": "", "": "", nil: "", "Overwrite": ""} {
		tools := existsFake(func(p string) bool { return p == "a.txt" })
		_, err := execDef(def, map[string]any{"path": "a.txt", "content": "x", "if_exists": v}, &Services{Tools: tools})
		if got := asNodeError(err); (want == "") != (got == nil) || (got != nil && got.Code != want) {
			t.Errorf("%#v: error %v, want %q", v, err, want)
		}
		node := &Node{ID: testNodeID(1), Params: map[string]any{"if_exists": v}}
		if issues := def.Validate(node, ValidateContext{}); len(issues) != 0 {
			t.Errorf("%#v: issues = %+v", v, issues)
		}
	}
	// A template is only known at run time: Validate lets it pass, Execute judges the result.
	node := &Node{ID: testNodeID(1), Params: map[string]any{"if_exists": "{{trigger.data.mode}}"}}
	if issues := def.Validate(node, ValidateContext{}); len(issues) != 0 {
		t.Errorf("template: issues = %+v", issues)
	}
	if _, err := execDef(def, map[string]any{"path": "a.txt", "if_exists": "bogus"}, &Services{Tools: &fakeTools{respond: docOK}}); asNodeError(err).Code != "FLOW_PARAM_INVALID" {
		t.Errorf("resolved template: %v", err)
	}
	if issues := def.Validate(nil, ValidateContext{}); len(issues) != 0 {
		t.Errorf("nil node: %+v", issues)
	}
}

// The unique-name search stops at maxUniqueNames: one probe for the path and one
// per number from 2 to the maximum, then FLOW_FILE_EXISTS without a write.
func TestFileWriteUniqueIsBounded(t *testing.T) {
	def := lookupDef(t, docRegistry(t, nil), TypeFileWrite)
	params := map[string]any{"path": "notes/a.txt", "content": "x", "if_exists": "unique"}

	tools := existsFake(func(string) bool { return true })
	_, err := execDef(def, params, &Services{Tools: tools})
	ne := asNodeError(err)
	calls := tools.allCalls()
	stats := opsOf(calls, "stat")
	if ne == nil || ne.Code != "FLOW_FILE_EXISTS" || len(calls) != maxUniqueNames || len(stats) != maxUniqueNames {
		t.Fatalf("all names taken: error %v after %d calls (%d stats), want %d probes", err, len(calls), len(stats), maxUniqueNames)
	}
	if stats[0] != "notes/a.txt" || stats[1] != "notes/a (2).txt" || stats[len(stats)-1] != "notes/a (100).txt" || len(opsOf(calls, "write_file")) != 0 {
		t.Errorf("probes = %q ... %q, writes %d", stats[:2], stats[len(stats)-1], len(opsOf(calls, "write_file")))
	}
	if !strings.Contains(ne.Message, "100") || len(ne.Message) > maxEchoMessageBytes {
		t.Errorf("message = %q", ne.Message)
	}

	// The last number is usable.
	tools = existsFake(func(p string) bool { return p != "notes/a (100).txt" })
	res, err := execDef(def, params, &Services{Tools: tools})
	if err != nil || res.Output["file"].(map[string]any)["path"] != "notes/a (100).txt" {
		t.Fatalf("name 100 free: %#v, %v", res.Output, err)
	}
	if calls := tools.allCalls(); len(calls) != maxUniqueNames+1 || len(opsOf(calls, "write_file")) != 1 {
		t.Errorf("name 100 free: %d calls", len(calls))
	}

	// A path of the maximum length keeps the message short, and the probe still
	// reads the reason at the end of the tool's long message.
	long := strings.Repeat("d", maxFilePathBytes-4) + ".txt"
	tools = existsFake(func(string) bool { return true })
	_, err = execDef(def, map[string]any{"path": long, "content": "x", "if_exists": "unique"}, &Services{Tools: tools})
	if ne := asNodeError(err); ne == nil || ne.Code != "FLOW_FILE_EXISTS" || len(ne.Message) > maxEchoMessageBytes || !utf8.ValidString(ne.Message) {
		t.Errorf("long path, all taken: %v", err)
	}
	for _, mode := range []string{"unique", "fail"} {
		tools = existsFake(func(string) bool { return false })
		res, err = execDef(def, map[string]any{"path": long, "content": "x", "if_exists": mode}, &Services{Tools: tools})
		if err != nil || tools.count() != 2 || res.Output["file"].(map[string]any)["path"] != long {
			t.Errorf("long path, free, %s: %v after %d calls", mode, err, tools.count())
		}
	}
}

// The probe reads the end of the tool's own message, on the parsed output: the
// NodeError message is cut to 300 runes, which loses the reason after a long path.
// Anything it cannot call "missing" is an error, so "fail" and "unique" never write
// over a file because a stat went wrong.
func TestFileExistsProbe(t *testing.T) {
	longPath := strings.Repeat("deep/", 200) + "f.txt"
	bigMessage := "Failed to stat: statat " + strings.Repeat("é", 1<<20) + ": no such file or directory"
	cases := []struct {
		name   string
		resp   ToolResponse
		err    error
		exists bool
		code   string // the error code, "" for a clean answer
	}{
		{"found", ToolResponse{Output: `{"status":"success","data":{"name":"a"}}`, Status: "success"}, nil, true, ""},
		{"linux missing", fsError("Failed to stat: statat a.txt: no such file or directory", "io_error"), nil, false, ""},
		{"windows missing", fsError(`Failed to stat: CreateFile C:\w\a.txt: The system cannot find the file specified.`, "io_error"), nil, false, ""},
		{"windows missing dir", fsError(`Failed to stat: CreateFile C:\w\a.txt: The system cannot find the path specified.`, "io_error"), nil, false, ""},
		{"go not exist", fsError("Failed to stat: file does not exist", "io_error"), nil, false, ""},
		{"missing, reason past 300 runes", fsError("Failed to stat: statat "+longPath+strings.Repeat("x", 600)+": no such file or directory", "io_error"), nil, false, ""},
		{"missing, 1 MiB message", fsError(bigMessage, "io_error"), nil, false, ""},
		{"bare io_error", fsError("stat failed", "io_error"), nil, false, ""},
		{"missing, file named permission denied", fsError("Failed to stat: statat permission denied: no such file or directory", "io_error"), nil, false, ""},
		{"permission denied", fsError("Failed to stat: statat a.txt: permission denied", "io_error"), nil, false, "FLOW_TOOL_ERROR"},
		{"access denied", fsError(`Failed to stat: CreateFile C:\a.txt: Access is denied.`, "io_error"), nil, false, "FLOW_TOOL_ERROR"},
		{"not a directory", fsError("Failed to stat: statat a/b.txt: not a directory", "io_error"), nil, false, "FLOW_TOOL_ERROR"},
		{"i/o error", fsError("Failed to stat: statat a.txt: input/output error", "io_error"), nil, false, "FLOW_TOOL_ERROR"},
		{"denied, file named no such file", fsError("Failed to stat: statat no such file: permission denied", "io_error"), nil, false, "FLOW_TOOL_ERROR"},
		{"denied, reason past 300 runes", fsError("Failed to stat: statat "+strings.Repeat("p", 900)+": permission denied", "io_error"), nil, false, "FLOW_TOOL_ERROR"},
		{"no error code", fsError("boom", ""), nil, false, "FLOW_TOOL_ERROR"},
		{"other error code", fsError("no such file", "something_else"), nil, false, "FLOW_TOOL_ERROR"},
		{"path refused", fsError("path 'x' escapes the project root", "path_resolution_error"), nil, false, "FLOW_TOOL_DENIED"},
		{"read only", fsError("mounted read-only", "read_only_filesystem"), nil, false, "FLOW_TOOL_DENIED"},
		{"denied status", ToolResponse{Output: "no", IsError: true, Status: "denied"}, nil, false, "FLOW_TOOL_DENIED"},
		{"needs setup", ToolResponse{Output: "no", IsError: true, Status: "needs_setup"}, nil, false, "FLOW_NODE_UNAVAILABLE"},
		{"plain text reason", ToolResponse{Output: "ERROR: stat a.txt: no such file or directory", IsError: true}, nil, false, ""},
		{"plain text unknown", ToolResponse{Output: "ERROR: something broke", IsError: true}, nil, false, "FLOW_TOOL_ERROR"},
		{"transport error", ToolResponse{}, context.DeadlineExceeded, false, "FLOW_NODE_TIMEOUT"},
	}
	for _, c := range cases {
		tools := &fakeTools{respond: func(ToolRequest) (ToolResponse, error) { return c.resp, c.err }}
		exists, err := fileExists(context.Background(), toolInput(tools), "p")
		got := ""
		if err != nil {
			got = asNodeError(err).Code
		}
		if exists != c.exists || got != c.code {
			t.Errorf("%s: exists %v, error code %q (%v); want %v, %q", c.name, exists, got, err, c.exists, c.code)
		}
	}
}

// A run that ends between the probe and the write does not write: callTool fails on
// a cancelled context before it invokes the tool.
func TestFileWriteDoesNotWriteAfterContextEnds(t *testing.T) {
	def := lookupDef(t, docRegistry(t, nil), TypeFileWrite)
	ctx, cancel := context.WithCancel(context.Background())
	tools := &fakeTools{respond: func(req ToolRequest) (ToolResponse, error) {
		if req.Args["operation"] != "stat" {
			t.Errorf("%v ran after the context ended", req.Args["operation"])
			return docOK(req)
		}
		cancel() // the run ends while the probe is answering
		return fsError("Failed to stat: statat a.txt: no such file or directory", "io_error"), nil
	}}
	_, err := execDefCtx(ctx, def, map[string]any{"path": "a.txt", "content": "x", "if_exists": "fail"}, &Services{Tools: tools})
	if err == nil || tools.count() != 1 {
		t.Fatalf("error %v after %d calls, want the probe only", err, tools.count())
	}

	// Cancelled before the start: not even the probe runs.
	tools = &fakeTools{respond: docOK}
	for _, mode := range []string{"overwrite", "unique"} {
		if _, err := execDefCtx(ctx, def, map[string]any{"path": "a.txt", "content": "x", "if_exists": mode}, &Services{Tools: tools}); err == nil {
			t.Errorf("%s: no error on a cancelled context", mode)
		}
	}
	if tools.count() != 0 {
		t.Errorf("%d calls on a cancelled context", tools.count())
	}
}

// The tool's refusals of a path and of a read-only target cannot be fixed by a retry
// and become FLOW_TOOL_DENIED; other failures stay retryable. FLOW_FILE_EXISTS
// stays retryable too: nothing was written, so a retry costs only the delay, and
// changing the engine's set of final codes is not this node's decision.
func TestFileToolRefusalsAreFinalAndOtherFailuresRetried(t *testing.T) {
	reg := docRegistry(t, nil)
	for _, c := range []struct {
		resp  ToolResponse
		code  string
		final bool
	}{
		{fsError("path 'x' escapes the project root", "path_resolution_error"), "FLOW_TOOL_DENIED", true},
		{fsError("target is mounted read-only", "read_only_filesystem"), "FLOW_TOOL_DENIED", true},
		{fsError("Failed to read file: boom", "io_error"), "FLOW_TOOL_ERROR", false},
		{fsError("something", ""), "FLOW_TOOL_ERROR", false},
		{ToolResponse{Output: "no", IsError: true, Status: "denied"}, "FLOW_TOOL_DENIED", true},
	} {
		for _, typ := range []string{TypeFileRead, TypeFileWrite} {
			tools := &fakeTools{respond: func(ToolRequest) (ToolResponse, error) { return c.resp, nil }}
			_, err := execDef(lookupDef(t, reg, typ), map[string]any{"path": "a.txt", "content": "x"}, &Services{Tools: tools})
			ne := asNodeError(err)
			if ne == nil || ne.Code != c.code || hopeless(err) != c.final {
				t.Errorf("%s %q: error %+v, final %v; want %s, final %v", typ, c.resp.Output, ne, hopeless(err), c.code, c.final)
			}
		}
	}
	// A refused probe stops a unique write before it writes.
	tools := &fakeTools{respond: func(ToolRequest) (ToolResponse, error) {
		return fsError("path 'x' escapes the project root", "path_resolution_error"), nil
	}}
	_, err := execDef(lookupDef(t, reg, TypeFileWrite), map[string]any{"path": "../x", "if_exists": "unique"}, &Services{Tools: tools})
	if asNodeError(err).Code != "FLOW_TOOL_DENIED" || tools.count() != 1 {
		t.Errorf("refused probe: %v after %d calls", err, tools.count())
	}
	if hopeless(NewNodeError("FLOW_FILE_EXISTS", "x")) {
		t.Error("FLOW_FILE_EXISTS is retried")
	}
}

func TestNumberedPath(t *testing.T) {
	for in, want := range map[string]string{
		"notes/a.txt":      "notes/a (2).txt",
		"a.txt":            "a (2).txt",
		"a":                "a (2)",
		"a.tar.gz":         "a.tar (2).gz",
		"dir.v2/file":      "dir.v2/file (2)",
		`C:\a.b\file`:      `C:\a.b\file (2)`,
		`C:\a.b\file.txt`:  `C:\a.b\file (2).txt`,
		".env":             ".env (2)",
		"conf/.env":        "conf/.env (2)",
		".env.local":       ".env (2).local",
		"notes/":           "notes/ (2)",
		"":                 " (2)",
		"ü/ä.txt":          "ü/ä (2).txt",
		"a.txt.":           "a.txt (2).",
		"a b/c d.e f.TXT ": "a b/c d.e f (2).TXT ",
	} {
		if got := numberedPath(in, 2); got != want {
			t.Errorf("numberedPath(%q, 2) = %q, want %q", in, got, want)
		}
	}
	if got := numberedPath("a.txt", 100); got != "a (100).txt" {
		t.Errorf("numberedPath(a.txt, 100) = %q", got)
	}
}

// A list or object becomes compact JSON, scalars their natural form. A value that
// cannot be encoded fails the node instead of writing "<unserializable>" into a file.
func TestDocTextIsNeverUnserializable(t *testing.T) {
	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	reg := docRegistry(t, nil)
	cases := []struct {
		name string
		v    any
		want string // "" with fail: the node must fail
		fail bool
	}{
		{"nil", nil, "", false}, {"text", "<b>&</b>", "<b>&</b>", false}, {"number", 5.0, "5", false}, {"bool", true, "true", false},
		{"list", []any{1.0, "a"}, `[1,"a"]`, false}, {"object", map[string]any{"a": "<&>"}, `{"a":"<&>"}`, false},
		{"cyclic", cyclic, "", true}, {"NaN in list", []any{math.NaN()}, "", true}, {"func", func() {}, "", true}, {"chan", make(chan int), "", true},
	}
	for _, typ := range []string{TypeFileWrite, TypePDFCreate} {
		for _, param := range []string{"content", "title"} {
			if typ == TypeFileWrite && param == "title" {
				continue
			}
			for _, c := range cases {
				tools := &fakeTools{respond: docOK}
				params := map[string]any{"content": "c", "path": "a.txt"}
				params[param] = c.v
				_, err := execDef(lookupDef(t, reg, typ), params, &Services{Tools: tools})
				name := typ + " " + param + " " + c.name
				if c.fail {
					if ne := asNodeError(err); ne == nil || ne.Code != "FLOW_PARAM_INVALID" || tools.count() != 0 || !strings.Contains(ne.Message, param) {
						t.Errorf("%s: error %v after %d calls", name, err, tools.count())
					}
					continue
				}
				if err != nil {
					t.Errorf("%s: %v", name, err)
					continue
				}
				if got := Stringify(tools.last(t).Args[param]); got != c.want {
					t.Errorf("%s: sent %q, want %q", name, got, c.want)
				}
			}
		}
	}
}
