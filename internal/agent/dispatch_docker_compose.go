package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"aurago/internal/acestep"
	"aurago/internal/config"
	"aurago/internal/dockerutil"
	"aurago/internal/tools"
)

// dockerComposePreflight is what the agent Compose policy needs for one call,
// loaded once: the jailed file, its raw text, Compose's default resolved model
// (stdout only; it is the validity gate) and the all-profiles model. Texts are
// lower-cased.
type dockerComposePreflight struct {
	file string
	// root is the absolute jail root the file was confined to: the agent
	// workspace, or the process working directory when none is configured.
	// Path checks of the host-access policy use the same root.
	root     string
	raw      string
	resolved string
	model    tools.DockerComposeModel
	// allProfilesModel is `--profile * config --no-env-resolution`: every
	// service, including inactive profiles that `up <service>` activates, with
	// env_file entries kept as paths; allProfilesRaw holds the same resolution
	// as raw JSON per service and resource. Policies never read them directly:
	// effectiveModel takes from them only what a command runs. Both are empty
	// when this Compose cannot produce the model (allProfilesErr).
	allProfilesModel *tools.DockerComposeModel
	allProfilesRaw   dockerComposeRawModel
	allProfilesErr   error
}

// dockerComposeRawModel is a resolved Compose model as raw JSON per entry.
type dockerComposeRawModel struct {
	Services map[string]json.RawMessage `json:"services"`
	Volumes  map[string]json.RawMessage `json:"volumes"`
	Networks map[string]json.RawMessage `json:"networks"`
	Secrets  map[string]json.RawMessage `json:"secrets"`
	Configs  map[string]json.RawMessage `json:"configs"`
}

// dockerComposeEffectiveModel is what one Compose command runs.
type dockerComposeEffectiveModel struct {
	// model holds every service of the default model plus each service the
	// command names as a positional argument of up/create/build/pull/config/
	// convert that only an inactive profile defines, with its depends_on and
	// `service:` build-context closure, and the top-level volumes, networks,
	// secrets (including build secrets) and configs of those services.
	// Definitions come from the all-profiles model when it exists (env_file
	// kept as paths), otherwise from the default model.
	model tools.DockerComposeModel
	// profileServices lists the services added from inactive profiles, sorted.
	profileServices []string
	// unverified lists named services outside the default model that could not
	// be checked because this Compose cannot produce the all-profiles model
	// (never for config/convert, which create nothing).
	unverified []string
	// profileText is the lower-cased raw JSON of the added services and the
	// resources they add, for the text token checks.
	profileText string
	// fromAllProfiles reports that the service definitions come from the
	// all-profiles model, so their env_file entries are the real paths. In the
	// default-model fallback env files are inlined and their paths unknown.
	fromAllProfiles bool
}

// loadDockerComposePreflight resolves the Compose file once per variant. Every
// failure of the default resolution is returned so the caller blocks the call,
// as before this preflight; a failed all-profiles resolution is recorded only.
// Without a configured workspace the process working directory is the jail,
// exactly as the Compose checks before this preflight confined the file.
func loadDockerComposePreflight(ctx context.Context, cfg tools.DockerConfig, file string) (*dockerComposePreflight, error) {
	workspaceConfigured := strings.TrimSpace(cfg.WorkspaceDir) != ""
	if !workspaceConfigured {
		workdir, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("determine working directory for the compose file: %w", err)
		}
		cfg.WorkspaceDir = workdir
	}
	root, err := filepath.Abs(cfg.WorkspaceDir)
	if err != nil {
		return nil, fmt.Errorf("resolve the compose jail root: %w", err)
	}
	composeFile, err := tools.ResolveDockerComposeFile(cfg, file)
	if err != nil {
		if errors.Is(err, tools.ErrDockerComposeFileOutsideWorkspace) {
			return nil, &dockerComposeOutsideJailError{file: file, root: cfg.WorkspaceDir, workspaceConfigured: workspaceConfigured, err: err}
		}
		return nil, err
	}
	raw, err := readDockerComposeFile(composeFile)
	if err != nil {
		return nil, err
	}
	resolved, err := resolveDockerComposeConfig(ctx, cfg, composeFile, tools.DockerComposeConfigOptions{})
	if err != nil {
		return nil, err
	}
	model, err := tools.ParseDockerComposeModel(resolved)
	if err != nil {
		return nil, err
	}
	preflight := &dockerComposePreflight{
		file:     composeFile,
		root:     filepath.Clean(root),
		raw:      strings.ToLower(string(raw)),
		resolved: strings.ToLower(resolved),
		model:    model,
	}
	allResolved, err := resolveDockerComposeConfig(ctx, cfg, composeFile, tools.DockerComposeConfigOptions{AllProfiles: true})
	if err == nil {
		var allModel tools.DockerComposeModel
		if allModel, err = tools.ParseDockerComposeModel(allResolved); err == nil {
			var allRaw dockerComposeRawModel
			if err = json.Unmarshal([]byte(allResolved), &allRaw); err == nil {
				preflight.allProfilesModel = &allModel
				preflight.allProfilesRaw = allRaw
			}
		}
	}
	preflight.allProfilesErr = err
	if ctx != nil && ctx.Err() != nil {
		return nil, fmt.Errorf("resolve Compose config: %w", ctx.Err())
	}
	return preflight, nil
}

// dockerComposeOutsideJailError is a compose file outside the jail: the agent
// workspace, or the process working directory when no workspace is configured.
type dockerComposeOutsideJailError struct {
	file                string
	root                string
	workspaceConfigured bool
	err                 error
}

func (e *dockerComposeOutsideJailError) Error() string { return e.err.Error() }
func (e *dockerComposeOutsideJailError) Unwrap() error { return e.err }

// dockerComposeFileReadLimit bounds the raw compose file the policy reads.
const dockerComposeFileReadLimit = 4 << 20

// readDockerComposeFile reads the jailed compose file only when it is a
// regular file of at most 4 MiB, so a FIFO or device cannot hang the call.
// os.Stat follows a symlink, whose target the jail has already accepted.
func readDockerComposeFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("read compose file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("read compose file: %s is not a regular file", path)
	}
	if info.Size() > dockerComposeFileReadLimit {
		return nil, fmt.Errorf("read compose file: %s is larger than 4 MiB", path)
	}
	handle, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read compose file: %w", err)
	}
	defer handle.Close()
	if opened, err := handle.Stat(); err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, fmt.Errorf("read compose file: %s changed while it was opened", path)
	}
	raw, err := io.ReadAll(io.LimitReader(handle, dockerComposeFileReadLimit+1))
	if err != nil {
		return nil, fmt.Errorf("read compose file: %w", err)
	}
	if len(raw) > dockerComposeFileReadLimit {
		return nil, fmt.Errorf("read compose file: %s is larger than 4 MiB", path)
	}
	return raw, nil
}

// effectiveModel returns what command runs (see dockerComposeEffectiveModel).
// The default model is always part of it; a service of an inactive profile is
// added only when command names it, together with the services it depends on
// (`up -d a` also starts a's same-profile dependency) and the services it
// names as additional build contexts (`service:b` builds b too). Resolved
// depends_on already includes links, volumes_from and network_mode service:
// references.
func (p *dockerComposePreflight) effectiveModel(command string) dockerComposeEffectiveModel {
	source := p.model
	if p.allProfilesModel != nil {
		source = *p.allProfilesModel
	}
	effective := dockerComposeEffectiveModel{fromAllProfiles: p.allProfilesModel != nil, model: tools.DockerComposeModel{
		Name:     p.model.Name,
		Services: make(map[string]tools.DockerComposeService, len(p.model.Services)),
		Volumes:  map[string]tools.DockerComposeNamedVolume{},
		Networks: map[string]tools.DockerComposeNetwork{},
		Secrets:  map[string]tools.DockerComposeFileResource{},
		Configs:  map[string]tools.DockerComposeFileResource{},
	}}
	for name, service := range p.model.Services {
		if definition, ok := source.Services[name]; ok {
			service = definition
		}
		effective.model.Services[name] = service
	}
	copyDockerComposeResources(effective.model.Volumes, p.model.Volumes, source.Volumes)
	copyDockerComposeResources(effective.model.Networks, p.model.Networks, source.Networks)
	copyDockerComposeResources(effective.model.Secrets, p.model.Secrets, source.Secrets)
	copyDockerComposeResources(effective.model.Configs, p.model.Configs, source.Configs)

	var missing []string
	for _, name := range dockerComposeStartedServiceNames(command) {
		if _, ok := p.model.Services[name]; !ok {
			missing = append(missing, name)
		}
	}
	if p.allProfilesModel == nil {
		// config/convert only print the model and create nothing; without the
		// all-profiles model they keep running as before, unchecked.
		if !dockerComposeRendersModel(command) {
			effective.unverified = missing
		}
		return effective
	}
	all, raw := p.allProfilesModel, p.allProfilesRaw
	var text strings.Builder
	for queue := missing; len(queue) > 0; queue = queue[1:] {
		name := queue[0]
		if _, done := effective.model.Services[name]; done {
			continue
		}
		service, ok := all.Services[name]
		if !ok {
			continue // Compose itself rejects a service no profile defines.
		}
		effective.model.Services[name] = service
		effective.profileServices = append(effective.profileServices, name)
		text.Write(raw.Services[name])
		text.WriteByte('\n')
		var refs struct {
			DependsOn json.RawMessage `json:"depends_on"`
			Networks  json.RawMessage `json:"networks"`
			Secrets   json.RawMessage `json:"secrets"`
			Configs   json.RawMessage `json:"configs"`
		}
		_ = json.Unmarshal(raw.Services[name], &refs)
		queue = append(queue, dockerComposeRefNames(refs.DependsOn)...)
		if build := service.Build; build != nil {
			for _, key := range sortedDockerComposeContextKeys(build.AdditionalContexts) {
				if target, ok := strings.CutPrefix(strings.TrimSpace(build.AdditionalContexts[key]), "service:"); ok {
					queue = append(queue, strings.TrimSpace(target))
				}
			}
			for _, key := range build.Secrets {
				addDockerComposeResource(effective.model.Secrets, all.Secrets, raw.Secrets, key, &text)
			}
		}
		for _, mount := range service.Volumes {
			if strings.EqualFold(strings.TrimSpace(mount.Type), "volume") {
				addDockerComposeResource(effective.model.Volumes, all.Volumes, raw.Volumes, mount.Source, &text)
			}
		}
		for _, key := range dockerComposeRefNames(refs.Networks) {
			addDockerComposeResource(effective.model.Networks, all.Networks, raw.Networks, key, &text)
		}
		for _, key := range dockerComposeRefNames(refs.Secrets) {
			addDockerComposeResource(effective.model.Secrets, all.Secrets, raw.Secrets, key, &text)
		}
		for _, key := range dockerComposeRefNames(refs.Configs) {
			addDockerComposeResource(effective.model.Configs, all.Configs, raw.Configs, key, &text)
		}
	}
	sort.Strings(effective.profileServices)
	effective.profileText = strings.ToLower(text.String())
	return effective
}

// copyDockerComposeResources copies the default model's top-level resources,
// preferring the all-profiles definition of each.
func copyDockerComposeResources[V any](target, defaults, source map[string]V) {
	for key, value := range defaults {
		if definition, ok := source[key]; ok {
			value = definition
		}
		target[key] = value
	}
}

// addDockerComposeResource adds one top-level resource an added profile
// service references, and its raw JSON to the token text.
func addDockerComposeResource[V any](target, source map[string]V, raw map[string]json.RawMessage, key string, text *strings.Builder) {
	if key == "" {
		return
	}
	if _, ok := target[key]; ok {
		return
	}
	value, ok := source[key]
	if !ok {
		return
	}
	target[key] = value
	text.Write(raw[key])
	text.WriteByte('\n')
}

// dockerComposeRefNames reads the names a service reference lists: the keys of
// a map, or the entries of a list of strings or of {"source": …} objects.
func dockerComposeRefNames(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var byName map[string]json.RawMessage
	if json.Unmarshal(raw, &byName) == nil {
		names := make([]string, 0, len(byName))
		for name := range byName {
			names = append(names, name)
		}
		sort.Strings(names)
		return names
	}
	var items []json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		return nil
	}
	var names []string
	for _, item := range items {
		var name string
		if json.Unmarshal(item, &name) == nil {
			names = append(names, name)
			continue
		}
		var ref struct {
			Source string `json:"source"`
		}
		if json.Unmarshal(item, &ref) == nil && ref.Source != "" {
			names = append(names, ref.Source)
		}
	}
	return names
}

// dockerComposeRendersModel reports a `config` or `convert` command.
func dockerComposeRendersModel(command string) bool {
	parts := strings.Fields(command)
	return len(parts) > 0 && (parts[0] == "config" || parts[0] == "convert")
}

// sortedDockerComposeContextKeys returns the keys of a build's
// additional_contexts in a stable order.
func sortedDockerComposeContextKeys(contexts map[string]string) []string {
	keys := make([]string, 0, len(contexts))
	for key := range contexts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// dockerComposeStartedServiceNames returns the services command names as
// positional arguments of `up`, `create`, `build`, `pull`, `config` or
// `convert`, including after `--`. Naming a service activates its profiles
// (verified with --dry-run): up/create start it, build builds it, pull pulls
// its image and config/convert print it. The service values of --scale,
// --exit-code-from, --attach and --no-attach do not, and start and restart
// only act on containers that already exist.
func dockerComposeStartedServiceNames(command string) []string {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil
	}
	valueFlags, shortValueFlags := dockerComposeValueFlags, dockerComposeShortValueFlags
	switch parts[0] {
	case "up", "create":
	case "build":
		valueFlags, shortValueFlags = dockerComposeBuildValueFlags, dockerComposeBuildShortValueFlags
	case "pull":
		valueFlags, shortValueFlags = dockerComposePullValueFlags, nil
	case "config", "convert":
		valueFlags, shortValueFlags = dockerComposeConfigValueFlags, dockerComposeConfigShortValueFlags
	default:
		return nil
	}
	var names []string
	positional := false
	for i := 1; i < len(parts); i++ {
		arg := parts[i]
		if positional || !strings.HasPrefix(arg, "-") {
			names = append(names, arg)
			continue
		}
		if arg == "--" {
			positional = true
			continue
		}
		if strings.Contains(arg, "=") {
			continue
		}
		if valueFlags[arg] ||
			(!strings.HasPrefix(arg, "--") && len(arg) > 2 && shortValueFlags[arg[len(arg)-1]]) {
			i++ // the flag's value, e.g. `-t 5` or the combined `-dt 5`
		}
	}
	return names
}

// dockerComposeValueFlags are the up/create flags whose value is a separate
// argument, so it is not mistaken for a service name.
var dockerComposeValueFlags = map[string]bool{
	"--attach": true, "--exit-code-from": true, "--no-attach": true, "--pull": true,
	"--scale": true, "-t": true, "--timeout": true, "--wait-timeout": true,
}

// dockerComposeShortValueFlags are the short up/create flags that take a value;
// in a combined form such as `-dt 5` the last letter takes the next argument.
var dockerComposeShortValueFlags = map[byte]bool{'t': true}

// dockerComposeBuildValueFlags are the build flags that take a separate value
// (--progress for older Compose releases). build's --pull takes none.
var dockerComposeBuildValueFlags = map[string]bool{
	"--build-arg": true, "--builder": true, "-m": true, "--memory": true,
	"--progress": true, "--provenance": true, "--sbom": true, "--ssh": true,
}

var dockerComposeBuildShortValueFlags = map[byte]bool{'m': true}

// dockerComposePullValueFlags are the pull flags that take a separate value.
var dockerComposePullValueFlags = map[string]bool{"--policy": true}

// dockerComposeConfigValueFlags are the config/convert flags that take a
// separate value.
var dockerComposeConfigValueFlags = map[string]bool{
	"--format": true, "--hash": true, "-o": true, "--output": true,
}

var dockerComposeConfigShortValueFlags = map[byte]bool{'o': true}

// protectedOwner keeps every text match that blocked a call before the model
// existed (LocalLLM on raw and resolved text, Garage and Homepage on raw text)
// and adds the structured model checks over the default model and what the
// command runs. Profile services the command starts add their resolved JSON
// to the LocalLLM text check. Garage and Homepage do not match the resolved
// text: resolved JSON carries project and path names that must not block
// unrelated stacks.
func (p *dockerComposePreflight) protectedOwner(effective dockerComposeEffectiveModel) string {
	if dockerComposePayloadReferencesProtectedLocalLLM(p.raw) ||
		dockerComposePayloadReferencesProtectedLocalLLM(p.resolved) ||
		(effective.profileText != "" && dockerComposePayloadReferencesProtectedLocalLLM(effective.profileText)) {
		return dockerutil.LocalLLMOwner
	}
	if dockerComposeTextReferencesGarage(p.raw) {
		return dockerutil.BoringGarageOwner
	}
	if dockerComposeTextReferencesHomepage(p.raw) {
		return dockerutil.HomepageOwner
	}
	if owner := tools.DockerComposeModelOwner(p.model); owner != "" {
		return owner
	}
	return tools.DockerComposeModelOwner(effective.model)
}

func dockerComposeTextReferencesGarage(lower string) bool {
	for _, token := range []string{
		dockerutil.BoringGarageContainerName,
		"boring-garage",
		"data/sidecars/garage",
		`aurago.managed: boring-garage`,
		`"aurago.managed":"boring-garage"`,
		`aurago.managed=boring-garage`,
	} {
		if strings.Contains(lower, strings.ToLower(token)) {
			return true
		}
	}
	return false
}

func dockerComposeTextReferencesHomepage(lower string) bool {
	return strings.Contains(lower, dockerutil.HomepageContainerName) ||
		strings.Contains(lower, dockerutil.HomepageWebContainerName) ||
		strings.Contains(lower, dockerutil.HomepageImageRepository)
}

// dockerComposeReferencesProtectedLocalLLMVolume is the fail-closed LocalLLM
// predicate pinned by TestManagedLocalLLMComposeProtectionIsFailClosed.
func dockerComposeReferencesProtectedLocalLLMVolume(cfg tools.DockerConfig, file string) bool {
	preflight, err := loadDockerComposePreflight(context.Background(), cfg, file)
	if err != nil {
		return true
	}
	owner := preflight.protectedOwner(preflight.effectiveModel(""))
	return owner == dockerutil.LocalLLMOwner || owner == acestep.Owner
}

// dockerComposePolicy runs before every agent Compose call. It returns a Tool
// Output envelope that blocks the call, or "" to continue. The ownership
// checks apply to every subcommand; the host-access policy only to
// up/create/build and, for AuraGo state, config/convert.
func dockerComposePolicy(ctx context.Context, cfg *config.Config, dockerCfg tools.DockerConfig, req dockerArgs) string {
	preflight, err := loadDockerComposePreflight(ctx, dockerCfg, req.File)
	var outside *dockerComposeOutsideJailError
	if errors.As(err, &outside) {
		if !outside.workspaceConfigured {
			return dockerAgentError("docker_compose_file_outside_workspace", fmt.Sprintf(
				"No agent workspace is configured, so compose files must stay inside AuraGo's working directory %s. The compose file %q is outside it (directly or through a symlink), so nothing was run.", outside.root, outside.file))
		}
		return dockerAgentError("docker_compose_file_outside_workspace", fmt.Sprintf(
			"The compose file %q is outside the agent workspace %s (directly or through a symlink), so nothing was run. Use a compose file inside the workspace.", outside.file, outside.root))
	}
	if err != nil {
		return dockerAgentError("docker_compose_preflight_failed",
			"Docker Compose could not resolve this file, so nothing was run: "+dockerComposeErrorTail(err.Error(), 600)+
				". Fix the Compose file (for example a missing env_file or invalid YAML) or install the Docker Compose plugin.")
	}
	if preflight.allProfilesErr != nil {
		slog.Default().Warn("Docker Compose could not resolve all profiles; commands naming a service of an inactive profile are denied and env_file paths are unknown",
			"file", preflight.file, "error", dockerComposeErrorTail(preflight.allProfilesErr.Error(), 600))
	}
	effective := preflight.effectiveModel(req.Command)
	if denied := dockerComposeOwnerDenial(preflight.protectedOwner(effective)); denied != "" {
		return denied
	}
	if len(effective.unverified) > 0 {
		return dockerAgentError("docker_compose_profile_service_unverified", fmt.Sprintf(
			"Service %q is not part of the default Compose profiles, and this Docker Compose version cannot resolve services of inactive profiles for AuraGo's ownership check, so nothing was run. Profile checks need Docker Compose v2.35 or newer: update the Compose plugin, or start only services without a profile.", effective.unverified[0]))
	}
	return dockerComposeHostAccessPolicy(ctx, cfg, req, preflight, effective)
}

// dockerComposeErrorTail bounds a preflight error and keeps its end: Compose
// reports the actual error last.
func dockerComposeErrorTail(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	const marker = "…"
	start := len(text) - (limit - len(marker))
	for start < len(text) && !utf8.RuneStart(text[start]) {
		start++
	}
	return marker + text[start:]
}

func dockerComposeOwnerDenial(owner string) string {
	switch owner {
	case dockerutil.LocalLLMOwner, acestep.Owner:
		return `Tool Output: {"status":"error","message":"Docker Compose access to AuraGo's managed local LLM volumes is blocked."}`
	case dockerutil.BoringGarageOwner:
		return `Tool Output: {"status":"error","message":"Docker Compose access to AuraGo's managed Boring Computers Garage is blocked."}`
	case dockerutil.HomepageOwner:
		return dockerAgentError("docker_managed_homepage_resource", "Docker Compose access to AuraGo-managed homepage resources is blocked. Use homepage_project, homepage_file, or homepage_deploy.")
	case dockerutil.AppOwner:
		return dockerAgentError("docker_managed_aurago_resource", "Docker Compose access to AuraGo's application container is blocked.")
	}
	return ""
}
