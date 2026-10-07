package desktop

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCreateSymlinkUsesOpenedRealParent(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("symlink path behavior is covered on Linux")
	}
	svc := testService(t)
	ctx := context.Background()
	root := svc.Config().WorkspaceDir
	if err := svc.WriteFileBytes(ctx, "file.txt", []byte("inside"), SourceUser); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "deep"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("..", filepath.Join(root, "deep", "link")); err != nil {
		t.Fatal(err)
	}
	if err := svc.CreateSymlink(ctx, "file.txt", "deep/link/leak"); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(root, "leak")
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	if target != "file.txt" {
		t.Fatalf("symlink target = %q, want file.txt", target)
	}
	content, err := os.ReadFile(linkPath)
	if err != nil || string(content) != "inside" {
		t.Fatalf("internal symlink content = %q, %v", content, err)
	}
}
