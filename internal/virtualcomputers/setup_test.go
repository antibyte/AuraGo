package virtualcomputers

import (
	"context"
	"strings"
	"testing"
)

type fakeSSHExecutor struct {
	output string
	err    error
}

func (f fakeSSHExecutor) Run(ctx context.Context, command string) (string, error) {
	return f.output, f.err
}

func TestSetupPreflightParsesUnsupportedHost(t *testing.T) {
	manager := SetupManager{Executor: fakeSSHExecutor{output: "ARCH=aarch64\nHAS_KVM=0\nOS_ID=debian\n"}}
	result, err := manager.Preflight(context.Background())
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if result.Supported {
		t.Fatal("expected unsupported result")
	}
	want := []string{"unsupported architecture", "KVM is not available", "Ubuntu is required"}
	for _, part := range want {
		if !strings.Contains(strings.Join(result.Issues, "; "), part) {
			t.Fatalf("issues %v missing %q", result.Issues, part)
		}
	}
}

func TestSetupInstallLogRedactsSecrets(t *testing.T) {
	manager := SetupManager{}
	log := manager.RedactInstallLog("export BORING_TOKEN=super-secret\nANTHROPIC_API_KEY=abc\nok")
	for _, leaked := range []string{"super-secret", "abc"} {
		if strings.Contains(log, leaked) {
			t.Fatalf("redacted log leaked %q: %s", leaked, log)
		}
	}
	if !strings.Contains(log, "<redacted>") {
		t.Fatalf("redacted log should contain marker: %s", log)
	}
}
