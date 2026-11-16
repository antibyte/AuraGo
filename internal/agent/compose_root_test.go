package agent

import (
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/testutil"
	"aurago/internal/tools"
)

func TestReadDockerComposeFileRejectsSwappedDirectory(t *testing.T) {
	workspace, outside := t.TempDir(), t.TempDir()
	nested := filepath.Join(workspace, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{nested, outside} {
		if err := os.WriteFile(filepath.Join(dir, "compose.yml"), []byte("services: {}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	file, err := tools.ResolveDockerComposeFile(tools.DockerConfig{WorkspaceDir: workspace}, "nested/compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readDockerComposeFile(workspace, file); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(nested, nested+"-original"); err != nil {
		t.Fatal(err)
	}
	testutil.DirectoryLink(t, nested, outside)
	if _, err := readDockerComposeFile(workspace, file); err == nil {
		t.Fatal("rooted read followed swapped external directory")
	}
}
