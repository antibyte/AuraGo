package tools

import (
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

func TestResolveOutgoingAttachmentPath(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	docs := filepath.Join(root, "data", "documents")
	for _, dir := range []string{workspace, docs} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path string) {
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	inWorkspace := filepath.Join(workspace, "bericht.pdf")
	inDocs := filepath.Join(docs, "news.pdf")
	secret := filepath.Join(root, "config.yaml")
	write(inWorkspace)
	write(inDocs)
	write(secret)

	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workspace
	cfg.Tools.DocumentCreator.OutputDir = docs

	for _, ok := range []string{inWorkspace, "bericht.pdf", inDocs} {
		got, err := ResolveOutgoingAttachmentPath(ok, cfg)
		if err != nil {
			t.Errorf("%s: %v", ok, err)
			continue
		}
		if !filepath.IsAbs(got) {
			t.Errorf("%s resolved to a relative path %q", ok, got)
		}
	}
	for _, bad := range []string{secret, "../config.yaml", filepath.Join(workspace, "missing.pdf"), workspace, "", "   "} {
		if _, err := ResolveOutgoingAttachmentPath(bad, cfg); err == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
	if _, err := ResolveOutgoingAttachmentPath(inWorkspace, nil); err == nil {
		t.Error("a nil config must be rejected")
	}
}
