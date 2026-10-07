package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

const recheckCheckedModel = `{"services":{"web":{"image":"alpine"}}}`

// recheckDispatch runs an agent `compose up -d` through dispatch with a
// resolver that answers the first `checked` resolutions with
// recheckCheckedModel and later ones with `later`; it returns the output and
// the number of resolutions.
func recheckDispatch(t *testing.T, allowHostAccess bool, later string, beforeLater func(workspace string)) (string, int) {
	t.Helper()
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  web:\n    image: alpine\n")
	calls := 0
	const checked = 2 // the default and the all-profiles resolution of the preflight
	stubDockerComposeResolverModes(t, func(_ context.Context, _ string, _ tools.DockerComposeConfigOptions) (string, error) {
		calls++
		if calls == checked && beforeLater != nil {
			beforeLater(workspace)
		}
		if calls > checked {
			return later, nil
		}
		return recheckCheckedModel, nil
	})
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "tcp://127.0.0.1:1"
	cfg.Docker.AllowHostAccess = allowHostAccess
	cfg.Directories.WorkspaceDir = workspace
	useRuntimePermissionsForTest(t, cfg)
	output, ok := dispatchServices(context.Background(), ToolCall{Action: "docker", Operation: "compose", File: "compose.yml", Command: "up -d"}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if !ok {
		t.Fatal("expected docker operation to be handled")
	}
	return output, calls
}

func TestDispatchDockerComposeRefusesInputThatChangedAfterTheCheck(t *testing.T) {
	changed := `{"services":{"web":{"image":"alpine","privileged":true,"volumes":[{"type":"bind","source":"/","target":"/host"}]}}}`
	output, calls := recheckDispatch(t, false, changed, nil)
	if !strings.Contains(output, `"code":"docker_compose_input_changed"`) {
		t.Fatalf("output = %s, want docker_compose_input_changed", output)
	}
	if calls != 3 {
		t.Fatalf("resolutions = %d, want the two checked ones and the repeat that found the change", calls)
	}
}

func TestDispatchDockerComposeRefusesAComposeFileRewrittenAfterTheCheck(t *testing.T) {
	rewrite := func(workspace string) {
		if err := os.WriteFile(filepath.Join(workspace, "compose.yml"), []byte("services:\n  web:\n    image: alpine\n    privileged: true\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	output, _ := recheckDispatch(t, false, recheckCheckedModel, rewrite)
	if !strings.Contains(output, `"code":"docker_compose_input_changed"`) {
		t.Fatalf("output = %s, want docker_compose_input_changed for a rewritten file", output)
	}
}

func TestDispatchDockerComposeRunsUnchangedInputAfterTheRecheck(t *testing.T) {
	output, calls := recheckDispatch(t, false, recheckCheckedModel, nil)
	if strings.Contains(output, "docker_compose_input_changed") || strings.Contains(output, "policy_denied") {
		t.Fatalf("unchanged input was refused: %s", output)
	}
	if calls != 4 {
		t.Fatalf("resolutions = %d, want the two checked ones repeated once each before the run", calls)
	}
}

// Grandfathered installs (docker.allow_host_access true) are unchanged: no
// second resolution, no new refusal.
func TestDispatchDockerComposeSkipsTheRecheckWithHostAccess(t *testing.T) {
	changed := `{"services":{"web":{"image":"alpine","privileged":true}}}`
	output, calls := recheckDispatch(t, true, changed, nil)
	if strings.Contains(output, "docker_compose_input_changed") {
		t.Fatalf("a grandfathered install got the recheck: %s", output)
	}
	if calls != 2 {
		t.Fatalf("resolutions = %d, want only the two of the preflight", calls)
	}
}

// A resolution that ran out of the shared deadline during the check proves
// nothing about the input; succeeding on the repeat is no change.
func TestDispatchDockerComposeIgnoresAResolutionThatTimedOutDuringTheCheck(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  web:\n    image: alpine\n")
	allProfiles := 0
	stubDockerComposeResolverModes(t, func(_ context.Context, _ string, opts tools.DockerComposeConfigOptions) (string, error) {
		if opts.AllProfiles {
			allProfiles++
			if allProfiles == 1 {
				return "", fmt.Errorf("resolve Compose config: %w", context.DeadlineExceeded)
			}
		}
		return recheckCheckedModel, nil
	})
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "tcp://127.0.0.1:1"
	cfg.Directories.WorkspaceDir = workspace
	useRuntimePermissionsForTest(t, cfg)
	output, _ := dispatchServices(context.Background(), ToolCall{Action: "docker", Operation: "compose", File: "compose.yml", Command: "up -d"}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if strings.Contains(output, "docker_compose_input_changed") {
		t.Fatalf("a check-time deadline gave a false input change: %s", output)
	}
	if allProfiles != 1 {
		t.Fatalf("all-profiles resolutions = %d, want the timed-out one only (not repeated)", allProfiles)
	}
}

// A named-service resolution that tools.DockerComposeResolvedConfigContext
// cut off at its own per-call limit (it now wraps context.DeadlineExceeded
// instead of "signal: killed") is not repeated either.
func TestDispatchDockerComposeIgnoresANamedResolutionKilledByItsTimeout(t *testing.T) {
	workspace := t.TempDir()
	writeComposeFixture(t, workspace, "compose.yml", "services:\n  web:\n    image: alpine\n")
	named := 0
	stubDockerComposeResolverModes(t, func(_ context.Context, _ string, opts tools.DockerComposeConfigOptions) (string, error) {
		switch {
		case opts.AllProfiles:
			return "", errors.New("resolve Compose config: exit status 16: unknown flag: --no-env-resolution")
		case len(opts.Services) > 0:
			named++
			if named == 1 {
				return "", fmt.Errorf("resolve Compose config: %w", context.DeadlineExceeded)
			}
			return `{"services":{"hidden":{"image":"alpine"}}}`, nil
		}
		return recheckCheckedModel, nil
	})
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "tcp://127.0.0.1:1"
	cfg.Directories.WorkspaceDir = workspace
	useRuntimePermissionsForTest(t, cfg)
	output, _ := dispatchServices(context.Background(), ToolCall{Action: "docker", Operation: "compose", File: "compose.yml", Command: "config hidden"}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if strings.Contains(output, "docker_compose_input_changed") {
		t.Fatalf("a killed named resolution gave a false input change: %s", output)
	}
	if named != 1 {
		t.Fatalf("named resolutions = %d, want the timed-out one only (not repeated)", named)
	}
}
