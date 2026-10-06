package desktop

import (
	"context"
	"io"
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

func TestRootedHTTPFileSystemAllowsOnlyContainedSymlinks(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("symlink serving is covered on Linux")
	}
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("inside.txt", filepath.Join(root, "inside-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "inside.txt"), filepath.Join(root, "absolute-inside-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "outside-link")); err != nil {
		t.Fatal(err)
	}
	fSys, err := NewRootedHTTPFileSystem(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"/inside-link", "/absolute-inside-link"} {
		file, err := fSys.Open(name)
		if err != nil {
			t.Fatalf("open contained symlink %s: %v", name, err)
		}
		content, err := io.ReadAll(file)
		closeErr := file.Close()
		if err != nil || closeErr != nil || string(content) != "inside" {
			t.Fatalf("contained symlink %s read = %q, read %v, close %v", name, content, err, closeErr)
		}
	}
	if file, err := fSys.Open("/outside-link"); err == nil {
		file.Close()
		t.Fatal("served symlink outside workspace")
	}
}
