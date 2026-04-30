package tools

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCreateBackupResolvesHeadHashViaRevParse(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	if _, stderr, rc := runGitCmd(dir, "init"); rc != 0 {
		t.Fatalf("git init failed: %s", stderr)
	}
	if _, stderr, rc := runGitCmd(dir, "config", "user.name", "AuraGo Test"); rc != 0 {
		t.Fatalf("git config user.name failed: %s", stderr)
	}
	if _, stderr, rc := runGitCmd(dir, "config", "user.email", "aurago@example.com"); rc != 0 {
		t.Fatalf("git config user.email failed: %s", stderr)
	}
	if err := os.WriteFile(dir+string(os.PathSeparator)+"README.md", []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	result := createBackup(dir, "test backup")
	if result["status"] != "success" {
		t.Fatalf("unexpected status: %#v", result)
	}
	hash, _ := result["commit_hash"].(string)
	if hash == "" {
		t.Fatalf("expected commit hash, got %#v", result)
	}
	head, err := resolveGitHeadHash(dir)
	if err != nil {
		t.Fatalf("resolveGitHeadHash failed: %v", err)
	}
	if hash != head {
		t.Fatalf("commit hash = %q, want %q", hash, head)
	}
	if message, _ := result["message"].(string); !strings.Contains(message, hash) {
		t.Fatalf("message %q does not contain commit hash %q", message, hash)
	}
}

func TestRestoreGitHardModeResetsToCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	if _, stderr, rc := runGitCmd(dir, "init"); rc != 0 {
		t.Fatalf("git init failed: %s", stderr)
	}
	if _, stderr, rc := runGitCmd(dir, "config", "user.name", "AuraGo Test"); rc != 0 {
		t.Fatalf("git config user.name failed: %s", stderr)
	}
	if _, stderr, rc := runGitCmd(dir, "config", "user.email", "aurago@example.com"); rc != 0 {
		t.Fatalf("git config user.email failed: %s", stderr)
	}
	path := dir + string(os.PathSeparator) + "README.md"
	if err := os.WriteFile(path, []byte("one\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if result := createBackup(dir, "first"); result["status"] != "success" {
		t.Fatalf("first commit failed: %#v", result)
	}
	first, err := resolveGitHeadHash(dir)
	if err != nil {
		t.Fatalf("resolve first hash: %v", err)
	}
	if err := os.WriteFile(path, []byte("two\n"), 0o644); err != nil {
		t.Fatalf("write file second: %v", err)
	}
	if result := createBackup(dir, "second"); result["status"] != "success" {
		t.Fatalf("second commit failed: %#v", result)
	}

	result := restoreGit(dir, first, "hard")
	if result["status"] != "success" {
		t.Fatalf("restoreGit hard failed: %#v", result)
	}
	head, err := resolveGitHeadHash(dir)
	if err != nil {
		t.Fatalf("resolve head hash: %v", err)
	}
	if head != first {
		t.Fatalf("HEAD = %q, want %q", head, first)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if strings.TrimSpace(string(data)) != "one" {
		t.Fatalf("README content = %q, want one", string(data))
	}
}

func TestRestoreGitRejectsInvalidRefs(t *testing.T) {
	result := restoreGit(t.TempDir(), "--help", "hard")
	if result["status"] != "error" {
		t.Fatalf("expected invalid ref error, got %#v", result)
	}
}
