package onnxrt

import (
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
)

func TestRuntimeEnvironmentPrependsPrivateDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "lib"), 0o750); err != nil {
		t.Fatal(err)
	}
	environment := runtimeEnvironment(goruntime.GOOS, root)
	pathValue := environmentValue(environment, "PATH")
	if !strings.Contains(pathValue, root) {
		t.Fatalf("PATH %q does not contain runtime root", pathValue)
	}
	for key, value := range map[string]string{
		"AURAGO_MASTER_KEY":   "master-secret",
		"OPENAI_API_KEY":      "api-secret",
		"CUSTOM_AUTH_TOKEN":   "token-secret",
		"MY_DB_PASSWORD":      "password-secret",
		"PROJECT_CREDENTIALS": "credentials-secret",
	} {
		t.Setenv(key, value)
	}
	environment = runtimeEnvironment(goruntime.GOOS, root)
	for _, entry := range environment {
		upper := strings.ToUpper(entry)
		for _, forbidden := range []string{
			"AURAGO_MASTER_KEY=",
			"OPENAI_API_KEY=",
			"CUSTOM_AUTH_TOKEN=",
			"MY_DB_PASSWORD=",
			"PROJECT_CREDENTIALS=",
		} {
			if strings.HasPrefix(upper, forbidden) {
				t.Fatalf("sensitive environment variable escaped into native process: %s", forbidden)
			}
		}
	}
}

func TestRuntimeEnvironmentSetsLibraryPathPerPlatform(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LD_LIBRARY_PATH", "/opt/keep-ld")
	t.Setenv("DYLD_LIBRARY_PATH", "/opt/keep-dyld")
	linux := runtimeEnvironment("linux", root)
	if value := environmentValue(linux, "LD_LIBRARY_PATH"); !strings.HasPrefix(value, root) || !strings.HasSuffix(value, "/opt/keep-ld") {
		t.Fatalf("linux LD_LIBRARY_PATH = %q", value)
	}
	if value := environmentValue(linux, "DYLD_LIBRARY_PATH"); value != "/opt/keep-dyld" {
		t.Fatalf("linux DYLD_LIBRARY_PATH = %q", value)
	}
	darwin := runtimeEnvironment("darwin", root)
	if value := environmentValue(darwin, "DYLD_LIBRARY_PATH"); !strings.HasPrefix(value, root) {
		t.Fatalf("darwin DYLD_LIBRARY_PATH = %q", value)
	}
	windows := runtimeEnvironment("windows", root)
	if value := environmentValue(windows, "LD_LIBRARY_PATH"); value != "/opt/keep-ld" {
		t.Fatalf("windows LD_LIBRARY_PATH = %q", value)
	}
}

func TestConfigureWorkerCommandWithoutRootsKeepsPath(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "api-secret")
	command := exec.Command("aurago-test-worker")
	ConfigureWorkerCommand(command)
	if got, want := environmentValue(command.Env, "PATH"), os.Getenv("PATH"); got != want {
		t.Fatalf("PATH = %q, want unchanged %q", got, want)
	}
	for _, entry := range command.Env {
		if strings.HasPrefix(strings.ToUpper(entry), "OPENAI_API_KEY=") {
			t.Fatal("sensitive variable reached the worker environment")
		}
	}
}

func TestConfigureWorkerCommandPrependsRuntimeRoot(t *testing.T) {
	root := t.TempDir()
	command := exec.Command("aurago-test-worker")
	ConfigureWorkerCommand(command, root)
	if value := environmentValue(command.Env, "PATH"); !strings.HasPrefix(value, root) {
		t.Fatalf("PATH = %q, want prefix %q", value, root)
	}
}

// exec.Cmd treats a nil Env as "inherit everything", secrets included, so a
// filtered environment that ends up empty must stay a non-nil empty slice.
func TestFilterEnvironmentNeverFallsBackToInheritance(t *testing.T) {
	filtered := filterEnvironment([]string{"OPENAI_API_KEY=api-secret", "AURAGO_MASTER_KEY=master-secret"})
	if filtered == nil || len(filtered) != 0 {
		t.Fatalf("filtered = %#v, want a non-nil empty slice", filtered)
	}
	if got := filterEnvironment([]string{"KEEP=1", "OPENAI_API_KEY=api-secret"}); len(got) != 1 || got[0] != "KEEP=1" {
		t.Fatalf("filtered = %#v", got)
	}
}

func TestAccessibleRenderNodeRequiresUsableRenderNode(t *testing.T) {
	driRoot := t.TempDir()
	if _, err := AccessibleRenderNode(driRoot); err == nil {
		t.Fatal("empty DRI directory unexpectedly enabled Vulkan")
	}

	renderDevice := filepath.Join(driRoot, "renderD128")
	if err := os.WriteFile(renderDevice, []byte("test render device"), 0o600); err != nil {
		t.Fatal(err)
	}
	found, err := AccessibleRenderNode(driRoot)
	if err != nil {
		t.Fatalf("accessible render device was rejected: %v", err)
	}
	if found != renderDevice {
		t.Fatalf("render device = %q, want %q", found, renderDevice)
	}
}

// The probe's error text is part of the embeddings hardware fingerprint.
func TestAccessibleRenderNodeKeepsEmbeddingsErrorText(t *testing.T) {
	driRoot := t.TempDir()
	_, err := AccessibleRenderNode(driRoot)
	if err == nil || err.Error() != "no Vulkan render device found under "+driRoot {
		t.Fatalf("error = %v", err)
	}
}
