package agent

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func newDispatchTestVault(t *testing.T, secrets map[string]string) *security.Vault {
	t.Helper()
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault failed: %v", err)
	}
	for key, value := range secrets {
		if err := vault.WriteSecret(key, value); err != nil {
			t.Fatalf("WriteSecret(%q) failed: %v", key, err)
		}
	}
	return vault
}

func TestDispatchServicesHomepageExecRejectsEmptyCommandWithGuidance(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.AllowContainerManagement = true

	out, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "homepage",
		Operation: "exec",
		Params: map[string]interface{}{
			"command": "   ",
		},
	}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("expected dispatchServices to handle homepage")
	}
	for _, want := range []string{"command is required", "Do not retry", "list_files", "build"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected homepage exec empty-command guidance to contain %q, got:\n%s", want, out)
		}
	}
}

func TestHomepageExecEnvForCommandDoesNotExposeDeployProviderTokens(t *testing.T) {
	vault := newDispatchTestVault(t, map[string]string{
		"netlify_token": "nf-token-fixture",
		"vercel_token":  "vc-token-fixture",
	})

	env := homepageExecEnvForCommand(vault, "netlify deploy && vercel deploy")
	joined := strings.Join(env, "\n")

	if strings.Contains(joined, "NETLIFY_AUTH_TOKEN") || strings.Contains(joined, "nf-token-fixture") {
		t.Fatalf("homepage exec must not expose the Netlify vault token, got env %#v", env)
	}
	if strings.Contains(joined, "VERCEL_TOKEN") || strings.Contains(joined, "vc-token-fixture") {
		t.Fatalf("homepage exec must not expose the Vercel vault token, got env %#v", env)
	}
}
