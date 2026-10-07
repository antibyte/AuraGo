package tools

import (
	pathpkg "path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
)

// DockerComposeMasterKeyVariable is the process environment variable that
// holds AuraGo's master key. Docker CLI children never get it
// (dockerCLICommand), but a stack can still name it, for example in a secret
// with an environment source or `build --build-arg NAME`.
const DockerComposeMasterKeyVariable = "AURAGO_MASTER_KEY"

// DockerComposeHostPolicy is the agent Compose policy for the commands that
// create, build, pull, render or start and stop a stack. AllowHostAccess is
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
	// DockerComposeScopeBuild is `build`: the build sections and image names.
	DockerComposeScopeBuild
	// DockerComposeScopeRender is `config` and `convert`: only the AuraGo state
	// the rendered model would print (env files, which `config` inlines into
	// environment, secret and config files, and the master key value). The
	// host-access tier never applies.
	DockerComposeScopeRender
	// DockerComposeScopePull is `pull`: the master key in image names and,
	// without host access, programs Compose runs on the host (providers,
	// privileged hooks).
	DockerComposeScopePull
	// DockerComposeScopeLifecycle is `down`, `start`, `stop`, `restart` and
	// `rm`: without host access only the programs Compose runs on the host
	// (providers, privileged hooks).
	DockerComposeScopeLifecycle
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
// default model inlines them into environment and drops the paths. known is
// false when an entry had a shape the parser could not read.
func DockerComposeServiceEnvFiles(model DockerComposeModel, projectDir string) (files []DockerComposeEnvFile, known bool) {
	known = true
	for _, name := range sortedDockerComposeKeys(model.Services) {
		for _, ref := range model.Services[name].EnvFile {
			if ref.Unknown {
				known = false
				continue
			}
			path := strings.TrimSpace(ref.Path)
			if path == "" {
				continue
			}
			if !filepath.IsAbs(path) && !isWindowsAbsolutePath(path) && !strings.HasPrefix(path, "/") {
				path = filepath.Join(projectDir, path)
			}
			files = append(files, DockerComposeEnvFile{Service: name, Path: path})
		}
	}
	return files, known
}

// DockerComposeProtectedPaths lists AuraGo's own state for the always-enforced
// Compose tier: the data directory, /etc/aurago (systemd master key), the
// active config file, the vault, configured SQLite files, the .env next to the
// config and the Docker master-key secret. Parents of these paths are not
// protected by that tier.
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

// DockerBindTargetsAuraGoState reports a create/run volume string whose host
// path is equal to or inside AuraGo state (roots and files as
// DockerComposeProtectedPaths lists them, plus any aurago_master.key), with
// the Compose always tier's comparison: cleaned, absolute and resolved
// spellings, Docker Desktop aliases and macOS case folding. Parents of state
// and named volumes are not reported. It returns the host path it refused.
func DockerBindTargetsAuraGoState(bind string, roots, files []string) (string, bool) {
	spec, ok := parseDockerBindMount(bind)
	if !ok || !spec.isHostPath {
		return "", false
	}
	e := newDockerComposeEvaluation(DockerComposeHostPolicy{AllowHostAccess: true, ProtectedRoots: roots, ProtectedFiles: files}, DockerComposeScopeRun)
	if e.isProtected(dockerComposePathVariants(spec.hostPath)) {
		return spec.hostPath, true
	}
	return "", false
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

// DockerComposeTextCarriesMasterKey reports whether text (for example a
// resolved model's JSON) contains the master key value, ignoring case.
func DockerComposeTextCarriesMasterKey(text, masterKey string) bool {
	return DockerComposeLowerTextCarriesMasterKey(strings.ToLower(text), masterKey)
}

// DockerComposeLowerTextCarriesMasterKey is DockerComposeTextCarriesMasterKey
// for text that is already lower-cased.
func DockerComposeLowerTextCarriesMasterKey(lowerText, masterKey string) bool {
	masterKey = strings.ToLower(strings.TrimSpace(masterKey))
	return masterKey != "" && strings.Contains(lowerText, masterKey)
}

const (
	dockerComposeStateReason     = "AuraGo's own data, configuration or master key cannot be used by agent Compose stacks"
	dockerComposeMasterKeyReason = "AuraGo's master key cannot be passed to agent Compose stacks"
	dockerComposeParentReason    = "the path contains AuraGo's own data, configuration or master key"
)

// dockerComposeNetworkFilesystems are local-driver volume types that mount a
// network share or memory rather than a host disk.
var dockerComposeNetworkFilesystems = map[string]bool{
	"nfs": true, "nfs4": true, "cifs": true, "smb": true, "smb2": true, "smb3": true,
	"smbfs": true, "ceph": true, "glusterfs": true, "tmpfs": true,
}

// dockerComposeEvaluation holds one evaluation's precomputed paths.
type dockerComposeEvaluation struct {
	policy     DockerComposeHostPolicy
	hostTier   bool
	masterKey  string   // lower-cased
	protected  []string // cleaned, absolute and resolved spellings of AuraGo state
	root       string   // resolved jail root in bind notation, "" when unknown
	rootClean  string   // the jail root as configured, in bind notation
	violations []DockerComposeViolation
}

func newDockerComposeEvaluation(policy DockerComposeHostPolicy, scope DockerComposeHostAccessScope) *dockerComposeEvaluation {
	e := &dockerComposeEvaluation{
		policy:    policy,
		hostTier:  !policy.AllowHostAccess && scope != DockerComposeScopeRender,
		masterKey: strings.ToLower(strings.TrimSpace(policy.MasterKey)),
	}
	seen := map[string]bool{}
	for _, entry := range append(append([]string(nil), policy.ProtectedRoots...), policy.ProtectedFiles...) {
		// Spellings only: de-aliasing AuraGo's own paths would protect other
		// directories on a native host, where /host_mnt/... is an ordinary path.
		for _, variant := range dockerComposePathSpellings(entry) {
			if !seen[variant] {
				seen[variant] = true
				e.protected = append(e.protected, variant)
			}
		}
	}
	if root := strings.TrimSpace(policy.WorkspaceDir); root != "" {
		if resolved, err := secureResolveFinalPath(filepath.Clean(root)); err == nil {
			e.root = cleanDockerHostPath(resolved)
			e.rootClean = cleanDockerHostPath(root)
		}
	}
	return e
}

func (e *dockerComposeEvaluation) add(service, field, value, reason string, always bool) {
	e.violations = append(e.violations, DockerComposeViolation{Service: service, Field: field, Value: value, Reason: reason, Always: always})
}

// host records a host-access violation while the host-access tier applies.
func (e *dockerComposeEvaluation) host(service, field, value, reason string) {
	if e.hostTier {
		e.add(service, field, value, reason, false)
	}
}

func (e *dockerComposeEvaluation) checkMasterKey(service, field, value string) {
	if e.masterKey != "" && strings.Contains(strings.ToLower(value), e.masterKey) {
		e.add(service, field, "", dockerComposeMasterKeyReason, true)
	}
}

// checkPath rejects a host path equal to or inside AuraGo state (always) and,
// while the host-access tier applies, a path outside the jail root, a
// sensitive location below the root, or a path that contains AuraGo state.
// Being inside the root is tested before the sensitive list, so a workspace
// under /mnt, /root or C:\ProgramData keeps working.
func (e *dockerComposeEvaluation) checkPath(service, field, path string) {
	path = strings.TrimSpace(path)
	if path == "" || !dockerBindHostLooksLikePath(path) {
		return
	}
	variants := dockerComposePathVariants(path)
	if e.isProtected(variants) {
		e.add(service, field, path, dockerComposeStateReason, true)
		return
	}
	if !e.hostTier {
		return
	}
	if !e.withinRoot(path) {
		if err := validateDockerBindMount(DockerConfig{WorkspaceDir: e.policy.WorkspaceDir}, path+":/aurago-compose-policy"); err != nil {
			e.add(service, field, path, err.Error(), false)
			return
		}
	}
	if e.containsProtected(variants) {
		e.add(service, field, path, dockerComposeParentReason, false)
	}
}

func (e *dockerComposeEvaluation) isProtected(variants []string) bool {
	for _, candidate := range variants {
		base := strings.ToLower(pathpkg.Base(candidate))
		if base == "aurago_master.key" || base == "aurago_master_key" {
			return true
		}
		for _, protected := range e.protected {
			if dockerPathEqualOrWithin(dockerComposeFoldCase(candidate), dockerComposeFoldCase(protected)) {
				return true
			}
		}
	}
	return false
}

// containsProtected reports a path that is a parent of AuraGo state.
func (e *dockerComposeEvaluation) containsProtected(variants []string) bool {
	for _, candidate := range variants {
		for _, protected := range e.protected {
			if dockerComposePathContains(dockerComposeFoldCase(candidate), dockerComposeFoldCase(protected)) {
				return true
			}
		}
	}
	return false
}

// withinRoot reports a path whose resolved location is inside the jail root
// and that is not inside a sensitive location lying below the root (such as
// /etc when the root is /), comparing the path as written with the root as
// configured and the resolved path with the resolved root.
func (e *dockerComposeEvaluation) withinRoot(path string) bool {
	if e.root == "" {
		return false
	}
	cleaned := cleanDockerHostPath(path)
	resolved, err := secureResolveFinalPath(filepath.FromSlash(cleaned))
	if err != nil {
		return false
	}
	candidate := cleanDockerHostPath(resolved)
	if !dockerPathEqualOrWithin(candidate, e.root) {
		return false
	}
	return !dockerSensitiveLocationBelow(cleaned, e.rootClean) && !dockerSensitiveLocationBelow(candidate, e.root)
}

// dockerSensitiveLocationBelow reports a path inside a sensitive host location
// (isSensitiveDockerHostPath) that does not also contain root.
func dockerSensitiveLocationBelow(path, root string) bool {
	for _, sensitive := range sensitiveDockerHostPaths {
		if dockerPathEqualOrWithin(path, sensitive) && !dockerPathEqualOrWithin(root, sensitive) {
			return true
		}
	}
	location := sensitiveWindowsHostLocation(path)
	return location != "" && location != sensitiveWindowsHostLocation(root)
}

// dockerComposePathContains reports child equal to or below parent; unlike
// dockerPathEqualOrWithin, "/" contains every absolute path.
func dockerComposePathContains(parent, child string) bool {
	parent = strings.TrimRight(cleanDockerHostPath(parent), "/")
	child = cleanDockerHostPath(child)
	if isWindowsAbsolutePath(child) || isWindowsAbsolutePath(parent+"/") {
		parent = strings.ToLower(parent)
		child = strings.ToLower(child)
	}
	if parent == "" {
		return strings.HasPrefix(child, "/")
	}
	return child == parent || strings.HasPrefix(child, parent+"/")
}

// hostExecution checks what Compose runs on the host or with host privileges
// whenever it starts or stops services: provider programs and privileged
// post_start/pre_stop hooks.
func (e *dockerComposeEvaluation) hostExecution(name string, service DockerComposeService) {
	if service.Provider != nil {
		e.host(name, "provider", strings.TrimSpace(service.Provider.Type), "Compose runs the provider as a program on the host")
	}
	for _, hooks := range []struct {
		field string
		hooks DockerComposeHooks
	}{{"post_start", service.PostStart}, {"pre_stop", service.PreStop}} {
		for _, hook := range hooks.hooks {
			if hook.Privileged {
				e.host(name, hooks.field, "privileged", "privileged hooks run with full access to the host")
				break
			}
		}
	}
}

// EvaluateDockerComposeHostAccess applies the agent Compose policy for scope
// to a resolved model; envFiles are the model's env_file paths when they are
// known. Violations with Always=true are rejected even when
// policy.AllowHostAccess is set: paths equal to or inside AuraGo state and the
// master key value. Parents of protected paths (for example /) are not
// violations of that tier. The host-access tier (Always=false) applies to
// every scope but render while policy.AllowHostAccess is off. The result lists
// the always tier first.
func EvaluateDockerComposeHostAccess(model DockerComposeModel, envFiles []DockerComposeEnvFile, scope DockerComposeHostAccessScope, policy DockerComposeHostPolicy) []DockerComposeViolation {
	e := newDockerComposeEvaluation(policy, scope)
	buildsImages := scope == DockerComposeScopeRun || scope == DockerComposeScopeBuild
	for _, name := range sortedDockerComposeKeys(model.Services) {
		service := model.Services[name]
		if scope != DockerComposeScopeLifecycle {
			e.checkMasterKey(name, "image", service.Image)
		}
		if scope == DockerComposeScopeRun || scope == DockerComposeScopePull || scope == DockerComposeScopeLifecycle {
			e.hostExecution(name, service)
		}
		if scope == DockerComposeScopePull || scope == DockerComposeScopeLifecycle {
			continue
		}
		if build := service.Build; build != nil {
			for _, key := range sortedDockerComposeKeys(build.Args) {
				e.checkMasterKey(name, "build.args", key)
				if value := build.Args[key]; value != nil {
					e.checkMasterKey(name, "build.args."+key, *value)
				}
			}
			for _, label := range build.Labels {
				e.checkMasterKey(name, "build.labels", label)
			}
			if buildsImages {
				e.checkBuild(model, name, build, scope)
			}
		}
		if scope == DockerComposeScopeBuild {
			continue
		}
		for _, key := range sortedDockerComposeKeys(service.Environment) {
			if value := service.Environment[key]; value != nil {
				e.checkMasterKey(name, "environment."+key, *value)
			}
		}
		for _, key := range sortedDockerComposeKeys(service.Labels) {
			e.checkMasterKey(name, "labels."+key, service.Labels[key])
		}
		if scope == DockerComposeScopeRun {
			e.checkRunService(name, service)
		}
	}
	if scope == DockerComposeScopeRun || scope == DockerComposeScopeRender {
		for _, ref := range envFiles {
			e.checkPath(ref.Service, "env_file", ref.Path)
		}
		for _, kind := range []struct {
			name      string
			resources map[string]DockerComposeFileResource
		}{{"secrets", model.Secrets}, {"configs", model.Configs}} {
			for _, key := range sortedDockerComposeKeys(kind.resources) {
				e.checkFileResource("", kind.name+"."+key+".", kind.resources[key])
			}
		}
	}
	if scope == DockerComposeScopeRun {
		for _, key := range sortedDockerComposeKeys(model.Volumes) {
			e.checkLocalVolume(key, model.Volumes[key])
		}
	}
	sort.SliceStable(e.violations, func(i, j int) bool { return e.violations[i].Always && !e.violations[j].Always })
	return RedactDockerComposeMasterKey(e.violations, policy.MasterKey)
}

// RedactDockerComposeMasterKey replaces the master key value in the fields
// and values of violations, so a denial never echoes it.
func RedactDockerComposeMasterKey(violations []DockerComposeViolation, masterKey string) []DockerComposeViolation {
	key := strings.ToLower(strings.TrimSpace(masterKey))
	if key == "" {
		return violations
	}
	redact := func(text string) string {
		var out strings.Builder
		for i := 0; i < len(text); {
			if i+len(key) <= len(text) && strings.EqualFold(text[i:i+len(key)], key) {
				out.WriteString("[master key]")
				i += len(key)
				continue
			}
			out.WriteByte(text[i])
			i++
		}
		return out.String()
	}
	for i := range violations {
		violations[i].Field = redact(violations[i].Field)
		violations[i].Value = redact(violations[i].Value)
	}
	return violations
}

// checkBuild checks one build section for up/create/build.
func (e *dockerComposeEvaluation) checkBuild(model DockerComposeModel, name string, build *DockerComposeBuild, scope DockerComposeHostAccessScope) {
	contextDir := strings.TrimSpace(build.Context)
	e.checkPath(name, "build.context", contextDir)
	if dockerfile := strings.TrimSpace(build.Dockerfile); dockerfile != "" {
		switch {
		case filepath.IsAbs(dockerfile) || isWindowsAbsolutePath(dockerfile) || strings.HasPrefix(dockerfile, "/"):
			e.checkPath(name, "build.dockerfile", dockerfile)
		case contextDir != "" && dockerBindHostLooksLikePath(contextDir):
			// A relative Dockerfile resolves against the context and may leave
			// it; inside the context it adds no host access, only AuraGo state.
			joined := filepath.Join(filepath.FromSlash(contextDir), filepath.FromSlash(dockerfile))
			if dockerPathEqualOrWithin(joined, contextDir) {
				if e.isProtected(dockerComposePathVariants(joined)) {
					e.add(name, "build.dockerfile", joined, dockerComposeStateReason, true)
				}
			} else {
				e.checkPath(name, "build.dockerfile", joined)
			}
		}
	}
	for _, key := range sortedDockerComposeKeys(build.AdditionalContexts) {
		e.checkPath(name, "build.additional_contexts."+key, build.AdditionalContexts[key])
	}
	if len(build.SSH) > 0 {
		e.host(name, "build.ssh", strings.Join(build.SSH, ","), "the build gets the host's SSH agent or SSH keys")
	}
	if build.Privileged {
		e.host(name, "build.privileged", "true", "privileged builds run with full access to the host")
	}
	if len(build.Entitlements) > 0 {
		e.host(name, "build.entitlements", strings.Join(build.Entitlements, ","), "build entitlements lift the build sandbox")
	}
	if strings.EqualFold(strings.TrimSpace(string(build.Network)), "host") {
		e.host(name, "build.network", "host", "the build shares the host network")
	}
	if scope == DockerComposeScopeBuild {
		// up/create check every top-level secret and config instead.
		for _, ref := range build.Secrets {
			e.checkFileResource(name, "build.secrets."+ref+".", model.Secrets[ref])
		}
	}
}

// checkFileResource checks a secret or config source: a host file, a
// variable of the Compose process environment or inline content.
func (e *dockerComposeEvaluation) checkFileResource(service, prefix string, resource DockerComposeFileResource) {
	e.checkPath(service, prefix+"file", resource.File)
	if strings.EqualFold(strings.TrimSpace(resource.Environment), DockerComposeMasterKeyVariable) {
		e.add(service, prefix+"environment", resource.Environment, dockerComposeMasterKeyReason, true)
	}
	e.checkMasterKey(service, prefix+"content", resource.Content)
}

// checkRunService checks what a container of service gets from the host.
func (e *dockerComposeEvaluation) checkRunService(name string, service DockerComposeService) {
	for _, mount := range service.Volumes {
		switch strings.ToLower(strings.TrimSpace(mount.Type)) {
		case "bind":
			e.checkPath(name, "volumes", mount.Source)
		case "npipe":
			e.host(name, "volumes", mount.Source, "named pipe mounts reach host services such as the Docker engine")
		}
	}
	if service.Develop != nil {
		for _, watch := range service.Develop.Watch {
			e.checkPath(name, "develop.watch", watch.Path)
		}
	}
	if service.UseAPISocket {
		e.host(name, "use_api_socket", "true", "the container gets the Docker API socket and so control over the host")
	}
	if service.Privileged {
		e.host(name, "privileged", "true", "privileged containers get full access to host devices and the kernel")
	}
	for _, device := range service.Devices {
		e.host(name, "devices", device.Source, "host devices are passed into the container")
	}
	if len(service.DeviceCgroupRules) > 0 {
		e.host(name, "device_cgroup_rules", strings.Join(service.DeviceCgroupRules, ","), "device cgroup rules open host devices to the container")
	}
	if len(service.Gpus) > 0 {
		e.host(name, "gpus", strings.Join(service.Gpus, ","), "host GPUs are passed into the container")
	}
	if len(service.Deploy.ReservedDevices) > 0 {
		e.host(name, "deploy.resources.reservations.devices", strings.Join(service.Deploy.ReservedDevices, ","), "host devices are passed into the container")
	}
	for _, namespace := range []struct{ field, value string }{
		{"network_mode", service.NetworkMode}, {"pid", service.Pid}, {"ipc", service.Ipc}, {"userns_mode", service.UsernsMode},
		{"uts", service.Uts}, {"cgroup", service.Cgroup},
	} {
		if strings.EqualFold(strings.TrimSpace(namespace.value), "host") {
			e.host(name, namespace.field, "host", "the container shares the host namespace")
		}
	}
	for _, capability := range service.CapAdd {
		e.host(name, "cap_add", capability, "added kernel capabilities widen host access")
	}
	for _, option := range service.SecurityOpt {
		lower := strings.ToLower(strings.TrimSpace(option))
		if strings.Contains(lower, "unconfined") || lower == "label=disable" || lower == "label:disable" {
			e.host(name, "security_opt", option, "the container runs without its security profile")
		}
	}
}

// checkLocalVolume checks a local-driver volume that mounts something from
// the host: a bind of a host directory, a block device under /dev, or a
// filesystem type other than a network share or tmpfs.
func (e *dockerComposeEvaluation) checkLocalVolume(key string, volume DockerComposeNamedVolume) {
	driver := strings.ToLower(strings.TrimSpace(volume.Driver))
	if driver != "" && driver != "local" {
		return
	}
	field := "volumes." + key + ".driver_opts"
	device := strings.TrimSpace(volume.DriverOpts["device"])
	fsType := strings.ToLower(strings.TrimSpace(volume.DriverOpts["type"]))
	if dockerComposeVolumeIsLocalBind(volume) {
		e.checkPath("", field+".device", device)
		return
	}
	// Whatever the mount type, a device path inside AuraGo state is rejected.
	if device != "" && dockerBindHostLooksLikePath(device) && e.isProtected(dockerComposePathVariants(device)) {
		e.add("", field+".device", device, dockerComposeStateReason, true)
		return
	}
	switch {
	case device != "" && dockerComposePathContains("/dev", device):
		e.host("", field+".device", device, "the volume mounts a host block device")
	case fsType != "" && fsType != "none" && !dockerComposeNetworkFilesystems[fsType]:
		e.host("", field+".type", fsType, "the volume mounts a host filesystem")
	}
}

// dockerComposeVolumeIsLocalBind reports a local-driver volume that binds a
// host directory (driver_opts o=bind or rbind, or type=none with a device).
func dockerComposeVolumeIsLocalBind(volume DockerComposeNamedVolume) bool {
	driver := strings.ToLower(strings.TrimSpace(volume.Driver))
	if driver != "" && driver != "local" {
		return false
	}
	if strings.TrimSpace(volume.DriverOpts["device"]) == "" {
		return false
	}
	for _, option := range strings.Split(volume.DriverOpts["o"], ",") {
		option = strings.TrimSpace(option)
		if strings.EqualFold(option, "bind") || strings.EqualFold(option, "rbind") {
			return true
		}
	}
	return strings.EqualFold(strings.TrimSpace(volume.DriverOpts["type"]), "none")
}

// dockerHostPathsFoldCase is true where host paths compare case-insensitively
// by default (macOS APFS/HFS+). Windows drive paths are folded by
// dockerPathEqualOrWithin already. Tests flip it.
var dockerHostPathsFoldCase = runtime.GOOS == "darwin"

func dockerComposeFoldCase(path string) string {
	if dockerHostPathsFoldCase {
		return strings.ToLower(path)
	}
	return path
}

// dockerDesktopHostAlias returns the host path a Docker Desktop VM spelling
// names: /host_mnt/<path> (macOS, Docker Desktop for Linux) and
// /run/desktop/mnt/host/<drive>/<path> or /mnt/host/<drive>/<path> (Windows).
// Callers pass a cleaned path (cleanDockerHostPath), so `..`, `.` and doubled
// slashes cannot hide the prefix; /host_mnt itself names the host root.
func dockerDesktopHostAlias(path string) (string, bool) {
	p := strings.ReplaceAll(strings.TrimSpace(path), `\`, "/")
	if p == "/host_mnt" {
		return "/", true
	}
	if rest, ok := strings.CutPrefix(p, "/host_mnt/"); ok {
		return "/" + rest, true
	}
	for _, prefix := range []string{"/run/desktop/mnt/host/", "/mnt/host/"} {
		rest, ok := strings.CutPrefix(p, prefix)
		if !ok {
			continue
		}
		drive, tail, _ := strings.Cut(rest, "/")
		if len(drive) == 1 && (drive[0] >= 'a' && drive[0] <= 'z' || drive[0] >= 'A' && drive[0] <= 'Z') {
			return drive + ":/" + tail, true
		}
	}
	return "", false
}

// dockerComposePathVariants returns the cleaned, absolute and symlink-resolved
// spellings of path in Docker bind notation and, for a Docker Desktop VM
// spelling (dockerDesktopHostAlias, read from the cleaned path), those of the
// host path it names. Only candidate paths get the alias; AuraGo's own
// protected paths use dockerComposePathSpellings.
func dockerComposePathVariants(path string) []string {
	variants := dockerComposePathSpellings(path)
	if strings.TrimSpace(path) == "" {
		return variants
	}
	if alias, ok := dockerDesktopHostAlias(cleanDockerHostPath(path)); ok {
		variants = append(variants, dockerComposePathSpellings(alias)...)
	}
	return variants
}

// dockerComposePathSpellings returns the cleaned, absolute and
// symlink-resolved spellings of path in Docker bind notation.
func dockerComposePathSpellings(path string) []string {
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
