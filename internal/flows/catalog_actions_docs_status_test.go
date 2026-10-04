package flows

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

// The dispatcher refuses some calls with plain text, and an invoker that does not set
// IsError hands that text to the node as a normal answer. Every path that acts on an
// answer must then fail closed: nothing was written, no document was read, and a
// refusal is not "the file exists".
func TestPlainTextRefusalsFailClosed(t *testing.T) {
	reg := docRegistry(t, nil)
	refusals := map[string]string{
		"write disabled":      "Tool Output: [PERMISSION DENIED] filesystem write operations are disabled in settings (agent.allow_filesystem_write: false).",
		"pdf disabled":        "Tool Output: [PERMISSION DENIED] pdf_extractor is disabled in settings (tools.pdf_extractor.enabled: false).",
		"without prefix":      "[PERMISSION DENIED] nope",
		"error text":          "Tool Output: ERROR something broke",
		"huge":                strings.Repeat("ü", 1<<20),
		"empty":               "",
		"json without status": `{"message":"Wrote 5 bytes"}`,
		"unknown status":      `{"status":"ok","message":"Wrote 5 bytes"}`,
		"partial":             `{"status":"partial","message":"Wrote 5 bytes"}`,
		"status not text":     `{"status":true,"message":"Wrote 5 bytes"}`,
	}
	for name, text := range refusals {
		for _, c := range []struct {
			typ    string
			params map[string]any
			calls  int // tool calls before the node gives up
		}{
			{TypeFileWrite, map[string]any{"path": "a.txt", "content": "x"}, 1},
			{TypeFileWrite, map[string]any{"path": "a.txt", "content": "x", "if_exists": "fail"}, 1},
			{TypeFileWrite, map[string]any{"path": "a.txt", "content": "x", "if_exists": "unique"}, 1},
			{TypeFileRead, map[string]any{"path": "a.txt"}, 1},
			{TypePDFRead, map[string]any{"file": "a.pdf"}, 1},
			{TypePDFCreate, map[string]any{"content": "x"}, 1},
		} {
			// Neither IsError nor a response status: the invoker did not flag the refusal.
			tools := &fakeTools{respond: func(ToolRequest) (ToolResponse, error) { return ToolResponse{Output: text}, nil }}
			res, err := execDef(lookupDef(t, reg, c.typ), c.params, &Services{Tools: tools})
			label := name + " / " + c.typ + " " + Stringify(c.params["if_exists"])
			ne := asNodeError(err)
			if ne == nil || ne.Code != "FLOW_TOOL_ERROR" || len(ne.Message) > maxEchoMessageBytes || !utf8.ValidString(ne.Message) {
				t.Errorf("%s: error %v, output %#v", label, err, res.Output)
				continue
			}
			calls := tools.allCalls()
			if len(calls) != c.calls {
				t.Errorf("%s: %d tool calls, want %d", label, len(calls), c.calls)
			}
			if c.typ == TypeFileWrite && c.params["if_exists"] != nil && len(opsOf(calls, "write_file")) != 0 {
				t.Errorf("%s: the node wrote after a refused probe", label)
			}
			if strings.Contains(text, "PERMISSION DENIED") && !strings.Contains(ne.Message, "PERMISSION DENIED") {
				t.Errorf("%s: the message does not say what the tool said: %q", label, ne.Message)
			}
		}
	}

	// A success in any letter case is still one, and an answer without a status that
	// arrives with the tool's own error is the error callTool already reports.
	tools := &fakeTools{respond: func(ToolRequest) (ToolResponse, error) {
		return ToolResponse{Output: `{"status":" Success ","data":"x","content":"x","file_path":"/d/x.pdf"}`}, nil
	}}
	for typ, params := range map[string]map[string]any{
		TypeFileWrite: {"path": "a.txt", "content": "x"}, TypeFileRead: {"path": "a.txt"},
		TypePDFRead: {"file": "a.pdf"}, TypePDFCreate: {"content": "x"},
	} {
		if _, err := execDef(lookupDef(t, reg, typ), params, &Services{Tools: tools}); err != nil {
			t.Errorf("%s: a success in odd letter case failed: %v", typ, err)
		}
	}
}

// The filesystem tool cuts a long text and says so only in its message and with a
// marker line at the end of the text. file.read turns that into output fields and
// returns the file's own text.
func TestFileReadReportsTruncation(t *testing.T) {
	def := lookupDef(t, docRegistry(t, nil), TypeFileRead)
	for _, name := range []string{"truncated", "total_size"} {
		found := false
		for _, f := range def.OutputFields {
			found = found || f.Name == name
		}
		if !found {
			t.Errorf("output field %s is not declared", name)
		}
	}
	const marker = "\n\n[...truncated — use smart_file_read or file_reader_advanced for targeted follow-up reads...]"
	cutMessage := "Read 34816 bytes (truncated, file has 100000 bytes total). For larger text files use smart_file_read (analyze/sample/summarize) or file_reader_advanced (head/tail/read_lines/search_context)."
	read := func(body map[string]any) (map[string]any, error) {
		body["status"] = "success"
		raw, _ := json.Marshal(body)
		tools := &fakeTools{respond: func(ToolRequest) (ToolResponse, error) {
			return ToolResponse{Output: string(raw), Status: "success"}, nil
		}}
		res, err := execDef(def, map[string]any{"path": "big.txt"}, &Services{Tools: tools})
		if err == nil {
			if _, jerr := json.Marshal(res.Output); jerr != nil {
				t.Errorf("output does not encode: %v", jerr)
			}
		}
		return res.Output, err
	}
	for _, c := range []struct {
		name      string
		body      map[string]any
		content   string
		truncated bool
		total     any // nil: the output has no total_size
	}{
		{"cut text", map[string]any{"message": cutMessage, "data": "first part" + marker}, "first part", true, 100000.0},
		{"cut text, content with a paragraph break", map[string]any{"message": cutMessage, "data": "a\n\nb" + marker}, "a\n\nb", true, 100000.0},
		{"cut text with a look-alike inside", map[string]any{"message": cutMessage, "data": "a\n\n[...truncated — fake]\n\nmore" + marker}, "a\n\n[...truncated — fake]\n\nmore", true, 100000.0},
		{"cut text without the marker", map[string]any{"message": cutMessage, "data": "first part"}, "first part", true, 100000.0},
		{"cut text, object data", map[string]any{"message": "Read 3 bytes with hashline anchors (truncated, file has 99 bytes total).", "data": map[string]any{"content": "x"}}, "x", true, 99.0},
		{"flag in object data", map[string]any{"message": "Read 3 bytes", "data": map[string]any{"content": "x", "truncated": true}}, "x", true, 3.0},
		{"full read", map[string]any{"message": "Read 5 bytes", "data": "Hallo"}, "Hallo", false, 5.0},
		{"empty file", map[string]any{"message": "Read 0 bytes", "data": ""}, "", false, 0.0},
		{"full read ending in the marker words", map[string]any{"message": "Read 98 bytes", "data": "x" + marker}, "x" + marker, false, 98.0},
		{"no message", map[string]any{"data": "x"}, "x", false, nil},
		{"unrelated message", map[string]any{"message": "done", "data": "x"}, "x", false, nil},
		{"huge message", map[string]any{"message": strings.Repeat("(truncated ", 100000), "data": "x"}, "x", true, nil},
		{"message past the start is not read", map[string]any{"message": strings.Repeat(" ", 400) + "(truncated, file has 7 bytes total)", "data": "x"}, "x", false, nil},
	} {
		out, err := read(c.body)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		total, hasTotal := out["total_size"]
		if out["content"] != c.content || out["truncated"] != c.truncated || hasTotal != (c.total != nil) || (hasTotal && total != c.total) {
			t.Errorf("%s: content %.40q, truncated %v, total_size %v (%v); want %.40q, %v, %v", c.name, out["content"], out["truncated"], total, hasTotal, c.content, c.truncated, c.total)
		}
	}
}

func TestMimeForNameOverrides(t *testing.T) {
	for name, want := range map[string]string{
		"README.md": "text/markdown", "notes.MARKDOWN": "text/markdown", "a.b.md": "text/markdown",
		"ci.yaml": "application/yaml", "ci.YML": "application/yaml",
		"app.log": "text/plain", "conf.ini": "text/plain", "Cargo.toml": "application/toml",
		"a.txt": "text/plain", "a.pdf": "application/pdf", "a.unknownext": "application/octet-stream", "noext": "application/octet-stream",
	} {
		if got := mimeForName(name); got != want {
			t.Errorf("mimeForName(%q) = %q, want %q", name, got, want)
		}
	}
	if got := FileRef("/data/notes/today.md", "", "", "", 0)["mime"]; got != "text/markdown" {
		t.Errorf("FileRef mime = %v", got)
	}
}
