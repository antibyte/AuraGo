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
