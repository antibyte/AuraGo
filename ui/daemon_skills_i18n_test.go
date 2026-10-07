package ui

import (
	"os/exec"
	"testing"
)

func TestDaemonStatusI18n(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for the daemon status runtime check")
	}
	if out, err := exec.Command(node, "testdata/daemon-status-i18n.cjs").CombinedOutput(); err != nil {
		t.Fatalf("daemon status translations: %v\n%s", err, out)
	}
}
