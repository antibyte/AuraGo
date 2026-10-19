package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func TestDispatchShellExecuteSandboxNeverFallsBack(t *testing.T) {
	previous := sandboxExecuteCodeFunc
	t.Cleanup(func() { sandboxExecuteCodeFunc = previous })
	const secret = "sandbox-error-secret-fixture"
	security.RegisterSensitive(secret)
	for _, legacy := range []bool{false, true} {
		for _, failure := range []error{fmt.Errorf("not ready: %w", tools.ErrSandboxUnavailable), fmt.Errorf("TIMEOUT %s", secret)} {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			sandboxExecuteCodeFunc = func(got context.Context, code, lang string, libraries []string, timeout int, logger *slog.Logger) (string, error) {
				if got != ctx {
					t.Fatal("sandbox lost caller context")
				}
				return "", failure
			}
			cfg := &config.Config{}
			cfg.Sandbox.Enabled = true
			cfg.Sandbox.AllowLocalPythonFallback = legacy
			cfg.Agent.AllowPython = true
			cfg.Agent.AllowUnsafeHostExecution = true
			out := dispatchShellWithContext(ctx, ToolCall{Action: "execute_sandbox", Code: "print(1)", Language: "python"}, &DispatchContext{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
			if !strings.Contains(out, "[EXECUTION ERROR]") || strings.Contains(out, "ran via local Python") || strings.Contains(out, secret) {
				t.Fatalf("fallback or unsanitized output: %s", out)
			}
		}
	}
}
