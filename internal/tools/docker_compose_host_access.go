package tools

import (
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
)

// DockerComposeMasterKeyVariable is the process environment variable that
// holds AuraGo's master key. Compose inherits AuraGo's environment, so the
// variable reaches a stack through interpolation, bare environment entries,
// secrets with an environment source and `build --build-arg NAME`.
const DockerComposeMasterKeyVariable = "AURAGO_MASTER_KEY"

// DockerComposeHostPolicy is the agent Compose policy for the commands that
// create containers, build images or render the model. AllowHostAccess is
// docker.allow_host_access intersected with the run's runtime permissions.
// WorkspaceDir is the jail root the preflight confined the compose file to
// (the agent workspace, or the process working directory without one); it
// must not be empty, because an empty root disables the bind jail.
type DockerComposeHostPolicy struct {
	WorkspaceDir    string
	AllowHostAccess bool
	ProtectedRoots  []string // AuraGo state directories: equal or inside is rejected
	ProtectedFiles  []string // AuraGo state files: equal is rejected
	MasterKey       string
}

// DockerComposeHostAccessScope selects what EvaluateDockerComposeHostAccess
// checks for one Compose subcommand.
type DockerComposeHostAccessScope int

const (
	// DockerComposeScopeRun is `up` and `create`: everything a container or
	// an image build gets from the host.
	DockerComposeScopeRun DockerComposeHostAccessScope = iota + 1
	// DockerComposeScopeBuild is `build`: the build sections only.
	DockerComposeScopeBuild
	// DockerComposeScopeRender is `config` and `convert`: only the AuraGo state
	// the rendered model would print (env files, which `config` inlines into
	// environment, secret and config files, and the master key value). The
	// host-access tier never applies.
	DockerComposeScopeRender
)

// DockerComposeViolation names one rejected attribute of a resolved model.
// Always is true for the tier that docker.allow_host_access never lifts.
type DockerComposeViolation struct {
	Service string `json:"service,omitempty"`
	Field   string `json:"field"`
	Value   string `json:"value,omitempty"`
	Reason  string `json:"reason"`
	Always  bool   `json:"always"`
}

// String renders the violation for the agent's tool result.
func (v DockerComposeViolation) String() string {
	subject := v.Field
	if v.Service != "" {
		subject = "service " + v.Service + " " + v.Field
	}
	if v.Value != "" {
		subject += " " + v.Value
	}
	return subject + ": " + v.Reason
}

// DockerComposeEnvFile is one service env_file reference.
type DockerComposeEnvFile struct {
	Service string
	Path    string
}

// DockerComposeServiceEnvFiles lists the env_file paths of model, sorted by
// service, with relative paths joined to projectDir. Only a model resolved
// with `--no-env-resolution` (the all-profiles model) lists env files; the
// default model inlines them into environment and drops the paths.
func DockerComposeServiceEnvFiles(model DockerComposeModel, projectDir string) []DockerComposeEnvFile {
	var out []DockerComposeEnvFile
	for _, name := range sortedDockerComposeKeys(model.Services) {
		for _, ref := range model.Services[name].EnvFile {
			path := strings.TrimSpace(ref.Path)
			if path == "" {
				continue
			}
			if !filepath.IsAbs(path) && !isWindowsAbsolutePath(path) && !strings.HasPrefix(path, "/") {
				path = filepath.Join(projectDir, path)
			}
			out = append(out, DockerComposeEnvFile{Service: name, Path: path})
		}
	}
	return out
}

// DockerComposeProtectedPaths lists AuraGo's own state for the always-enforced
// Compose tier: the data directory, /etc/aurago (systemd master key), the
// active config file, the vault, configured SQLite files, the .env next to the
// config and the Docker master-key secret. Parents of these paths are not
// protected.
func DockerComposeProtectedPaths(cfg *config.Config) (roots, files []string) {
	if cfg == nil {
		return nil, nil
	}
	if dataDir := strings.TrimSpace(cfg.Directories.DataDir); dataDir != "" {
		roots = append(roots, dataDir)
	}
	roots = append(roots, "/etc/aurago")
	files = append(files, protectedSystemFilesFromConfig(cfg)...)
	if configPath := strings.TrimSpace(cfg.ConfigPath); configPath != "" {
		files = append(files, filepath.Join(filepath.Dir(configPath), ".env"))
	}
	files = append(files, "/run/secrets/aurago_master_key")
	return roots, files
}

// DockerComposeSubcommand returns the validated Compose subcommand of cmd, or
// "" when cmd is not an allowed agent Compose command.
func DockerComposeSubcommand(cmd string) string {
	parts, err := dockerComposeParts(cmd)
	if err != nil {
		return ""
	}
	return parts[0]
}

const dockerComposeStateReason = "AuraGo's own data, configuration or master key cannot be used by agent Compose stacks"
const dockerComposeMasterKeyReason = "AuraGo's master key cannot be passed to agent Compose stacks"

// EvaluateDockerComposeHostAccess applies the agent Compose policy for scope
// to a resolved model; envFiles are the model's env_file paths when they are
// known. Violations with Always=true are rejected even when
// policy.AllowHostAccess is set: paths equal to or inside AuraGo state and the
// master key value. Parents of protected paths (for example /) are not
// violations of that tier. The host-access tier (Always=false) applies to the
// run and build scopes only while policy.AllowHostAccess is off. The result
// lists the always tier first.
func EvaluateDockerComposeHostAccess(model DockerComposeModel, envFiles []DockerComposeEnvFile, scope DockerComposeHostAccessScope, policy DockerComposeHostPolicy) []DockerComposeViolation {
	var violations []DockerComposeViolation
	hostTier := !policy.AllowHostAccess && (scope == DockerComposeScopeRun || scope == DockerComposeScopeBuild)
	add := func(service, field, value, reason string, always bool) {
		violations = append(violations, DockerComposeViolation{Service: service, Field: field, Value: value, Reason: reason, Always: always})
	}
	host := func(service, field, value, reason string) {
		if hostTier {
			add(service, field, value, reason, false)
		}
	}
	checkPath := func(service, field, path string) {
		path = strings.TrimSpace(path)
		if path == "" || !dockerBindHostLooksLikePath(path) {
			return
		}
		if dockerComposeProtectedPath(path, policy) {
			add(service, field, path, dockerComposeStateReason, true)
			return
		}
		if !hostTier {
			return
		}
		if err := validateDockerBindMount(DockerConfig{WorkspaceDir: policy.WorkspaceDir}, path+":/aurago-compose-policy"); err != nil {
			add(service, field, path, err.Error(), false)
		}
	}
	masterKey := func(service, field string, value *string) {
		if dockerComposeValueCarriesMasterKey(value, policy.MasterKey) {
			add(service, field, "", dockerComposeMasterKeyReason, true)
		}
	}

	for _, name := range sortedDockerComposeKeys(model.Services) {
		service := model.Services[name]
		if build := service.Build; build != nil {
			for _, key := range sortedDockerComposeKeys(build.Args) {
				masterKey(name, "build.args."+key, build.Args[key])
			}
			if scope != DockerComposeScopeRender {
				checkPath(name, "build.context", build.Context)
				if dockerfile := strings.TrimSpace(build.Dockerfile); filepath.IsAbs(dockerfile) || isWindowsAbsolutePath(dockerfile) || strings.HasPrefix(dockerfile, "/") {
					checkPath(name, "build.dockerfile", dockerfile)
				}
				for _, key := range sortedDockerComposeKeys(build.AdditionalContexts) {
					checkPath(name, "build.additional_contexts."+key, build.AdditionalContexts[key])
				}
				if len(build.SSH) > 0 {
					host(name, "build.ssh", strings.Join(build.SSH, ","), "the build gets the host's SSH agent or SSH keys")
				}
				if build.Privileged {
					host(name, "build.privileged", "true", "privileged builds run with full access to the host")
				}
				if len(build.Entitlements) > 0 {
					host(name, "build.entitlements", strings.Join(build.Entitlements, ","), "build entitlements lift the build sandbox")
				}
				if scope == DockerComposeScopeBuild {
					// up/create check every top-level secret below.
					for _, ref := range build.Secrets {
						checkPath(name, "build.secrets."+ref, model.Secrets[ref].File)
					}
				}
			}
		}
		if scope == DockerComposeScopeBuild {
			continue
		}
		for _, key := range sortedDockerComposeKeys(service.Environment) {
			masterKey(name, "environment."+key, service.Environment[key])
		}
		for _, key := range sortedDockerComposeKeys(service.Labels) {
			value := service.Labels[key]
			masterKey(name, "labels."+key, &value)
		}
		if scope == DockerComposeScopeRender {
			continue
		}
		for _, mount := range service.Volumes {
			switch strings.ToLower(strings.TrimSpace(mount.Type)) {
			case "bind":
				checkPath(name, "volumes", mount.Source)
			case "npipe":
				host(name, "volumes", mount.Source, "named pipe mounts reach host services such as the Docker engine")
			}
		}
		if service.Develop != nil {
			for _, watch := range service.Develop.Watch {
				checkPath(name, "develop.watch", watch.Path)
			}
		}
		if service.Provider != nil {
			host(name, "provider", strings.TrimSpace(service.Provider.Type), "Compose runs the provider as a program on the host")
		}
		if service.Privileged {
			host(name, "privileged", "true", "privileged containers get full access to host devices and the kernel")
		}
		for _, device := range service.Devices {
			host(name, "devices", device.Source, "host devices are passed into the container")
		}
		for _, namespace := range []struct{ field, value string }{
			{"network_mode", service.NetworkMode}, {"pid", service.Pid}, {"ipc", service.Ipc}, {"userns_mode", service.UsernsMode},
			{"uts", service.Uts}, {"cgroup", service.Cgroup},
		} {
			if strings.EqualFold(strings.TrimSpace(namespace.value), "host") {
				host(name, namespace.field, "host", "the container shares the host namespace")
			}
		}
		for _, capability := range service.CapAdd {
			host(name, "cap_add", capability, "added kernel capabilities widen host access")
		}
		for _, option := range service.SecurityOpt {
			lower := strings.ToLower(strings.TrimSpace(option))
			if strings.Contains(lower, "unconfined") || lower == "label=disable" || lower == "label:disable" {
				host(name, "security_opt", option, "the container runs without its security profile")
			}
		}
	}
	if scope == DockerComposeScopeBuild {
		return sortDockerComposeViolations(violations)
	}
	for _, ref := range envFiles {
		checkPath(ref.Service, "env_file", ref.Path)
	}
	for _, kind := range []struct {
		name      string
		resources map[string]DockerComposeFileResource
	}{{"secrets", model.Secrets}, {"configs", model.Configs}} {
		for _, key := range sortedDockerComposeKeys(kind.resources) {
			resource := kind.resources[key]
			prefix := kind.name + "." + key + "."
			checkPath("", prefix+"file", resource.File)
			if strings.EqualFold(strings.TrimSpace(resource.Environment), DockerComposeMasterKeyVariable) {
				add("", prefix+"environment", resource.Environment, dockerComposeMasterKeyReason, true)
			}
			content := resource.Content
			masterKey("", prefix+"content", &content)
		}
	}
	if scope == DockerComposeScopeRun {
		for _, key := range sortedDockerComposeKeys(model.Volumes) {
			if volume := model.Volumes[key]; dockerComposeVolumeIsLocalBind(volume) {
				checkPath("", "volumes."+key+".driver_opts.device", volume.DriverOpts["device"])
			}
		}
	}
	return sortDockerComposeViolations(violations)
}

func sortDockerComposeViolations(violations []DockerComposeViolation) []DockerComposeViolation {
	sort.SliceStable(violations, func(i, j int) bool { return violations[i].Always && !violations[j].Always })
	return violations
}

// DockerComposeTextCarriesMasterKey reports whether text (for example a
// resolved model's JSON) contains the master key value, ignoring case.
func DockerComposeTextCarriesMasterKey(text, masterKey string) bool {
	masterKey = strings.TrimSpace(masterKey)
	return masterKey != "" && strings.Contains(strings.ToLower(text), strings.ToLower(masterKey))
}

func dockerComposeValueCarriesMasterKey(value *string, masterKey string) bool {
	return value != nil && DockerComposeTextCarriesMasterKey(*value, masterKey)
}

// dockerComposeVolumeIsLocalBind reports a local-driver volume that binds a
// host directory (driver_opts o=bind or type=none with a device).
func dockerComposeVolumeIsLocalBind(volume DockerComposeNamedVolume) bool {
	driver := strings.ToLower(strings.TrimSpace(volume.Driver))
	if driver != "" && driver != "local" {
		return false
	}
	if strings.TrimSpace(volume.DriverOpts["device"]) == "" {
		return false
	}
	for _, option := range strings.Split(volume.DriverOpts["o"], ",") {
		if strings.EqualFold(strings.TrimSpace(option), "bind") {
			return true
		}
	}
	return strings.EqualFold(strings.TrimSpace(volume.DriverOpts["type"]), "none")
}

// dockerComposeProtectedPath reports a path equal to or inside AuraGo state,
// comparing cleaned, absolute and symlink-resolved spellings.
func dockerComposeProtectedPath(path string, policy DockerComposeHostPolicy) bool {
	protected := append(append([]string(nil), policy.ProtectedRoots...), policy.ProtectedFiles...)
	for _, candidate := range dockerComposePathVariants(path) {
		base := strings.ToLower(pathpkg.Base(candidate))
		if base == "aurago_master.key" || base == "aurago_master_key" {
			return true
		}
		for _, entry := range protected {
			for _, root := range dockerComposePathVariants(entry) {
				if dockerPathEqualOrWithin(candidate, root) {
					return true
				}
			}
		}
	}
	return false
}

// dockerComposePathVariants returns the cleaned, absolute and symlink-resolved
// spellings of path in Docker bind notation.
func dockerComposePathVariants(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	variants := []string{cleanDockerHostPath(path)}
	native := filepath.FromSlash(dockerutil.NormalizeHostPathForBind(path))
	if abs, err := filepath.Abs(native); err == nil {
		variants = append(variants, cleanDockerHostPath(abs))
		if resolved, err := secureResolveFinalPath(filepath.Clean(abs)); err == nil {
			variants = append(variants, cleanDockerHostPath(resolved))
		}
	}
	return variants
}
