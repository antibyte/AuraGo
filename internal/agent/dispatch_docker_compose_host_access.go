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
// host-access policy. kill, pause, unpause, ps, logs and the other
// inspection commands are never checked by it.
func dockerComposeHostAccessScope(subcommand string) (tools.DockerComposeHostAccessScope, bool) {
	switch subcommand {
	case "up", "create":
		return tools.DockerComposeScopeRun, true
	case "build":
		return tools.DockerComposeScopeBuild, true
	case "config", "convert":
		return tools.DockerComposeScopeRender, true
	case "pull":
		return tools.DockerComposeScopePull, true
	case "down", "start", "stop", "restart", "rm":
		// rm -s stops the containers first (pre_stop hooks, providers).
		return tools.DockerComposeScopeLifecycle, true
	}
	return 0, false
}

// dockerComposeLifecycleModel adds the inactive-profile services a lifecycle
// command names, with the services Compose enables together with them
// (depends_on and `service:` build contexts, as effectiveModel follows them),
// to the model the host-program check reads. The definitions come from the
// all-profiles model; without it (Compose < v2.35) named profile services
// cannot be checked here.
func dockerComposeLifecycleModel(effective tools.DockerComposeModel, preflight *dockerComposePreflight, command string) tools.DockerComposeModel {
	names := dockerComposeLifecycleServiceNames(command)
	all := preflight.allProfilesModel
	if all == nil || len(names) == 0 {
		return effective
	}
	model := effective
	model.Services = make(map[string]tools.DockerComposeService, len(effective.Services)+len(names))
	for name, service := range effective.Services {
		model.Services[name] = service
	}
	for _, name := range preflight.activatedProfileServices(names) {
		if _, present := model.Services[name]; present {
			continue
		}
		model.Services[name] = all.Services[name]
	}
	return model
}

// dockerComposeMasterKey is the master key the policy protects: the run
// config's value, or AuraGo's process environment.
func dockerComposeMasterKey(cfg *config.Config) string {
	if cfg != nil {
		if key := strings.TrimSpace(cfg.Server.MasterKey); key != "" {
			return key
		}
	}
	return strings.TrimSpace(os.Getenv(tools.DockerComposeMasterKeyVariable))
}

// dockerComposeHostAccessPolicy checks the command's effective model with the
// preflight's jail root as the workspace: up/create everything, build the
// build sections, config/convert AuraGo state only, pull the master key and
// host programs, down/start/stop/restart/rm host programs only (including the
// profile services they name). It runs after the ownership checks and the
// unverified-profile denial.
func dockerComposeHostAccessPolicy(ctx context.Context, cfg *config.Config, req dockerArgs, preflight *dockerComposePreflight, effective dockerComposeEffectiveModel) string {
	scope, checked := dockerComposeHostAccessScope(tools.DockerComposeSubcommand(req.Command))
	if !checked {
		return ""
	}
	roots, files := tools.DockerComposeProtectedPaths(cfg)
	// In a container AuraGo's data directory is also a host directory (a bind
	// at /app/data, or the directory behind its volume); its host spelling is
	// AuraGo state too.
	roots = append(roots, preflight.self.StateHostRoots()...)
	policy := tools.DockerComposeHostPolicy{
		WorkspaceDir:    preflight.root,
		AllowHostAccess: dockerComposeHostAccessAllowed(ctx, cfg),
		ProtectedRoots:  roots,
		ProtectedFiles:  files,
		MasterKey:       dockerComposeMasterKey(cfg),
	}
	envFilesKnown := false
	var envFiles []tools.DockerComposeEnvFile
	if effective.fromAllProfiles {
		envFiles, envFilesKnown = tools.DockerComposeServiceEnvFiles(effective.model, filepath.Dir(preflight.file))
	} else {
		// Compose without the all-profiles model (< v2.35) inlines env files and
		// drops their paths; only the main file's text can tell they may exist,
		// directly or through an included or extended file.
		envFilesKnown = !dockerComposeMayUseEnvFiles(preflight.raw)
	}
	envFilesUnknown := !envFilesKnown && scope == tools.DockerComposeScopeRun && !policy.AllowHostAccess
	model := effective.model
	if scope == tools.DockerComposeScopeLifecycle && !policy.AllowHostAccess {
		model = dockerComposeLifecycleModel(model, preflight, req.Command)
	}
	violations := dockerComposeCommandViolations(req.Command, scope, policy)
	evaluated := tools.EvaluateDockerComposeHostAccess(model, envFiles, scope, policy)
	narrowed := false
	if !policy.AllowHostAccess && scope != tools.DockerComposeScopeRender {
		// A command that names services runs only them and what they need
		// (and, but for build and pull, what depends on them); host-tier
		// findings of other services are not what it runs.
		if closure, ok := preflight.namedServiceClosure(model, req.Command); ok {
			evaluated = dockerComposeKeepClosureViolations(evaluated, closure)
			narrowed = true
		}
	}
	violations = append(violations, evaluated...)
	if scope != tools.DockerComposeScopeLifecycle && !dockerComposeHasAlwaysViolation(violations) &&
		(tools.DockerComposeLowerTextCarriesMasterKey(preflight.resolved, policy.MasterKey) ||
			tools.DockerComposeLowerTextCarriesMasterKey(effective.profileText, policy.MasterKey)) {
		// The default and the named resolutions inline env files, so a copy of
		// the key in any env file or interpolated field shows up here.
		violations = append(violations, tools.DockerComposeViolation{Field: "resolved model",
			Reason: "AuraGo's master key value appears in the resolved Compose file (for example through an env_file or an interpolated variable)", Always: true})
	}
	sort.SliceStable(violations, func(i, j int) bool { return violations[i].Always && !violations[j].Always })
	violations = tools.RedactDockerComposeMasterKey(violations, policy.MasterKey)
	if len(violations) > 0 {
		return dockerComposeViolationOutputFor(violations, narrowed)
	}
	if envFilesUnknown {
		return dockerComposeDenied("docker_compose_host_access_denied",
			"Docker Compose stack rejected before anything ran: this Docker Compose version cannot list env_file paths, so AuraGo cannot check that they stay inside the agent workspace (that needs Docker Compose v2.35 or newer). Update the Docker Compose plugin, move the variables into the Compose file, or enable Config → Danger Zone → \"Docker host access for agent Compose stacks\" (docker.allow_host_access). Do not retry unchanged.", nil)
	}
	return ""
}

// dockerComposeRawStateReference is a coarse check of the lower-cased main
// Compose file for commands whose profile services cannot be resolved: it
// reports an absolute AuraGo state path the text names (data directory,
// /etc/aurago, config, the .env next to it, vault, SQLite files, the
// master-key secret, an aurago_master.key file) or the master key value.
// Relative paths and included files are not seen.
func dockerComposeRawStateReference(lowerRaw string, cfg *config.Config) (string, bool) {
	if tools.DockerComposeLowerTextCarriesMasterKey(lowerRaw, dockerComposeMasterKey(cfg)) {
		return "", true
	}
	text := dockerComposeSlashPath(lowerRaw)
	roots, files := tools.DockerComposeProtectedPaths(cfg)
	for _, path := range append(append([]string(nil), roots...), files...) {
		needle := strings.TrimRight(dockerComposeSlashPath(strings.ToLower(strings.TrimSpace(path))), "/")
		absolute := strings.HasPrefix(needle, "/") || (len(needle) > 2 && needle[1] == ':' && needle[2] == '/')
		if absolute && dockerComposeTextNamesPath(text, needle) {
			return path, true
		}
	}
	for _, name := range []string{"aurago_master.key", "aurago_master_key"} {
		if strings.Contains(text, name) {
			return name, true
		}
	}
	return "", false
}

// dockerComposeSlashPath turns backslashes into slashes and collapses runs of
// slashes, so Windows paths written either way compare equal.
func dockerComposeSlashPath(text string) string {
	text = strings.ReplaceAll(text, `\`, "/")
	for strings.Contains(text, "//") {
		text = strings.ReplaceAll(text, "//", "/")
	}
	return text
}

// dockerComposeTextNamesPath reports needle in text as a whole path (or a
// parent of the path written there), not as part of a longer name.
func dockerComposeTextNamesPath(text, needle string) bool {
	nameByte := func(b byte) bool {
		return b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '.' || b == '_' || b == '-'
	}
	for offset := 0; ; {
		index := strings.Index(text[offset:], needle)
		if index < 0 {
			return false
		}
		start, end := offset+index, offset+index+len(needle)
		if (start == 0 || !nameByte(text[start-1])) && (end == len(text) || !nameByte(text[end])) {
			return true
		}
		offset = start + 1
	}
}

// dockerComposeMayUseEnvFiles reports a lower-cased main Compose file that
// names env_file or pulls in other files (include, extends) that may.
func dockerComposeMayUseEnvFiles(lowerRaw string) bool {
	return strings.Contains(lowerRaw, "env_file") || strings.Contains(lowerRaw, "include:") || strings.Contains(lowerRaw, "extends:")
}

// dockerComposeCommandViolations checks the build flags that reach the host
// directly: `--ssh` forwards the host's SSH agent or keys (host-access tier)
// and `--build-arg NAME` without a value copies NAME from the environment, so
// it must never name the master key (always tier).
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

// dockerComposeShownViolations bounds the violations a denial lists.
const dockerComposeShownViolations = 6

const (
	dockerComposeHostAccessHint = "Without Config → Danger Zone → \"Docker host access for agent Compose stacks\" (docker.allow_host_access) agent Compose stacks cannot use host paths outside the agent workspace, devices, privileged mode, host namespaces or other host access; every service of the file's default profiles is checked, not only the ones named in the command. Enable that setting or keep the stack inside the workspace. Do not retry unchanged."
	dockerComposeStateHint      = "AuraGo's own data directory, configuration, .env and master key can never be used by agent Compose stacks, even with docker.allow_host_access. Do not retry unchanged."
	// dockerComposeNarrowedHostAccessHint replaces dockerComposeHostAccessHint
	// when the check was limited to the named services (namedServiceClosure).
	dockerComposeNarrowedHostAccessHint = "Without Config → Danger Zone → \"Docker host access for agent Compose stacks\" (docker.allow_host_access) agent Compose stacks cannot use host paths outside the agent workspace, devices, privileged mode, host namespaces or other host access; the named services and the services they need were checked. Enable that setting or keep the stack inside the workspace. Do not retry unchanged."
)

func dockerComposeViolationOutput(violations []tools.DockerComposeViolation) string {
	return dockerComposeViolationOutputFor(violations, false)
}

// dockerComposeViolationOutputFor is dockerComposeViolationOutput; narrowed
// selects the hint for a check limited to the named services.
func dockerComposeViolationOutputFor(violations []tools.DockerComposeViolation, narrowed bool) string {
	hostHint := dockerComposeHostAccessHint
	if narrowed {
		hostHint = dockerComposeNarrowedHostAccessHint
	}
	always, host := false, false
	for _, violation := range violations {
		always = always || violation.Always
		host = host || !violation.Always
	}
	code := "docker_compose_host_access_denied"
	hint := hostHint
	if always {
		code = "docker_compose_protected_path_denied"
		hint = dockerComposeStateHint
		if host {
			hint = dockerComposeStateHint + " " + hostHint
		}
	}
	shown := violations
	if len(shown) > dockerComposeShownViolations {
		shown = shown[:dockerComposeShownViolations]
	}
	parts := make([]string, 0, len(shown))
	for _, violation := range shown {
		parts = append(parts, violation.String())
	}
	message := "Docker Compose stack rejected before anything ran: " + strings.Join(parts, "; ")
	if len(violations) > len(shown) {
		message += fmt.Sprintf("; and %d more", len(violations)-len(shown))
	}
	return dockerComposeDenied(code, message+". "+hint, shown)
}
