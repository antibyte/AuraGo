package tools

import (
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/testutil"
)

func writeComposePathFixture(t *testing.T, workspace string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(workspace, "compose.yml"), []byte("services: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestDockerComposePathResolutionFailsClosed(t *testing.T) {
	workspace, outside := t.TempDir(), t.TempDir()
	for _, root := range []string{workspace, outside} {
		if err := os.WriteFile(filepath.Join(root, "compose.yml"), []byte("services: {}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := DockerConfig{WorkspaceDir: workspace}
	if _, err := ResolveDockerComposeFile(cfg, "compose.yml"); err != nil {
		t.Fatal(err)
	}
	testutil.DirectoryLink(t, filepath.Join(workspace, "external"), outside)
	if _, err := ResolveDockerComposeFile(cfg, "external/compose.yml"); err == nil {
		t.Fatal("external link accepted")
	}
	if _, err := ResolveDockerComposeFile(cfg, "missing.yml"); err == nil {
		t.Fatal("unresolved file accepted")
	}
	dangling := filepath.Join(outside, "removed")
	if err := os.Mkdir(dangling, 0700); err != nil {
		t.Fatal(err)
	}
	testutil.DirectoryLink(t, filepath.Join(workspace, "dangling"), dangling)
	if err := os.Remove(dangling); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveDockerComposeFile(cfg, "dangling/compose.yml"); err == nil {
		t.Fatal("unresolved directory link accepted")
	}
}
