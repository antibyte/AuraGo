package tools

import (
	"os"
	"os/exec"
	"strings"

	"aurago/internal/sandbox"
)

// ensureFilteredEnv prevents child processes from inheriting host secrets by default.
// Integrations that drive Docker themselves (Ansible local runs, MCP stdio,
// image builds) use it and keep the Docker endpoint variables.
func ensureFilteredEnv(cmd *exec.Cmd) {
	if cmd == nil || cmd.Env != nil {
		return
	}
	cmd.Env = sandbox.FilterEnv(os.Environ())
}

// dockerClientEnvNames select the Docker Engine endpoint and its credentials.
// Operator shells and host Python only see them while the Docker tool is
// permitted, mirroring the Landlock ExtraEnv rule in cmd/aurago/main.go that
// passes DOCKER_HOST into the sandbox only when Docker is enabled. Names are
// upper case; lookups upper-case the variable name like sandbox.FilterEnv.
var dockerClientEnvNames = map[string]bool{
	"DOCKER_HOST":        true,
	"DOCKER_CONFIG":      true,
	"DOCKER_CERT_PATH":   true,
	"DOCKER_TLS_VERIFY":  true,
	"DOCKER_CONTEXT":     true,
	"DOCKER_API_VERSION": true,
}

// filteredShellEnv is the environment for operator shells and host Python:
// sandbox.FilterEnv without the Docker client variables unless the Docker
// tool is permitted.
func filteredShellEnv() []string {
	env := sandbox.FilterEnv(os.Environ())
	if requireDockerPermission() == nil {
		return env
	}
	return withoutDockerClientEnv(env)
}

// withoutDockerClientEnv returns env without dockerClientEnvNames entries,
// leaving env itself unchanged.
func withoutDockerClientEnv(env []string) []string {
	kept := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if !dockerClientEnvNames[strings.ToUpper(strings.TrimSpace(name))] {
			kept = append(kept, kv)
		}
	}
	return kept
}

// ensureFilteredShellEnv applies filteredShellEnv unless the caller set cmd.Env.
func ensureFilteredShellEnv(cmd *exec.Cmd) {
	if cmd == nil || cmd.Env != nil {
		return
	}
	cmd.Env = filteredShellEnv()
}
