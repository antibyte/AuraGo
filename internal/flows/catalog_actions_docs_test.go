package flows

import (
	"reflect"
	"testing"
)

func docRegistry(t *testing.T, env CatalogEnv) *Registry {
	t.Helper()
	reg := NewRegistry()
	if err := registerDocNodes(reg, env); err != nil {
		t.Fatal(err)
	}
	return reg
}

func TestFileRefHelpers(t *testing.T) {
	ref := FileRef("/data/documents/ki.pdf", "", "", "/files/documents/ki.pdf", 1200)
	want := map[string]any{"$type": "file", "path": "/data/documents/ki.pdf", "name": "ki.pdf", "mime": "application/pdf",
		"size": 1200.0, "web_path": "/files/documents/ki.pdf"}
	if !reflect.DeepEqual(ref, want) {
		t.Fatalf("FileRef = %#v", ref)
	}
	if FilePath(ref) != "/data/documents/ki.pdf" || FilePath(" notes/a.txt ") != "notes/a.txt" || FilePath(3.0) != "" {
		t.Fatal("FilePath mismatch")
	}
	if mimeForName("a.unknownext") != "application/octet-stream" {
		t.Fatal("unknown extensions use application/octet-stream")
	}
}

func TestPDFCreate(t *testing.T) {
	tools := &fakeTools{respond: toolReply(`{"status":"success","file_path":"/data/documents/ki.pdf","web_path":"/files/documents/ki.pdf","filename":"ki.pdf","backend":"maroto"}`)}
	def := lookupDef(t, docRegistry(t, StaticEnv{"document_creator": {}}), TypePDFCreate)
	res, err := execDef(def, map[string]any{"title": "KI", "content": "Text"}, &Services{Tools: tools})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	file := res.Output["file"].(map[string]any)
	if file["path"] != "/data/documents/ki.pdf" || file["name"] != "ki.pdf" || file["mime"] != "application/pdf" || res.Output["web_path"] != "/files/documents/ki.pdf" {
		t.Fatalf("output = %#v", res.Output)
	}
	args := tools.last(t).Args
	want := map[string]any{"operation": "create_pdf", "title": "KI", "content": "Text", "paper_size": "A4", "landscape": false}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v", args)
	}
	if _, err := execDef(def, map[string]any{"content": "# H", "format": "markdown", "filename": "report.pdf"}, &Services{Tools: tools}); err != nil {
		t.Fatal(err)
	}
	if args := tools.last(t).Args; args["operation"] != "markdown_to_pdf" || args["filename"] != "report" {
		t.Fatalf("markdown args = %#v", args)
	}
	tools.respond = toolReply(`{"status":"success"}`)
	if _, err := execDef(def, map[string]any{"content": "x"}, &Services{Tools: tools}); asNodeError(err).Code != "FLOW_TOOL_ERROR" {
		t.Fatalf("missing file_path = %v", err)
	}
	node := &Node{ID: testNodeID(1), Params: map[string]any{"format": "markdown"}}
	if issues := def.Validate(node, ValidateContext{}); len(issues) != 1 || issues[0].Code != IssueParamInvalid {
		t.Fatalf("markdown without Gotenberg = %+v", issues)
	}
	withGotenberg := lookupDef(t, docRegistry(t, StaticEnv{"document_creator": {}, GotenbergTool: {}}), TypePDFCreate)
	if issues := withGotenberg.Validate(node, ValidateContext{}); len(issues) != 0 {
		t.Fatalf("markdown with Gotenberg = %+v", issues)
	}
}

func TestPDFRead(t *testing.T) {
	tools := &fakeTools{respond: toolReply("{\"status\":\"success\",\"content\":\"<external_data>\\nSeite 1\\n</external_data>\"}")}
	def := lookupDef(t, docRegistry(t, nil), TypePDFRead)
	res, err := execDef(def, map[string]any{"file": FileRef("docs/a.pdf", "", "", "", 0)}, &Services{Tools: tools})
	if err != nil || res.Output["text"] != "Seite 1" {
		t.Fatalf("read = %#v, %v", res.Output, err)
	}
	if req := tools.last(t); req.Tool != PDFExtractorTool || req.Args["filepath"] != "docs/a.pdf" {
		t.Fatalf("request = %+v", req)
	}
	if _, err := execDef(def, map[string]any{"file": ""}, &Services{Tools: tools}); asNodeError(err).Code != "FLOW_PARAM_INVALID" {
		t.Fatalf("empty file = %v", err)
	}
}

func TestFileRead(t *testing.T) {
	tools := &fakeTools{respond: toolReply(`{"status":"success","message":"Read 5 bytes","data":"Hallo"}`)}
	def := lookupDef(t, docRegistry(t, nil), TypeFileRead)
	res, err := execDef(def, map[string]any{"path": "notes/a.txt"}, &Services{Tools: tools})
	if err != nil || res.Output["content"] != "Hallo" || res.Output["file"].(map[string]any)["path"] != "notes/a.txt" {
		t.Fatalf("read = %#v, %v", res.Output, err)
	}
	if args := tools.last(t).Args; args["operation"] != "read_file" || args["file_path"] != "notes/a.txt" {
		t.Fatalf("args = %#v", args)
	}
	tools.respond = toolReply(`{"status":"success","data":{"content":"Inhalt","format":"text"}}`)
	if res, _ := execDef(def, map[string]any{"path": "x"}, &Services{Tools: tools}); res.Output["content"] != "Inhalt" {
		t.Fatalf("object data = %#v", res.Output)
	}
}

func TestFileWrite(t *testing.T) {
	existing := map[string]bool{"notes/a.txt": true}
	tools := &fakeTools{}
	tools.respond = func(req ToolRequest) (ToolResponse, error) {
		switch req.Args["operation"] {
		case "stat":
			if existing[Stringify(req.Args["file_path"])] {
				return ToolResponse{Output: `{"status":"success","data":{"name":"a.txt","size":3}}`}, nil
			}
			return ToolResponse{Output: `{"status":"error","message":"stat failed","data":{"error_code":"io_error"}}`, IsError: true, Status: "failed"}, nil
		case "write_file":
			return ToolResponse{Output: `{"status":"success","message":"Wrote 5 bytes"}`}, nil
		}
		return ToolResponse{Output: "unexpected", IsError: true}, nil
	}
	def := lookupDef(t, docRegistry(t, nil), TypeFileWrite)

	res, err := execDef(def, map[string]any{"path": "notes/a.txt", "content": "Hallo"}, &Services{Tools: tools})
	if err != nil || tools.count() != 1 || res.Output["file"].(map[string]any)["path"] != "notes/a.txt" {
		t.Fatalf("overwrite = %#v, %v, %d calls", res.Output, err, tools.count())
	}
	res, err = execDef(def, map[string]any{"path": "notes/a.txt", "content": "Hallo", "if_exists": "unique"}, &Services{Tools: tools})
	if err != nil || res.Output["file"].(map[string]any)["path"] != "notes/a (2).txt" {
		t.Fatalf("unique = %#v, %v", res.Output, err)
	}
	if args := tools.last(t).Args; args["operation"] != "write_file" || args["file_path"] != "notes/a (2).txt" || args["content"] != "Hallo" {
		t.Fatalf("unique write args = %#v", args)
	}
	if _, err := execDef(def, map[string]any{"path": "notes/a.txt", "content": "x", "if_exists": "fail"}, &Services{Tools: tools}); asNodeError(err).Code != "FLOW_FILE_EXISTS" {
		t.Fatalf("fail mode = %v", err)
	}
	if !reflect.DeepEqual(def.Effects, []Effect{EffectWritesFiles}) {
		t.Fatalf("effects = %v", def.Effects)
	}
}
