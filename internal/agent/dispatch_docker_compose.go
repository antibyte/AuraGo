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
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
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
	root string
	// dockerCfg is the Docker config the file was resolved with (WorkspaceDir
	// set to root), for the follow-up resolution of named profile services.
	dockerCfg tools.DockerConfig
	raw       string
	resolved  string
	model     tools.DockerComposeModel
	// allProfilesModel is `--profile * config --no-env-resolution`: every
	// service, including inactive profiles that `up <service>` activates, with
	// env_file entries kept as paths; allProfilesRaw holds the same resolution
	// as raw JSON per service and resource. Policies never read them directly:
	// effectiveModel takes from them only what a command runs. Both are empty
	// when this Compose cannot produce the model (allProfilesErr).
	allProfilesModel *tools.DockerComposeModel
	allProfilesRaw   dockerComposeRawModel
	allProfilesErr   error
	// self is AuraGo's own container (zero on native installs), read once per
	// call by dockerComposePolicy.
	self tools.DockerSelfIdentity
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
	// kept as paths), otherwise from the default model and, after
	// resolveNamedProfileServices, from Compose's resolution of the named
	// services.
	model tools.DockerComposeModel
	// profileServices lists the services added from inactive profiles, sorted.
	profileServices []string
	// unverified lists named services outside the default model that are not
	// resolved yet because this Compose cannot produce the all-profiles model;
	// resolveNamedProfileServices resolves them.
	unverified []string
	// profileText is the lower-cased raw JSON of the added services and the
	// resources they add, for the text token checks; after
	// resolveNamedProfileServices it also holds their env-resolved JSON.
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
		file:      composeFile,
		root:      filepath.Clean(root),
		dockerCfg: cfg,
		raw:       strings.ToLower(string(raw)),
		resolved:  strings.ToLower(resolved),
		model:     model,
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
		effective.unverified = missing
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
			Networks json.RawMessage `json:"networks"`
			Secrets  json.RawMessage `json:"secrets"`
			Configs  json.RawMessage `json:"configs"`
		}
		_ = json.Unmarshal(raw.Services[name], &refs)
		queue = append(queue, dockerComposeServiceDependencies(raw.Services[name], service)...)
		if build := service.Build; build != nil {
			for _, key := range build.Secrets {
				addDockerComposeResource(effective.model.Secrets, all.Secrets, raw.Secrets, key, &text)
			}
		}
		for _, mount := range service.Volumes {
			if strings.EqualFold(strings.TrimSpace(mount.Type), "volume") {
				addDockerComposeResource(effective.model.Volumes, all.Volumes, raw.Volumes, mount.Source, &text)
			}
		}
		for _, key := range tools.DockerComposeRefNames(refs.Networks) {
			addDockerComposeResource(effective.model.Networks, all.Networks, raw.Networks, key, &text)
		}
		for _, key := range tools.DockerComposeRefNames(refs.Secrets) {
			addDockerComposeResource(effective.model.Secrets, all.Secrets, raw.Secrets, key, &text)
		}
		for _, key := range tools.DockerComposeRefNames(refs.Configs) {
			addDockerComposeResource(effective.model.Configs, all.Configs, raw.Configs, key, &text)
		}
	}
	sort.Strings(effective.profileServices)
	effective.profileText = strings.ToLower(text.String())
	return effective
}

// dockerComposeServiceDependencies returns the services Compose enables
// together with service: its depends_on entries (resolved depends_on already
// includes links, volumes_from and network_mode service: references) and the
// services it names as additional build contexts (`service:b`).
func dockerComposeServiceDependencies(raw json.RawMessage, service tools.DockerComposeService) []string {
	var refs struct {
		DependsOn json.RawMessage `json:"depends_on"`
	}
	_ = json.Unmarshal(raw, &refs)
	names := tools.DockerComposeRefNames(refs.DependsOn)
	if build := service.Build; build != nil {
		for _, key := range tools.SortedDockerComposeKeys(build.AdditionalContexts) {
			if target, ok := strings.CutPrefix(strings.TrimSpace(build.AdditionalContexts[key]), "service:"); ok {
				names = append(names, strings.TrimSpace(target))
			}
		}
	}
	return names
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

// dockerComposeNamedResolutionRounds bounds the repeated named resolution that
// closes over `service:` build contexts.
const dockerComposeNamedResolutionRounds = 8

// resolveNamedProfileServices resolves the inactive-profile services the
// command runs with `config --format json -- <names>` (Compose v2.0+), which
// activates their profiles and returns them with their depends_on closure and
// env files inlined; the call is repeated to close over `service:` build
// contexts. Without the all-profiles model (Compose < v2.35) the result adds
// the unverified services' definitions (env file paths stay unknown) and
// clears unverified; with it, the result only adds the env-resolved text of
// effective.profileServices for the text checks. It returns false when the
// call fails, or, without the all-profiles model, when it does not return a
// named service.
func (p *dockerComposePreflight) resolveNamedProfileServices(ctx context.Context, effective *dockerComposeEffectiveModel) bool {
	names := effective.profileServices
	if p.allProfilesModel == nil {
		names = effective.unverified
	}
	if len(names) == 0 {
		return true
	}
	names = append([]string(nil), names...)
	var model tools.DockerComposeModel
	var raw dockerComposeRawModel
	for round := 1; ; round++ {
		resolved, err := resolveDockerComposeConfig(ctx, p.dockerCfg, p.file, tools.DockerComposeConfigOptions{Services: names})
		if err == nil {
			if model, err = tools.ParseDockerComposeModel(resolved); err == nil {
				raw = dockerComposeRawModel{}
				err = json.Unmarshal([]byte(resolved), &raw)
			}
		}
		if err != nil {
			slog.Default().Warn("Docker Compose could not resolve the named profile services", "file", p.file,
				"services", strings.Join(names, ","), "error", dockerComposeErrorTail(err.Error(), 600))
			return false
		}
		if p.allProfilesModel != nil {
			break // the all-profiles closure already named every service
		}
		var more []string
		for _, name := range names {
			if _, ok := model.Services[name]; !ok {
				return false
			}
		}
		for _, name := range tools.SortedDockerComposeKeys(model.Services) {
			build := model.Services[name].Build
			if build == nil {
				continue
			}
			for _, key := range tools.SortedDockerComposeKeys(build.AdditionalContexts) {
				target, ok := strings.CutPrefix(strings.TrimSpace(build.AdditionalContexts[key]), "service:")
				target = strings.TrimSpace(target)
				if !ok || target == "" || slices.Contains(names, target) || slices.Contains(more, target) {
					continue
				}
				if _, returned := model.Services[target]; returned {
					continue
				}
				if _, isDefault := p.model.Services[target]; isDefault {
					continue
				}
				more = append(more, target)
			}
		}
		if len(more) == 0 {
			break
		}
		if round >= dockerComposeNamedResolutionRounds {
			return false
		}
		names = append(names, more...)
	}

	var text strings.Builder
	if p.allProfilesModel != nil {
		for _, name := range effective.profileServices {
			text.Write(raw.Services[name])
			text.WriteByte('\n')
		}
	} else {
		for _, name := range tools.SortedDockerComposeKeys(model.Services) {
			if _, present := effective.model.Services[name]; present {
				continue
			}
			effective.model.Services[name] = model.Services[name]
			effective.profileServices = append(effective.profileServices, name)
			text.Write(raw.Services[name])
			text.WriteByte('\n')
		}
		sort.Strings(effective.profileServices)
		effective.unverified = nil
	}
	addDockerComposeResources(effective.model.Volumes, model.Volumes, raw.Volumes, &text)
	addDockerComposeResources(effective.model.Networks, model.Networks, raw.Networks, &text)
	addDockerComposeResources(effective.model.Secrets, model.Secrets, raw.Secrets, &text)
	addDockerComposeResources(effective.model.Configs, model.Configs, raw.Configs, &text)
	if text.Len() > 0 {
		effective.profileText += "\n" + strings.ToLower(text.String())
	}
	return true
}

// addDockerComposeResources adds the top-level resources of a named
// resolution that the effective model lacks, and the raw JSON of every
// resource to the token text (env-resolved values matter to the text checks).
func addDockerComposeResources[V any](target, source map[string]V, raw map[string]json.RawMessage, text *strings.Builder) {
	for _, key := range tools.SortedDockerComposeKeys(source) {
		if _, ok := target[key]; !ok {
			target[key] = source[key]
		}
		text.Write(raw[key])
		text.WriteByte('\n')
	}
}

// dockerComposeLifecycleServiceNames returns the services a `down`, `stop`,
// `start`, `restart` or `rm` command names as positional arguments. Naming a
// service activates its profiles, so the host-access policy checks what these
// commands run on the host for them (providers, privileged hooks); they never
// feed the ownership or unverified-profile checks.
func dockerComposeLifecycleServiceNames(command string) []string {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil
	}
	switch parts[0] {
	case "down", "stop", "start", "restart", "rm":
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
		if dockerComposeLifecycleValueFlags[arg] || dockerComposeShortFlagTakesNext(arg, dockerComposeLifecycleShortValueFlags) {
			i++
		}
	}
	return names
}

// dockerComposeLifecycleValueFlags are the down/stop/start/restart/rm flags
// whose value is a separate argument.
var dockerComposeLifecycleValueFlags = map[string]bool{
	"-t": true, "--timeout": true, "--rmi": true, "--wait-timeout": true,
}

// dockerComposeLifecycleShortValueFlags are their short flags that take a
// value; in a cluster such as `-vt 5` the last letter takes the next argument.
var dockerComposeLifecycleShortValueFlags = map[byte]bool{'t': true}

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
		if valueFlags[arg] || dockerComposeShortFlagTakesNext(arg, shortValueFlags) {
			i++ // the flag's value, e.g. `-t 5` or the combined `-dt 5`
		}
	}
	return names
}

// dockerComposeShortFlagTakesNext reports whether a short flag cluster such as
// `-dt` or `-qo` ends in a value-taking flag, whose value is then the next
// argument. A value-taking letter earlier in the cluster has its value
// attached (`-ofile.yml`, `-t5`), so the next argument is a positional one,
// even when the attached value happens to end in the flag letter (`-ologo`).
func dockerComposeShortFlagTakesNext(arg string, valueFlags map[byte]bool) bool {
	if strings.HasPrefix(arg, "--") {
		return false
	}
	for i := 1; i < len(arg); i++ {
		if valueFlags[arg[i]] {
			return i == len(arg)-1
		}
	}
	return false
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
	if ctx == nil {
		ctx = context.Background()
	}
	// One deadline for every Compose resolution of this call (default,
	// all-profiles and the named rounds); each call is also bounded by 20 s.
	ctx, cancel := context.WithTimeout(ctx, dockerComposePreflightTimeout)
	defer cancel()
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
		logDockerComposeAllProfilesFailure(preflight.file, preflight.allProfilesErr)
	}
	subcommand := dockerComposeCommandName(req.Command)
	preflight.self = tools.DockerSelfIdentityFor(ctx, dockerCfg)
	if denied := dockerComposeSelfProjectDenial(preflight.model.Name, subcommand, preflight.self); denied != "" {
		return denied
	}
	effective := preflight.effectiveModel(req.Command)
	if preflight.allProfilesModel == nil && len(effective.unverified) > 0 {
		// Compose < v2.35 resolves the named profile services by name instead.
		// That resolution reads env files, which build and pull never need.
		if !preflight.resolveNamedProfileServices(ctx, &effective) {
			switch {
			case subcommand == "config" || subcommand == "convert":
				// They create nothing and Compose reports the same failure.
				effective.unverified = nil
			case (subcommand == "build" || subcommand == "pull") && dockerComposeHostAccessAllowed(ctx, cfg):
				if path, found := dockerComposeRawStateReference(preflight.raw, cfg); found {
					return dockerComposeViolationOutput([]tools.DockerComposeViolation{{Field: "compose file", Value: path, Always: true,
						Reason: "the file names AuraGo's own data, configuration or master key, and this Docker Compose cannot resolve the named profile services for a full check"}})
				}
				slog.Default().Warn("Docker Compose could not resolve the named profile services; running the command unchecked because docker.allow_host_access is on",
					"file", preflight.file, "command", subcommand, "services", strings.Join(effective.unverified, ","))
				effective.unverified = nil
			}
		}
	}
	if denied := dockerComposeOwnerDenial(preflight.protectedOwner(effective)); denied != "" {
		return denied
	}
	if len(effective.unverified) > 0 {
		return dockerComposeProfileServiceUnverified(effective.unverified[0])
	}
	if preflight.allProfilesModel != nil && len(effective.profileServices) > 0 &&
		(subcommand == "up" || subcommand == "create" || subcommand == "config" || subcommand == "convert") {
		// The all-profiles model keeps env files as paths; the named
		// resolution adds the env-resolved text of the profile services that
		// up/create start and config/convert print. build and pull never read
		// env files, so they keep the all-profiles definitions only.
		resolvedFrom := len(effective.profileText)
		if !preflight.resolveNamedProfileServices(ctx, &effective) {
			if subcommand == "up" || subcommand == "create" {
				return dockerComposeProfileServiceUnverified(effective.profileServices[0])
			}
		} else if dockerComposePayloadReferencesProtectedLocalLLM(effective.profileText[resolvedFrom:]) {
			return dockerComposeOwnerDenial(dockerutil.LocalLLMOwner)
		}
	}
	return dockerComposeHostAccessPolicy(ctx, cfg, req, preflight, effective)
}

// dockerComposeAllProfilesWarned remembers the (file, error) pairs whose
// all-profiles failure was logged at warning level; later calls log at debug.
// The set is bounded and starts over when full.
var dockerComposeAllProfilesWarned = struct {
	sync.Mutex
	seen map[string]bool
}{seen: map[string]bool{}}

const dockerComposeAllProfilesWarnLimit = 256

// logDockerComposeAllProfilesFailure logs that this Compose cannot produce the
// all-profiles model (Compose < v2.35 fails on every call) at warning level
// once per file and error, and at debug level afterwards.
func logDockerComposeAllProfilesFailure(file string, err error) {
	const message = "Docker Compose could not resolve all profiles; named profile services are resolved by name and env_file paths are unknown"
	detail := dockerComposeErrorTail(err.Error(), 600)
	key := file + "\x00" + detail
	dockerComposeAllProfilesWarned.Lock()
	first := !dockerComposeAllProfilesWarned.seen[key]
	if first {
		if len(dockerComposeAllProfilesWarned.seen) >= dockerComposeAllProfilesWarnLimit {
			dockerComposeAllProfilesWarned.seen = map[string]bool{}
		}
		dockerComposeAllProfilesWarned.seen[key] = true
	}
	dockerComposeAllProfilesWarned.Unlock()
	if first {
		slog.Default().Warn(message, "file", file, "error", detail)
		return
	}
	slog.Default().Debug(message, "file", file, "error", detail)
}

// dockerComposePreflightTimeout bounds all Compose resolutions of one policy
// call together.
const dockerComposePreflightTimeout = 60 * time.Second

// dockerComposeCommandName returns the first word of a Compose command.
func dockerComposeCommandName(command string) string {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

// dockerComposeProfileServiceUnverified denies a command naming a profile
// service that Compose could not resolve for the policy checks.
func dockerComposeProfileServiceUnverified(service string) string {
	return dockerAgentError("docker_compose_profile_service_unverified", fmt.Sprintf(
		"Service %q is not part of the default Compose profiles, and Docker Compose could not resolve it for AuraGo's ownership and host-access checks, so nothing was run. Check that the service exists and its env files are present; Docker Compose v2.35 or newer resolves profile services most reliably: update the Compose plugin, or start only services without a profile.", service))
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
		return dockerAgentError("docker_managed_garage_resource", "Docker Compose access to AuraGo's managed Boring Computers Garage is blocked.")
	case dockerutil.HomepageOwner:
		return dockerAgentError("docker_managed_homepage_resource", "Docker Compose access to AuraGo-managed homepage resources is blocked. Use homepage_project, homepage_file, or homepage_deploy.")
	case dockerutil.AppOwner:
		return dockerAgentError("docker_managed_aurago_resource", "Docker Compose access to AuraGo's application container is blocked.")
	}
	return ""
}
