package agent

import (
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/tools"
)

func TestManagedLocalLLMDockerResourcesAreNeverAgentAccessible(t *testing.T) {
	for _, operation := range []string{
		"inspect", "stats", "port", "start", "stop", "restart", "remove",
		"logs", "exec", "top", "cp", "copy",
	} {
		if localLLMDockerOperationSafe(operation) {
			t.Fatalf("target-specific operation %q was allowed", operation)
		}
	}
	for _, name := range []string{"aurago_models", "AURAGO_LOCAL_LLM_RUNTIME"} {
		if !dockerProtectedLocalLLMVolumeName(name) {
			t.Fatalf("protected volume %q was not recognized", name)
		}
	}
	if !dockerRequestMountsProtectedLocalLLMVolume([]string{"aurago_models:/models:ro"}) {
		t.Fatal("protected model volume mount was allowed")
	}
	if dockerRequestMountsProtectedLocalLLMVolume([]string{"ordinary-data:/data"}) {
		t.Fatal("ordinary volume was treated as protected")
	}
	if !tools.DockerContainerManagedBy(tools.DockerConfig{}, "aurago-local-llm", "local-llm") {
		t.Fatal("reserved container name was not blocked when inspect is unavailable")
	}
}

func TestManagedLocalLLMComposeProtectionIsFailClosed(t *testing.T) {
	workspace := t.TempDir()
	originalResolver := resolveDockerComposeConfig
	t.Cleanup(func() { resolveDockerComposeConfig = originalResolver })
	resolveDockerComposeConfig = func(_ tools.DockerConfig, file string) (string, error) {
		payload, err := os.ReadFile(filepath.Join(workspace, filepath.Base(file)))
		return string(payload), err
	}
	protected := filepath.Join(workspace, "protected.yml")
	if err := os.WriteFile(protected, []byte("services:\n  app:\n    volumes:\n      - aurago_models:/models\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	safe := filepath.Join(workspace, "safe.yml")
	if err := os.WriteFile(safe, []byte("services:\n  app:\n    image: example.invalid/app\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := tools.DockerConfig{WorkspaceDir: workspace}
	if !dockerComposeReferencesProtectedLocalLLMVolume(cfg, filepath.Base(protected)) {
		t.Fatal("Compose reference to protected volume was not blocked")
	}
	if dockerComposeReferencesProtectedLocalLLMVolume(cfg, filepath.Base(safe)) {
		t.Fatal("safe Compose file was blocked")
	}
	resolveDockerComposeConfig = func(_ tools.DockerConfig, _ string) (string, error) {
		return `{"services":{"app":{"labels":{"aurago.managed": "local-llm"}}}}`, nil
	}
	if !dockerComposeReferencesProtectedLocalLLMVolume(cfg, filepath.Base(safe)) {
		t.Fatal("interpolated canonical managed label was not blocked")
	}
	resolveDockerComposeConfig = func(_ tools.DockerConfig, _ string) (string, error) {
		return `{"services":{"app":{"volumes_from":["aurago-local-llm"]}}}`, nil
	}
	if !dockerComposeReferencesProtectedLocalLLMVolume(cfg, filepath.Base(safe)) {
		t.Fatal("interpolated volumes_from was not blocked")
	}
	resolveDockerComposeConfig = func(_ tools.DockerConfig, file string) (string, error) {
		payload, err := os.ReadFile(filepath.Join(workspace, filepath.Base(file)))
		return string(payload), err
	}
	for _, file := range []string{"missing.yml", filepath.Join("..", "outside.yml")} {
		if !dockerComposeReferencesProtectedLocalLLMVolume(cfg, file) {
			t.Fatalf("unreadable or out-of-workspace Compose file %q did not fail closed", file)
		}
	}
}
