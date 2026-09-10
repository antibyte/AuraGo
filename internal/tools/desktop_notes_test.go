package tools

import (
	"aurago/internal/desktop"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopNotesAgentRights(t *testing.T) {
	cfg := testVirtualDesktopConfig(t)
	ctx := context.Background()
	decode := func(run VirtualDesktopExecution) (string, json.RawMessage) {
		t.Helper()
		var result struct {
			Status string
			Data   json.RawMessage
		}
		if err := json.Unmarshal([]byte(run.Output), &result); err != nil {
			t.Fatal(err, run.Output)
		}
		return result.Status, result.Data
	}
	status, data := decode(ExecuteDesktopNotes(ctx, cfg, map[string]interface{}{"operation": "create", "title": "Research", "content": "# Research\nNeedle München"}))
	if status != "ok" {
		t.Fatal(string(data))
	}
	var note desktop.Note
	if err := json.Unmarshal(data, &note); err != nil || note.Path == "" {
		t.Fatal(err, string(data))
	}
	for _, operation := range []string{"list", "search", "read"} {
		status, _ = decode(ExecuteDesktopNotes(ctx, cfg, map[string]interface{}{"operation": operation, "path": note.Path, "query": "München"}))
		if status != "ok" {
			t.Fatal(operation)
		}
	}
	for _, operation := range []string{"update", "delete", "rename", "move", "archive"} {
		status, _ = decode(ExecuteDesktopNotes(ctx, cfg, map[string]interface{}{"operation": operation, "path": note.Path, "content": "overwritten"}))
		if status != "error" {
			t.Fatal("agent mutation accepted", operation)
		}
	}
	for _, args := range []map[string]interface{}{
		{"offset": 1.5}, {"offset": -1.0}, {"offset": math.Inf(1)}, {"offset": "0"}, {"limit": 0.0}, {"limit": 201.0},
	} {
		args["operation"] = "list"
		status, _ = decode(ExecuteDesktopNotes(ctx, cfg, args))
		if status != "error" {
			t.Fatal("invalid pagination accepted", args)
		}
	}
	status, _ = decode(ExecuteVirtualDesktop(ctx, cfg, map[string]interface{}{"operation": "write_file", "path": note.Path, "content": "generic overwrite"}))
	if status != "error" {
		t.Fatal("generic Desktop tool bypass")
	}
	bytes, err := os.ReadFile(filepath.Join(cfg.VirtualDesktop.WorkspaceDir, filepath.FromSlash(note.Path)))
	if err != nil || string(bytes) != "# Research\nNeedle München" {
		t.Fatal("original changed", err)
	}
	cfg.VirtualDesktop.AllowAgentControl = false
	status, _ = decode(ExecuteDesktopNotes(ctx, cfg, map[string]interface{}{"operation": "read", "path": note.Path}))
	if status != "error" {
		t.Fatal("disabled agent access")
	}
}

func TestNotesNativeFileGuards(t *testing.T) {
	root := t.TempDir()
	notes := filepath.Join(root, "Documents", "Notes")
	path := filepath.Join(notes, "original.md")
	if err := os.MkdirAll(notes, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	previous, configured := currentRuntimePermissions()
	ConfigureRuntimePermissions(RuntimePermissions{AllowFilesystemWrite: true, ProtectedNotesRoots: []string{notes}})
	defer func() {
		if configured {
			ConfigureRuntimePermissions(previous)
		} else {
			ClearRuntimePermissionsForTest()
		}
	}()
	for _, target := range []string{path, notes, filepath.Dir(notes), root} {
		if err := requireUnprotectedNotesPath(target, true); err == nil {
			t.Fatal("unguarded mutation", target)
		}
	}
	if err := requireUnprotectedNotesPath(filepath.Join(root, "unrelated.md"), true); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(path, []byte("changed")); err == nil {
		t.Fatal("file editor bypass")
	}
	bytes, _ := os.ReadFile(path)
	if string(bytes) != "original" {
		t.Fatal("protected file changed")
	}
}
