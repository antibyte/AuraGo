package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/sandbox"
)

func TestAnsibleLocalRealTargetPolicy(t *testing.T) {
	if os.Getenv("AURAGO_RUN_ANSIBLE_SMOKE") != "1" {
		t.Skip("opt in to real local Ansible acceptance")
	}
	for _, binary := range []string{"ansible", "ansible-inventory", "ansible-playbook"} {
		if _, err := exec.LookPath(binary); err != nil {
			t.Fatalf("required acceptance binary missing: %s: %v", binary, err)
		}
	}
	t.Cleanup(sandbox.SetForTest(&sandbox.FallbackSandbox{}))
	perms := defaultRuntimePermissionsForTests()
	perms.AllowShell, perms.AllowUnsafeHostExecution = true, true
	ConfigureRuntimePermissions(perms)
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	root := t.TempDir()
	inventory := filepath.Join(root, "hosts.ini")
	if err := os.WriteFile(inventory, []byte("[lab]\nallowed ansible_connection=local\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := AnsibleLocalConfig{Context: WithRuntimePermissions(context.Background(), perms), PlaybooksDir: root, DefaultInventory: inventory, WorkDir: t.TempDir(), Timeout: 60, ReadOnly: true}
	stdout, stderr, err := ansibleRunCmd(cfg, "ansible", "lab", "-i", inventory, "-m", "ping")
	if err != nil || !strings.Contains(stdout, "pong") {
		t.Fatalf("declared group failed: %v; stdout=%s stderr=%s", err, stdout, stderr)
	}
	if _, _, err := ansibleRunCmd(cfg, "ansible", "localhost", "-i", inventory, "-m", "setup"); err == nil || !strings.Contains(err.Error(), "outside the static inventory") {
		t.Fatalf("implicit localhost not rejected: %v", err)
	}
	for _, readonly := range []bool{true, false} {
		cfg.ReadOnly = readonly
		if _, _, err := ansibleRunCmd(cfg, "ansible", "allowed", "-i", inventory, "-m", "setup", "--extra-vars", `{"ansible_connection":"local"}`); err == nil {
			t.Fatal("connection override accepted")
		}
	}
	playbook := filepath.Join(root, "site.yml")
	if err := os.WriteFile(playbook, []byte("- hosts: '{{ target }}'\n  gather_facts: false\n  tasks:\n    - ansible.builtin.ping:\n"), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err = ansibleRunCmd(cfg, "ansible-playbook", "site.yml", "-i", inventory, "-e", `{"target":"lab"}`)
	if err != nil || !strings.Contains(stdout, "ok=1") {
		t.Fatalf("application vars or playbook preflight failed: %v; stdout=%s stderr=%s", err, stdout, stderr)
	}
}
