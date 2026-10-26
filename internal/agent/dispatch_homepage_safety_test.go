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
	"aurago/internal/tools"
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

// TestDispatchServicesRegistersVercelTokenForScrubbing proves the Vercel token
// read from the vault is registered for scrubbing, so it can never reach an
// outgoing text.
func TestDispatchServicesRegistersVercelTokenForScrubbing(t *testing.T) {
	const token = "vercel-token-scrub-fixture-0123456789"
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.AllowDeploy = true
	cfg.Homepage.WorkspacePath = t.TempDir()
	cfg.Vercel.Enabled = true
	cfg.Vercel.AllowDeploy = true
	cfg.Vercel.DefaultProjectID = "site-a"

	db, err := tools.InitHomepageRegistryDB(filepath.Join(t.TempDir(), "homepage.db"))
	if err != nil {
		t.Fatalf("InitHomepageRegistryDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := tools.EnsureHomepageProjectForDir(db, tools.HomepageConfig{WorkspacePath: cfg.Homepage.WorkspacePath}, "site-a", "site-a", "html"); err != nil {
		t.Fatalf("EnsureHomepageProjectForDir failed: %v", err)
	}
	vault := newDispatchTestVault(t, map[string]string{"vercel_token": token})

	out, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "homepage",
		Operation: "deploy_vercel",
		Params: map[string]interface{}{
			"project_dir": "site-a",
			"build_dir":   ".",
			// A mutating homepage operation requires a reason; without it the
			// dispatch stops before the Vercel token is ever read.
			"reason": "verify that the Vercel token is registered for scrubbing",
		},
	}, &DispatchContext{
		Cfg:                cfg,
		Logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		Vault:              vault,
		HomepageRegistryDB: db,
	})
	if !ok {
		t.Fatal("expected dispatchServices to handle the homepage deploy_vercel operation")
	}
	if strings.Contains(out, "reason is required") {
		t.Fatalf("the deploy_vercel dispatch must reach the Vercel token read, got: %s", out)
	}

	scrubbed := security.Scrub("surrounding text " + token + " more text")
	if strings.Contains(scrubbed, token) {
		t.Fatalf("the Vercel token must be registered for scrubbing, got: %s", scrubbed)
	}
	if !strings.Contains(scrubbed, "surrounding text") {
		t.Fatalf("expected the surrounding text to survive scrubbing, got: %s", scrubbed)
	}
}
