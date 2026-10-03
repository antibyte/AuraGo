package gamemaker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilesystemReceiptRecoveryRejectsMissingDeleteTarget(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	receipt := filesystemReceipt{Version: 1, ID: "delete_missing", Kind: "delete", ProjectID: project.ID,
		ProjectKey: project.ProjectKey, HadTarget: true}
	path, err := s.writeFilesystemReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = NewService(s.opts)
	if err == nil || !strings.Contains(err.Error(), "deletion artifacts missing") {
		t.Fatalf("NewService error = %v, want missing deletion target", err)
	}
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("ambiguous receipt was discarded: %v", err)
	}
}

func TestFilesystemReceiptRecoveryRestoresInterruptedDeletion(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	receipt := filesystemReceipt{Version: 1, ID: "delete_interrupted", Kind: "delete", ProjectID: project.ID,
		ProjectKey: project.ProjectKey, HadTarget: true}
	target, backup, _, err := s.receiptPaths(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "marker.txt"), []byte("published"), 0o640); err != nil {
		t.Fatal(err)
	}
	path, err := s.writeFilesystemReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(target, backup); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	recovered, err := NewService(s.opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = recovered.Close() })
	content, err := os.ReadFile(filepath.Join(target, "marker.txt"))
	if err != nil || string(content) != "published" {
		t.Fatalf("project was not restored: %q, %v", content, err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("recovered receipt was not removed: %v", err)
	}
}

func TestFilesystemReceiptRecoveryRemovesIncompleteWrite(t *testing.T) {
	s := newTestService(t)
	path := filepath.Join(s.receiptDir(), "publish_unfinished.json.tmp-123456")
	if err := os.MkdirAll(s.receiptDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("incomplete"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	recovered, err := NewService(s.opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = recovered.Close() })
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("incomplete receipt was not removed: %v", err)
	}
}

func TestFilesystemReceiptPublicationAcceptsRelativeWorkspace(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Rel(wd, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewService(Options{
		DBPath:        filepath.Join(root, "data", "game_maker.db"),
		WorkspacePath: filepath.Join(root, "workspace"),
		Enabled:       true,
		AllowCreate:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	project := createTestProject(t, s, "2d")
	stage := filepath.Join(s.stagingDir, "relative-stage")
	if err := os.MkdirAll(stage, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "marker.txt"), []byte("relative"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := s.publish(context.Background(), stage, project, Job{}, "test", "relative path"); err != nil {
		t.Fatal(err)
	}
}
