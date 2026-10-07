package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/tools"
)

// c13DocsConfig lays out a workspace and a documents folder like a real installation
// (data/documents next to agent_workspace/workdir).
func c13DocsConfig(t *testing.T) (*config.Config, string, string) {
	t.Helper()
	root := t.TempDir()
	workspace := filepath.Join(root, "agent_workspace", "workdir")
	docs := filepath.Join(root, "data", "documents")
	for _, dir := range []string{workspace, docs} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workspace
	cfg.Directories.DataDir = filepath.Join(root, "data")
	cfg.Tools.DocumentCreator.Enabled = true
	cfg.Tools.DocumentCreator.Backend = "maroto"
	cfg.Tools.DocumentCreator.OutputDir = docs
	cfg.Tools.PDFExtractor.Enabled = true
	return cfg, workspace, docs
}

// c13CopyCheck is a fake dispatcher that records the path the tool got, the file's content
// at that moment, and answers success.
func c13CopyCheck(key string, seen *[]string) func(context.Context, *agent.ToolCall, *agent.DispatchContext) agent.ToolDispatchResult {
	return func(_ context.Context, tc *agent.ToolCall, _ *agent.DispatchContext) agent.ToolDispatchResult {
		args := tc.Params
		if tc.Action == "execute_skill" {
			args = tc.SkillArgs
		}
		p, _ := args[key].(string)
		data, err := os.ReadFile(p)
		if err != nil {
			*seen = append(*seen, p, "unreadable: "+err.Error())
		} else {
			*seen = append(*seen, p, string(data))
		}
		return agent.ToolDispatchResult{Output: `Tool Output: {"status":"success","content":"ok"}`, Status: agent.ToolResultSuccess}
	}
}

// C12: pdf_create → pdf_read on the returned path works through the invoker, with the real
// document_creator and the real pdf_extractor skill.
func TestC13PDFCreateThenPDFReadThroughTheInvoker(t *testing.T) {
	cfg, workspace, docs := c13DocsConfig(t)
	inv, s, _ := c13Invoker(cfg, map[string]bool{"document_creator": true, "execute_skill": true}, func(ctx context.Context, tc *agent.ToolCall, dc *agent.DispatchContext) agent.ToolDispatchResult {
		return agent.DispatchToolCallResult(ctx, tc, dc, "")
	})
	s.bindRuntimePermissions()
	t.Cleanup(func() { tools.SetRuntimePermissionResolver(nil) })
	env := flows.StaticEnv{"document_creator": {}, flows.PDFExtractorTool: {}}

	created, err := c13Exec(t, flows.TypePDFCreate, env, inv, map[string]any{"title": "Bridge", "content": "C13 bridge sentence for the reader."}, "run_c13")
	if err != nil {
		t.Fatalf("doc.pdf_create: %v", err)
	}
	file, _ := created.Output["file"].(map[string]any)
	pdfPath, _ := file["path"].(string)
	if !strings.HasPrefix(pdfPath, docs) {
		t.Fatalf("the PDF is not in the documents folder: %v", created.Output)
	}
	for _, ref := range []any{file, pdfPath, created.Output["web_path"]} {
		read, err := c13Exec(t, flows.TypePDFRead, env, inv, map[string]any{"file": ref}, "run_c13")
		if err != nil {
			t.Fatalf("doc.pdf_read of %v: %v", ref, err)
		}
		if text, _ := read.Output["text"].(string); !strings.Contains(text, "C13 bridge sentence") {
			t.Fatalf("doc.pdf_read of %v = %.300v", ref, read.Output)
		}
	}
	// The copies are gone.
	leftovers, _ := filepath.Glob(filepath.Join(workspace, flowScratchDir, "*", "doc-*"))
	if len(leftovers) != 0 {
		t.Fatalf("scratch copies were left: %v", leftovers)
	}
}

// C12: file.read and doc.pdf_read of a documents path read a copy in the workspace that is
// gone after the call; other paths reach the tool unchanged.
func TestC13DocumentsBridgeCopiesAndCleansUp(t *testing.T) {
	cfg, workspace, docs := c13DocsConfig(t)
	if err := os.WriteFile(filepath.Join(docs, "report.pdf"), []byte("%PDF-c13"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "notes.txt"), []byte("c13 notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		tool, key string
		args      map[string]any
		content   string
	}{
		{flows.PDFExtractorTool, "filepath", map[string]any{"filepath": filepath.Join(docs, "report.pdf")}, "%PDF-c13"},
		{flows.PDFExtractorTool, "filepath", map[string]any{"filepath": "/files/documents/report.pdf"}, "%PDF-c13"},
		{"filesystem", "file_path", map[string]any{"operation": "read_file", "file_path": filepath.Join(docs, "notes.txt")}, "c13 notes"},
		{"filesystem", "path", map[string]any{"operation": "read", "path": "/files/documents/notes.txt"}, "c13 notes"},
	}
	for _, c := range cases {
		var seen []string
		inv, _, calls := c13Invoker(cfg, map[string]bool{"filesystem": true, "execute_skill": true}, c13CopyCheck(c.key, &seen))
		original := c.args[c.key]
		resp, err := inv.InvokeTool(context.Background(), c13Request(c.tool, c.args))
		if err != nil || resp.IsError || len(*calls) != 1 {
			t.Fatalf("%s %v: %+v, %v", c.tool, original, resp, err)
		}
		got := seen[0]
		if !strings.HasPrefix(got, filepath.Join(workspace, flowScratchDir, "run_c13")+string(filepath.Separator)) || seen[1] != c.content {
			t.Fatalf("%s %v: the tool read %q with %q", c.tool, original, got, seen[1])
		}
		if _, err := os.Stat(got); !os.IsNotExist(err) {
			t.Fatalf("the copy %s is still there: %v", got, err)
		}
		if c.args[c.key] != original {
			t.Fatal("the node's arguments were changed")
		}
	}

	// Paths that are not in the documents folder go to the tool as they are.
	if err := os.WriteFile(filepath.Join(workspace, "w.txt"), []byte("w"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"w.txt", filepath.Join(workspace, "w.txt"), "/files/generated_images/x.png", filepath.Join(filepath.Dir(docs), "other.txt")} {
		var seen []string
		inv, _, calls := c13Invoker(cfg, map[string]bool{"filesystem": true}, c13CopyCheck("file_path", &seen))
		if _, err := inv.InvokeTool(context.Background(), c13Request("filesystem", map[string]any{"operation": "read_file", "file_path": p})); err != nil || len(*calls) != 1 {
			t.Fatalf("%s: %v", p, err)
		}
		if (*calls)[0].tc.Params["file_path"] != p {
			t.Errorf("%s was rewritten to %v", p, (*calls)[0].tc.Params["file_path"])
		}
	}
	// Writes and other operations are never bridged.
	var seen []string
	inv, _, calls := c13Invoker(cfg, map[string]bool{"filesystem": true}, c13CopyCheck("file_path", &seen))
	target := filepath.Join(docs, "notes.txt")
	if _, err := inv.InvokeTool(context.Background(), c13Request("filesystem", map[string]any{"operation": "write_file", "file_path": target, "content": "x"})); err != nil {
		t.Fatal(err)
	}
	if (*calls)[0].tc.Params["file_path"] != target {
		t.Error("a write was bridged")
	}
}

// C12: a documents path that leaves the folder, or a file the attachment jail refuses, is
// denied without a dispatch.
func TestC13DocumentsBridgeRefusesEscapes(t *testing.T) {
	cfg, _, docs := c13DocsConfig(t)
	secret := filepath.Join(filepath.Dir(docs), "vault.bin")
	if err := os.WriteFile(secret, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	refused := []string{
		"/files/documents/../vault.bin",
		"/files/documents/%2e%2e/vault.bin",
		"/files/documents/a\\..\\..\\vault.bin",
		"/files/documents/missing.pdf",
		filepath.Join(docs, "missing.pdf"),
	}
	if err := os.Symlink(secret, filepath.Join(docs, "link.pdf")); err == nil {
		refused = append(refused, filepath.Join(docs, "link.pdf"), "/files/documents/link.pdf")
	} else {
		t.Logf("symlink case skipped: %v", err)
	}
	big := filepath.Join(docs, "big.pdf")
	if f, err := os.Create(big); err == nil {
		_ = f.Truncate(flowDocumentCopyLimit + 1)
		_ = f.Close()
		refused = append(refused, big)
	}
	for _, p := range refused {
		inv, _, calls := c13Invoker(cfg, map[string]bool{"execute_skill": true}, nil)
		resp, err := inv.InvokeTool(context.Background(), c13Request(flows.PDFExtractorTool, map[string]any{"filepath": p}))
		if err != nil || resp.Status != "denied" || !resp.IsError || len(*calls) != 0 {
			t.Errorf("%s: %+v, %v, dispatched %d", p, resp, err, len(*calls))
		}
		if strings.Contains(resp.Output, "secret") && !strings.Contains(p, "vault") {
			t.Errorf("%s: the refusal shows file content: %s", p, resp.Output)
		}
	}
	// Without a workspace there is nowhere to copy to: that is configuration.
	if err := os.WriteFile(filepath.Join(docs, "ok.pdf"), []byte("%PDF"), 0o644); err != nil {
		t.Fatal(err)
	}
	noWorkspace := *cfg
	noWorkspace.Directories.WorkspaceDir = ""
	inv, _, calls := c13Invoker(&noWorkspace, map[string]bool{"execute_skill": true}, nil)
	resp, err := inv.InvokeTool(context.Background(), c13Request(flows.PDFExtractorTool, map[string]any{"filepath": "/files/documents/ok.pdf"}))
	if err != nil || resp.Status != "needs_setup" || len(*calls) != 0 {
		t.Errorf("no workspace: %+v, %v, dispatched %d", resp, err, len(*calls))
	}
}

// C12: attachments need no bridge: the tools open them with tools.OpenOutgoingAttachment,
// which accepts the documents folder; the invoker passes the path on unchanged.
func TestC13AttachmentsFromTheDocumentsFolder(t *testing.T) {
	cfg, _, docs := c13DocsConfig(t)
	pdf := filepath.Join(docs, "report.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-c13"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, resolved, err := tools.OpenOutgoingAttachment(pdf, cfg)
	if err != nil {
		t.Fatalf("the attachment jail refuses the documents folder: %v", err)
	}
	_ = f.Close()
	if filepath.Base(resolved) != "report.pdf" {
		t.Fatalf("resolved = %s", resolved)
	}
	inv, _, calls := c13Invoker(cfg, map[string]bool{"send_email": true, "send_telegram": true}, nil)
	if _, err := inv.InvokeTool(context.Background(), c13Request("send_email", map[string]any{"to": "a@b.c", "body": "x", "attachments": []any{pdf}})); err != nil {
		t.Fatal(err)
	}
	if _, err := inv.InvokeTool(context.Background(), c13Request("send_telegram", map[string]any{"message": "x", "file_path": pdf})); err != nil {
		t.Fatal(err)
	}
	if got := (*calls)[0].tc.Params["attachments"].([]any)[0]; got != pdf {
		t.Errorf("email attachment = %v", got)
	}
	if got := (*calls)[1].tc.Params["file_path"]; got != pdf {
		t.Errorf("telegram file = %v", got)
	}
}

// C12: run folders older than the retention are swept; the current run's folder stays.
func TestC13ScratchFoldersAreSwept(t *testing.T) {
	cfg, workspace, docs := c13DocsConfig(t)
	if err := os.WriteFile(filepath.Join(docs, "report.pdf"), []byte("%PDF-c13"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(workspace, flowScratchDir, "run_old")
	fresh := filepath.Join(workspace, flowScratchDir, "run_fresh")
	for _, dir := range []string{filepath.Join(old, "doc-left"), fresh} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-flowDocumentScratchRetention - time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	inv, _, _ := c13Invoker(cfg, map[string]bool{"execute_skill": true}, nil)
	if _, err := inv.InvokeTool(context.Background(), c13Request(flows.PDFExtractorTool, map[string]any{"filepath": "/files/documents/report.pdf"})); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("the old run folder is still there: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("the fresh run folder was removed: %v", err)
	}
}

func TestC13ScratchNames(t *testing.T) {
	cases := map[string]string{"run_abc-1": "run_abc-1", "../..": "_", "..": "x", "a/b\\c": "a_b_c", "": "x", "Bericht März.pdf": "Bericht_M_rz.pdf",
		strings.Repeat("a", 300): strings.Repeat("a", 120)}
	for in, want := range cases {
		if got := flowScratchName(in, "x"); got != want {
			t.Errorf("flowScratchName(%q) = %q, want %q", in, got, want)
		}
	}
}
