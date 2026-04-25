package tools

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSSHKeyManagerGenerateDefaultsToEd25519(t *testing.T) {
	t.Parallel()

	m := NewSSHKeyManager(t.TempDir())
	privateKey, publicKey, err := m.Generate("test", false)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(privateKey, "OPENSSH PRIVATE KEY") {
		t.Fatalf("expected OpenSSH private key, got %q", privateKey[:min(len(privateKey), 40)])
	}
	if !strings.HasPrefix(publicKey, "ssh-ed25519 ") {
		t.Fatalf("public key = %q, want ssh-ed25519 prefix", publicKey)
	}
}

func TestSSHKeyManagerRejectsAuthorizedKeysPathOutsideWorkspaceOrHome(t *testing.T) {
	t.Parallel()

	m := NewSSHKeyManager(t.TempDir())
	err := m.SetAuthorizedKeysPath(filepath.Join(t.TempDir(), "authorized_keys"))
	if err == nil || !strings.Contains(err.Error(), "outside allowed") {
		t.Fatalf("expected outside-path rejection, got %v", err)
	}
}

func TestSSHKeyManagerAllowsAuthorizedKeysPathInsideWorkspace(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	m := NewSSHKeyManager(workspace)
	path := filepath.Join(workspace, "nested", "authorized_keys")
	if err := m.SetAuthorizedKeysPath(path); err != nil {
		t.Fatalf("expected workspace path to be allowed: %v", err)
	}
}
