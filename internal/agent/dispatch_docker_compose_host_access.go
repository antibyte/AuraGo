package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"aurago/internal/config"
	"aurago/internal/tools"
)

// dockerComposeHostAccessAllowed reads docker.allow_host_access from the run's
// already intersected config and the run's runtime permissions; a run can
// narrow but never widen the server snapshot.
func dockerComposeHostAccessAllowed(ctx context.Context, cfg *config.Config) bool {
	if cfg == nil || !cfg.Docker.AllowHostAccess {
		return false
	}
	perms, configured := tools.EffectiveRuntimePermissions(ctx)
	return !configured || perms.AllowDockerHostAccess
}

// dockerComposeHostAccessScope maps a Compose subcommand to the scope of the
// host-access policy. Every other subcommand (down, stop, start, restart, rm,
// kill, pause, unpause, pull, ps, logs, …) is never checked by it.
func dockerComposeHostAccessScope(subcommand string) (tools.DockerComposeHostAccessScope, bool) {
	switch subcommand {
	case "up", "create":
		return tools.DockerComposeScopeRun, true
	case "build":
		return tools.DockerComposeScopeBuild, true
	case "config", "convert":
		return tools.DockerComposeScopeRender, true
	}
	return 0, false
}

// dockerComposeMasterKey is the master key the policy protects: the run
// config's value, or the process environment Compose inherits.
func dockerComposeMasterKey(cfg *config.Config) string {
	if cfg != nil {
		if key := strings.TrimSpace(cfg.Server.MasterKey); key != "" {
			return key
		}
	}
	return strings.TrimSpace(os.Getenv(tools.DockerComposeMasterKeyVariable))
}

// dockerComposeHostAccessPolicy checks up/create (everything), build (build
// sections) and config/convert (AuraGo state only) on the command's effective
// model, with the preflight's jail root as the workspace. It runs after the
// ownership checks and the unverified-profile denial.
func dockerComposeHostAccessPolicy(ctx context.Context, cfg *config.Config, req dockerArgs, preflight *dockerComposePreflight, effective dockerComposeEffectiveModel) string {
	scope, checked := dockerComposeHostAccessScope(tools.DockerComposeSubcommand(req.Command))
	if !checked {
		return ""
	}
	roots, files := tools.DockerComposeProtectedPaths(cfg)
	policy := tools.DockerComposeHostPolicy{
		WorkspaceDir:    preflight.root,
		AllowHostAccess: dockerComposeHostAccessAllowed(ctx, cfg),
		ProtectedRoots:  roots,
		ProtectedFiles:  files,
		MasterKey:       dockerComposeMasterKey(cfg),
	}
	var envFiles []tools.DockerComposeEnvFile
	envFilesUnknown := false
	if effective.fromAllProfiles {
		envFiles = tools.DockerComposeServiceEnvFiles(effective.model, filepath.Dir(preflight.file))
	} else {
		// Compose without the all-profiles model (< v2.35) inlines env files and
		// drops their paths. Only the main file's text can tell they exist.
		envFilesUnknown = scope == tools.DockerComposeScopeRun && !policy.AllowHostAccess && strings.Contains(preflight.raw, "env_file")
	}
	violations := dockerComposeCommandViolations(req.Command, scope, policy)
	violations = append(violations, tools.EvaluateDockerComposeHostAccess(effective.model, envFiles, scope, policy)...)
	if scope != tools.DockerComposeScopeBuild && !dockerComposeHasAlwaysViolation(violations) &&
		(tools.DockerComposeTextCarriesMasterKey(preflight.resolved, policy.MasterKey) ||
			tools.DockerComposeTextCarriesMasterKey(effective.profileText, policy.MasterKey)) {
		// The default model inlines env files into environment, so a copy of
		// the key in any env file or interpolated field shows up here.
		violations = append(violations, tools.DockerComposeViolation{Field: "resolved model",
			Reason: "AuraGo's master key value appears in the resolved Compose file (for example through an env_file or an interpolated variable)", Always: true})
	}
	sort.SliceStable(violations, func(i, j int) bool { return violations[i].Always && !violations[j].Always })
	if len(violations) > 0 {
		return dockerComposeViolationOutput(violations)
	}
	if envFilesUnknown {
		return dockerComposeDenied("docker_compose_host_access_denied",
			"Docker Compose stack rejected before anything ran: this Docker Compose version cannot list env_file paths, so AuraGo cannot check that they stay inside the agent workspace (that needs Docker Compose v2.35 or newer). Update the Docker Compose plugin, move the variables into the Compose file, or enable Config → Danger Zone → \"Docker host access for agent Compose stacks\" (docker.allow_host_access). Do not retry unchanged.", nil)
	}
	return ""
}

// dockerComposeCommandViolations checks the build flags that reach the host
// directly: `--ssh` forwards the host's SSH agent or keys (host-access tier)
// and `--build-arg NAME` without a value copies NAME from AuraGo's process
// environment, so it must never name the master key (always tier).
func dockerComposeCommandViolations(command string, scope tools.DockerComposeHostAccessScope, policy tools.DockerComposeHostPolicy) []tools.DockerComposeViolation {
	if scope != tools.DockerComposeScopeBuild {
		return nil
	}
	parts := strings.Fields(command)
	var violations []tools.DockerComposeViolation
	for i := 1; i < len(parts); i++ {
		if parts[i] == "--" {
			break
		}
		flag, value, inline := strings.Cut(parts[i], "=")
		if flag != "--build-arg" && flag != "--ssh" {
			continue
		}
		if !inline && i+1 < len(parts) {
			i++
			value = parts[i]
		}
		switch flag {
		case "--build-arg":
			name, argValue, hasValue := strings.Cut(value, "=")
			if (!hasValue && strings.EqualFold(strings.TrimSpace(name), tools.DockerComposeMasterKeyVariable)) ||
				(hasValue && tools.DockerComposeTextCarriesMasterKey(argValue, policy.MasterKey)) {
				violations = append(violations, tools.DockerComposeViolation{Field: "--build-arg", Value: strings.TrimSpace(name),
					Reason: "AuraGo's master key cannot be passed to agent Compose builds", Always: true})
			}
		case "--ssh":
			if !policy.AllowHostAccess {
				violations = append(violations, tools.DockerComposeViolation{Field: "--ssh", Value: value,
					Reason: "the build gets the host's SSH agent or SSH keys"})
			}
		}
	}
	return violations
}

func dockerComposeHasAlwaysViolation(violations []tools.DockerComposeViolation) bool {
	for _, violation := range violations {
		if violation.Always {
			return true
		}
	}
	return false
}

// dockerComposeDenied builds a policy_denied envelope, which
// classifyLegacyToolResult reports as ToolResultDenied (never a success).
func dockerComposeDenied(code, message string, violations []tools.DockerComposeViolation) string {
	payload := map[string]interface{}{"status": "policy_denied", "code": code, "message": message}
	if len(violations) > 0 {
		payload["violations"] = violations
	}
	data, _ := json.Marshal(payload)
	return "Tool Output: " + string(data)
}

func dockerComposeViolationOutput(violations []tools.DockerComposeViolation) string {
	code := "docker_compose_host_access_denied"
	hint := "Enable Config → Danger Zone → \"Docker host access for agent Compose stacks\" (docker.allow_host_access) for stacks that need host paths, devices, privileged mode, host namespaces or other host access, or keep the stack's files inside the agent workspace. Do not retry unchanged."
	if violations[0].Always {
		code = "docker_compose_protected_path_denied"
		hint = "AuraGo's own data directory, configuration, .env and master key can never be used by agent Compose stacks, even with docker.allow_host_access. Do not retry unchanged."
	}
	shown := violations
	if len(shown) > 6 {
		shown = shown[:6]
	}
	parts := make([]string, 0, len(shown))
	for _, violation := range shown {
		parts = append(parts, violation.String())
	}
	message := "Docker Compose stack rejected before anything ran: " + strings.Join(parts, "; ")
	if len(violations) > len(shown) {
		message += fmt.Sprintf("; and %d more", len(violations)-len(shown))
	}
	return dockerComposeDenied(code, message+". "+hint, violations)
}
