package agent

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchSudoDeniesRunWithoutShellBeforeVaultLookup(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.SudoEnabled = true
	got := dispatchShellWithContext(context.Background(), ToolCall{
		Action: "execute_sudo",
		Params: map[string]interface{}{"command": "id"},
	}, &DispatchContext{Cfg: cfg})
	if !strings.Contains(got, "agent.allow_shell") || !strings.Contains(got, "permission_denied") {
		t.Fatalf("sudo output=%q, want shell permission denial", got)
	}
}

func TestDispatchPackageManagerMutationsForwardCancellation(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	cfg := enabledPackageManagerConfig()
	cfg.PackageManager.Override = "brew"
	useRuntimePermissionsForTest(t, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, operation := range []string{"install", "remove", "update", "upgrade"} {
		tc := packageManagerCall(operation)
		tc.Params["package"] = "test-only"
		got, handled := dispatchExec(ctx, tc, &DispatchContext{Cfg: cfg})
		if !handled || !strings.Contains(got, "context canceled") {
			t.Fatalf("%s output=%q, want caller cancellation", operation, got)
		}
	}
}
