//go:build windows

package tools

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NTFS names a file or directory through stream syntax (`config.yaml::$DATA`,
// `data::$INDEX_ALLOCATION\x`): no name comparison recognises these spellings
// as the protected file, but Windows opens the protected file or directory.
var ntfsStreamTargets = []string{
	"config.yaml::$DATA",
	".env::$DATA",
	"data::$INDEX_ALLOCATION\\x.yml",
	"data::$INDEX_ALLOCATION\\short_term.db",
	"data:$I30:$INDEX_ALLOCATION\\x.yml",
	"rendered\\stack.yml:stream",
	"rendered\\stack.yml:stream:$DATA",
}

func TestDockerComposeOutputRefusesNTFSStreamTargets(t *testing.T) {
	workspace := t.TempDir()
	dataDir := filepath.Join(workspace, "data")
	if err := os.MkdirAll(filepath.Join(workspace, "rendered"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(workspace, "config.yaml")
	protectAuraGoStateForTest(t, dataDir, configPath, "")
	for _, cfg := range []DockerConfig{{WorkspaceDir: workspace}, {}} {
		if cfg.WorkspaceDir == "" {
			t.Chdir(workspace) // no workspace configured: the working directory is the jail
		}
		for _, target := range ntfsStreamTargets {
			for _, command := range []string{"config -o " + target, "config --output=" + target, "config -qo" + target} {
				plan, err := planComposeOutput(t, cfg, command)
				var denied *dockerComposeDeniedError
				if err == nil || !errors.As(err, &denied) || denied.code != dockerComposeOutputDeniedCode {
					t.Fatalf("workspace %q: %s: accepted as %+v (err %v), want a coded denial", cfg.WorkspaceDir, command, plan, err)
				}
			}
		}
	}
}

func TestDockerComposeOutputPublishNeverWritesThroughNTFSStreams(t *testing.T) {
	// Defence in depth: even a plan that got past validation cannot write a
	// stream of a protected file, because the rooted writer refuses ':' names.
	workspace := t.TempDir()
	resolvedWorkspace, err := secureResolveFinalPath(workspace)
	if err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(resolvedWorkspace, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	protected := map[string]string{
		filepath.Join(resolvedWorkspace, "config.yaml"):      "config",
		filepath.Join(resolvedWorkspace, ".env"):             "secret",
		filepath.Join(dataDir, "short_term.db"):              "sqlite",
		filepath.Join(resolvedWorkspace, "rendered-ok.yaml"): "keep",
	}
	for path, content := range protected {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	staged := filepath.Join(t.TempDir(), "out")
	if err := os.WriteFile(staged, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, rel := range ntfsStreamTargets {
		plan := dockerComposeOutputPlan{root: resolvedWorkspace, rel: rel, target: filepath.Join(resolvedWorkspace, rel)}
		if published, err := plan.publish(staged); err == nil || published {
			t.Fatalf("%s: published = %v, err = %v, want a refusal", rel, published, err)
		}
	}
	for path, content := range protected {
		if data, err := os.ReadFile(path); err != nil || string(data) != content {
			t.Fatalf("%s changed to %q (%v)", path, data, err)
		}
	}
	entries, err := os.ReadDir(resolvedWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".aurago_edit_") {
			t.Fatalf("temporary file %s left behind", entry.Name())
		}
	}
}

func TestDockerComposeOutputRefusesReservedWindowsNamesWhilePlanning(t *testing.T) {
	cfg := DockerConfig{WorkspaceDir: t.TempDir()}
	for _, command := range []string{"config -o CON", `config -o rendered\COM1`, "config --output=NUL"} {
		_, err := planComposeOutput(t, cfg, command)
		var denied *dockerComposeDeniedError
		if !errors.As(err, &denied) || denied.code != dockerComposeOutputDeniedCode {
			t.Fatalf("%s: error %v, want a coded output denial while planning", command, err)
		}
	}
}
