package agent

import (
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func TestDispatchShellExecuteSandboxFailsClosedByDefault(t *testing.T) {
	oldSandboxExecuteCode := sandboxExecuteCodeFunc
	oldExecutePython := executePythonFunc
	oldExecutePythonWithSecrets := executePythonWithSecretsFunc
	defer func() {
		sandboxExecuteCodeFunc = oldSandboxExecuteCode
		executePythonFunc = oldExecutePython
		executePythonWithSecretsFunc = oldExecutePythonWithSecrets
	}()

	const secret = "SANDBOX_SECRET_SHOULD_NOT_LEAK_20260624"
	security.RegisterSensitive(secret)

	fallbackCalls := 0
	sandboxExecuteCodeFunc = func(code, language string, libraries []string, timeoutSecs int, logger *slog.Logger) (string, error) {
		if language != "python" {
			t.Fatalf("language = %q, want python", language)
		}
		return "", errors.New("container unavailable: " + secret)
	}
	executePythonFunc = func(code, workspaceDir, toolsDir string) (string, string, error) {
		fallbackCalls++
		return "local fallback ran", "", nil
	}

	cfg := &config.Config{}
	cfg.Sandbox.Enabled = true
	cfg.Agent.AllowPython = true
	cfg.Directories.WorkspaceDir = t.TempDir()
	cfg.Directories.ToolsDir = t.TempDir()

	out := dispatchShell(ToolCall{
		Action: "execute_sandbox",
		Code:   "print('hello')",
	}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	if fallbackCalls != 0 {
		t.Fatalf("local Python fallback calls = %d, want 0", fallbackCalls)
	}
	for _, want := range []string{"[EXECUTION ERROR]", "sandbox:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, secret) {
		t.Fatalf("expected sandbox error to be scrubbed, got:\n%s", out)
	}
	if strings.Contains(out, "local fallback ran") || strings.Contains(out, "ran via local Python") {
		t.Fatalf("expected fail-closed output without local Python fallback, got:\n%s", out)
	}
}

func TestDispatchShellExecuteSandboxFallbackRequiresExplicitOptIn(t *testing.T) {
	oldSandboxExecuteCode := sandboxExecuteCodeFunc
	oldExecutePython := executePythonFunc
	oldExecutePythonWithSecrets := executePythonWithSecretsFunc
	defer func() {
		sandboxExecuteCodeFunc = oldSandboxExecuteCode
		executePythonFunc = oldExecutePython
		executePythonWithSecretsFunc = oldExecutePythonWithSecrets
	}()

	fallbackCalls := 0
	sandboxExecuteCodeFunc = func(code, language string, libraries []string, timeoutSecs int, logger *slog.Logger) (string, error) {
		return "", errors.New("container unavailable")
	}
	executePythonFunc = func(code, workspaceDir, toolsDir string) (string, string, error) {
		fallbackCalls++
		return "local ok", "", nil
	}
	executePythonWithSecretsFunc = func(code, workspaceDir, toolsDir string, secrets map[string]string, creds []tools.CredentialFields) (string, string, error) {
		t.Fatal("did not expect secret-injecting fallback")
		return "", "", nil
	}

	cfg := &config.Config{}
	cfg.Sandbox.Enabled = true
	cfg.Sandbox.AllowLocalPythonFallback = true
	cfg.Agent.AllowPython = true
	cfg.Directories.WorkspaceDir = t.TempDir()
	cfg.Directories.ToolsDir = t.TempDir()

	out := dispatchShell(ToolCall{
		Action:   "execute_sandbox",
		Code:     "print('hello')",
		Language: "python",
	}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	if fallbackCalls != 1 {
		t.Fatalf("local Python fallback calls = %d, want 1", fallbackCalls)
	}
	for _, want := range []string{"ran via local Python", "STDOUT:", "local ok"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}
