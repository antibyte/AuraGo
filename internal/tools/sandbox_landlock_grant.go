package tools

import (
	"context"
	"fmt"
	"strings"

	"aurago/internal/sandbox"
)

const containerSandboxMCPName = "llm-sandbox"

type containerSandboxLandlockKey struct{}

type containerSandboxLandlock struct {
	workspace  string
	dockerHost string
}

func withContainerSandboxLandlock(ctx context.Context, workspace, dockerHost string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, containerSandboxLandlockKey{}, containerSandboxLandlock{
		workspace:  workspace,
		dockerHost: dockerHost,
	})
}

func containerSandboxLandlockFrom(ctx context.Context) (containerSandboxLandlock, bool) {
	if ctx == nil {
		return containerSandboxLandlock{}, false
	}
	grant, ok := ctx.Value(containerSandboxLandlockKey{}).(containerSandboxLandlock)
	return grant, ok
}

// applyContainerSandboxLandlockGrant opens the Docker unix socket and the
// agent workspace for the llm-sandbox MCP process after its Landlock
// environment exists. The workspace stays read-only. Docker tool permission
// is not consulted: this process is the container sandbox. Any other MCP
// name is unchanged.
func applyContainerSandboxLandlockGrant(name string, env []string, workspaceDir, sandboxDockerHost string) []string {
	if name != containerSandboxMCPName || !shellEnvHasKey(env, "AURAGO_SBX_RW") {
		return env
	}
	out := append([]string{}, env...)
	host := containerSandboxSocketHost(sandboxDockerHost)
	if host != "" && !shellEnvHasKey(out, "DOCKER_HOST") {
		out = append(out, "DOCKER_HOST="+host)
	}
	out = sandbox.AddUnixSocketGrant(out)
	return sandbox.AddRequiredReadOnlyGrant(out, workspaceDir)
}

// containerSandboxSocketHost is sandbox.docker_host when that value is
// unix://. A tcp:// sandbox host adds no filesystem path. An empty sandbox
// host uses the endpoint published by SetShellSandboxDockerHost, which main
// records only when Docker is enabled.
func containerSandboxSocketHost(sandboxDockerHost string) string {
	configured := strings.TrimSpace(sandboxDockerHost)
	switch {
	case strings.HasPrefix(configured, "unix://"):
		return configured
	case strings.HasPrefix(configured, "tcp://"):
		return configured
	case configured == "":
		return strings.TrimSpace(currentShellSandboxDockerHost())
	default:
		return ""
	}
}

func sandboxNotInstalledStatus(backend string) string {
	req, err := llmSandboxRequirement(backend)
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("llm-sandbox MCP extras not installed (set sandbox.auto_install: true or run: pip install '%s')", req)
}
