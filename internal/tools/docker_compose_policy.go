package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"aurago/internal/acestep"
	"aurago/internal/dockerutil"
)

// ErrDockerComposeFileOutsideWorkspace marks a compose file, or the target of a
// compose file symlink, outside the workspace jail.
var ErrDockerComposeFileOutsideWorkspace = errors.New("compose file is outside the workspace")

type dockerComposeJailError struct{ message string }

func (e *dockerComposeJailError) Error() string { return e.message }
func (e *dockerComposeJailError) Unwrap() error { return ErrDockerComposeFileOutsideWorkspace }

// ResolveDockerComposeFile applies the workspace jail DockerCompose uses and
// returns the absolute file Compose will read. A file (or symlink target)
// outside cfg.WorkspaceDir fails with ErrDockerComposeFileOutsideWorkspace. An
// empty WorkspaceDir applies no jail, as in DockerCompose; the agent Compose
// preflight therefore passes the process working directory as WorkspaceDir when
// no workspace is configured, confining the file to it. The preflight resolves
// the file twice: the default `config` (the validity gate) and the all-profiles
// `--profile * config --no-env-resolution` model that the ownership checks read.
func ResolveDockerComposeFile(cfg DockerConfig, file string) (string, error) {
	if strings.TrimSpace(file) == "" {
		return "", fmt.Errorf("compose file is required")
	}
	return validateDockerComposeFilePath(cfg, file)
}

// DockerComposeConfigOptions selects the `docker compose config` variant.
type DockerComposeConfigOptions struct {
	// AllProfiles resolves every service, including those of inactive profiles
	// that `up <service>` would activate: `--profile * config --format json
	// --no-env-resolution`. env_file entries stay paths instead of being inlined,
	// so a missing env_file of an inactive profile does not fail the resolution.
	AllProfiles bool
	// Services resolves only the named services, activating their profiles:
	// `config --format json -- <services>` (Compose v2.0+). The result holds
	// them and their depends_on closure with env files inlined; an unknown
	// name fails the call.
	Services []string
}

func dockerComposeConfigArgs(cfg DockerConfig, composeFile string, opts DockerComposeConfigOptions) []string {
	args := []string{"compose", "-f", composeFile}
	if opts.AllProfiles {
		args = append(args, "--profile", "*")
	}
	args = append(args, "config", "--format", "json")
	if opts.AllProfiles {
		args = append(args, "--no-env-resolution")
	}
	if len(opts.Services) > 0 {
		args = append(append(args, "--"), opts.Services...)
	}
	return dockerCLIArgs(cfg, args...)
}

// runDockerComposeConfig runs one `docker compose ... config` invocation and
// keeps stdout and stderr apart. Tests replace it.
var runDockerComposeConfig = func(ctx context.Context, args []string) ([]byte, []byte, error) {
	return runDockerComposeConfigCommand(dockerCLICommand(ctx, args...))
}

// runDockerComposeConfigMinimal is runDockerComposeConfig with the minimal
// environment (DockerConfig.MinimalCLIEnvironment). Tests replace it.
var runDockerComposeConfigMinimal = func(ctx context.Context, args []string) ([]byte, []byte, error) {
	return runDockerComposeConfigCommand(dockerCLICommandFor(ctx, DockerConfig{MinimalCLIEnvironment: true}, args...))
}

func runDockerComposeConfigCommand(cmd *exec.Cmd) ([]byte, []byte, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// dockerComposeConfigResult returns the model from stdout only. Compose prints
// warnings such as "the attribute `version` is obsolete" on stderr; they are
// dropped, and a failure carries only the sanitised error detail.
func dockerComposeConfigResult(stdout, stderr []byte, err error) (string, error) {
	if err != nil {
		detail := dockerComposeStderrDetail(stderr)
		if detail == "" {
			return "", fmt.Errorf("resolve Compose config: %w", err)
		}
		return "", fmt.Errorf("resolve Compose config: %w: %s", err, detail)
	}
	return string(stdout), nil
}

const (
	dockerComposeErrorDetailLimit = 480
	dockerComposeErrorDetailLines = 3
	dockerComposeQuotedSpanLimit  = 64
)

var (
	dockerComposeQuotedSpan = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
	dockerComposeLogLevel   = regexp.MustCompile(`(?i)\blevel=(\w+)`)
	dockerComposeLogMessage = regexp.MustCompile(`\bmsg=("(?:[^"\\]|\\.)*")`)
	// Compose's dotenv parser echoes env-file content: `unexpected character
	// "X" in variable name "<line>"` and `Invalid template: "<value>"` (Go %q),
	// and `unterminated quoted value <rest of the value>` (raw, without a
	// closing quote).
	dockerComposeEnvEchoQuoted       = regexp.MustCompile(`(unexpected character |variable name |Invalid template: )"(?:[^"\\]|\\.)*"`)
	dockerComposeEnvEchoUnterminated = regexp.MustCompile(`(?s)(unterminated quoted value ).*$`)
)

// dockerComposeStderrDetail keeps the part of Compose's stderr that explains a
// failure. Warning, info and debug lines are dropped (Compose prints them before
// the error), only the last few remaining lines are kept, the message of a
// logrus-formatted error line is unquoted, env-file content the dotenv parser
// echoes is redacted at any length, other double-quoted spans longer than 64
// characters become "…" because Compose quotes file content in them, and an
// over-long detail keeps its end, where the error is.
func dockerComposeStderrDetail(stderr []byte) string {
	// The unterminated-value echo runs to the end of the output, possibly over
	// several lines, so it is cut before the text is split into lines.
	text := dockerComposeEnvEchoUnterminated.ReplaceAllString(string(stderr), "${1}…")
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || dockerComposeNoiseLine(line) {
			continue
		}
		if match := dockerComposeLogMessage.FindStringSubmatch(line); match != nil && dockerComposeLogLevel.MatchString(line) {
			if message, err := strconv.Unquote(match[1]); err == nil {
				line = message
			}
		}
		lines = append(lines, dockerComposeEnvEchoQuoted.ReplaceAllString(line, `${1}"…"`))
	}
	if len(lines) > dockerComposeErrorDetailLines {
		lines = lines[len(lines)-dockerComposeErrorDetailLines:]
	}
	detail := dockerComposeQuotedSpan.ReplaceAllStringFunc(strings.Join(lines, " | "), func(span string) string {
		if utf8.RuneCountInString(span)-2 > dockerComposeQuotedSpanLimit {
			return `"…"`
		}
		return span
	})
	return dockerComposeKeepEnd(detail, dockerComposeErrorDetailLimit)
}

func dockerComposeNoiseLine(line string) bool {
	if match := dockerComposeLogLevel.FindStringSubmatch(line); match != nil {
		switch strings.ToLower(match[1]) {
		case "warning", "warn", "info", "debug", "trace":
			return true
		}
		return false
	}
	upper := strings.ToUpper(line)
	return strings.HasPrefix(upper, "WARN") || strings.HasPrefix(upper, "WARNING:")
}

// dockerComposeKeepEnd shortens text to at most limit bytes, keeping its end.
func dockerComposeKeepEnd(text string, limit int) string {
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

// DockerComposeModelOwner reports the first AuraGo owner whose resources the
// resolved model names, or "". It checks container_name, aurago.managed labels
// (canonical and legacy) on services, top-level volumes and networks, the
// homepage image repository, named and bind volume sources, local bind volume
// devices, volumes_from and container: namespace references. Owners: acestep.Owner, dockerutil.LocalLLMOwner,
// DecisionsOwner, BoringGarageOwner, HomepageOwner, AppOwner and CYDCreatorOwner (the exact
// builder container name, its two exact volume names and the published builder
// image as a service image or build tag). The app container matches only
// its exact name "aurago" or the aurago-app owner label: this tier also binds
// grandfathered configs, and user stacks name containers like "dev-aurago".
func DockerComposeModelOwner(model DockerComposeModel) string {
	for _, name := range sortedDockerComposeKeys(model.Services) {
		service := model.Services[name]
		if owner := dockerComposeContainerOwner(service.ContainerName); owner != "" {
			return owner
		}
		if owner := dockerComposeLabelOwner(service.Labels); owner != "" {
			return owner
		}
		if dockerutil.IsHomepageImageReference(service.Image) {
			return dockerutil.HomepageOwner
		}
		if dockerComposeServiceUsesImage(service, func(ref string) bool { return dockerutil.IsCYDCreatorImageReference(ref, "") }) {
			// build would tag (and pull, push or down --rmi would touch) the
			// published builder image.
			return dockerutil.CYDCreatorOwner
		}
		for _, mount := range service.Volumes {
			if owner := dockerComposeMountOwner(model, mount); owner != "" {
				return owner
			}
		}
		for _, ref := range service.VolumesFrom {
			target, isContainer := dockerComposeVolumesFromTarget(ref)
			if _, sameProject := model.Services[target]; sameProject && !isContainer {
				continue
			}
			if owner := dockerComposeContainerOwner(target); owner != "" {
				return owner
			}
		}
		for _, mode := range []string{service.NetworkMode, service.Pid, service.Ipc} {
			if target, ok := strings.CutPrefix(strings.TrimSpace(mode), "container:"); ok {
				if owner := dockerComposeContainerOwner(target); owner != "" {
					return owner
				}
			}
		}
	}
	for _, key := range sortedDockerComposeKeys(model.Volumes) {
		volume := model.Volumes[key]
		if owner := dockerComposeVolumeOwner(volume.Name); owner != "" {
			return owner
		}
		if dockerPathUsesRustFSVolume(volume.DriverOpts["device"]) {
			return dockerutil.RustFSOwner
		}
		if dockerutil.IsBoringGarageDataPath(volume.DriverOpts["device"]) {
			return dockerutil.BoringGarageOwner
		}
		if owner := dockerComposeLabelOwner(volume.Labels); owner != "" {
			return owner
		}
	}
	for _, key := range sortedDockerComposeKeys(model.Networks) {
		if owner := dockerComposeLabelOwner(model.Networks[key].Labels); owner != "" {
			return owner
		}
	}
	return ""
}

// dockerComposeServiceUsesImage reports whether matches accepts the service's
// image or one of its build tags.
func dockerComposeServiceUsesImage(service DockerComposeService, matches func(string) bool) bool {
	if matches(service.Image) {
		return true
	}
	if service.Build != nil {
		for _, tag := range service.Build.Tags {
			if matches(tag) {
				return true
			}
		}
	}
	return false
}

// DockerComposeModelUsesImage reports whether a service of the model uses an
// image reference that matches accepts (image or build tag); the agent passes
// the configured CYD Creator builder image.
func DockerComposeModelUsesImage(model DockerComposeModel, matches func(string) bool) bool {
	for _, name := range sortedDockerComposeKeys(model.Services) {
		if dockerComposeServiceUsesImage(model.Services[name], matches) {
			return true
		}
	}
	return false
}

// DockerComposeContainerReferences returns the containers outside the project
// that the model's services reach into: container: targets of network_mode,
// pid and ipc, and volumes_from entries that are container: references or name
// no service of the model. Docker resolves such a target as a name, a full ID
// or an ID prefix, so the agent asks Docker who owns each one
// (DockerContainerOwnership); the name rules of DockerComposeModelOwner alone
// miss an ID or a renamed container. Sorted and deduplicated.
func DockerComposeContainerReferences(model DockerComposeModel) []string {
	seen := map[string]bool{}
	var targets []string
	add := func(target string) {
		// Docker resolves "/name" like "name"; a target that is still no
		// valid name afterwards ("//x") is refused by the agent, never skipped.
		target = strings.TrimPrefix(strings.TrimSpace(target), "/")
		if target != "" && !seen[target] {
			seen[target] = true
			targets = append(targets, target)
		}
	}
	for _, name := range sortedDockerComposeKeys(model.Services) {
		service := model.Services[name]
		for _, mode := range []string{service.NetworkMode, service.Pid, service.Ipc} {
			if target, ok := strings.CutPrefix(strings.TrimSpace(mode), "container:"); ok {
				add(target)
			}
		}
		for _, ref := range service.VolumesFrom {
			if strings.HasPrefix(strings.TrimSpace(ref), "service:") {
				// A service of this project, like network_mode "service:".
				continue
			}
			target, isContainer := dockerComposeVolumesFromTarget(ref)
			if _, sameProject := model.Services[target]; sameProject && !isContainer {
				continue
			}
			add(target)
		}
	}
	sort.Strings(targets)
	return targets
}

// dockerComposeVolumesFromTarget returns the service or container a
// volumes_from entry names and whether it carried the "container:" prefix.
// Everything after the first ':' is the access mode, which Compose keeps as
// written (":ro", ":rw", ":z", ":Z", ":ro,z", ":cached") and the Engine accepts;
// container and service names cannot contain ':'.
func dockerComposeVolumesFromTarget(ref string) (target string, isContainer bool) {
	target, isContainer = strings.CutPrefix(strings.TrimSpace(ref), "container:")
	target, _, _ = strings.Cut(target, ":")
	return strings.TrimSpace(target), isContainer
}

// dockerComposeLabelOwner reports the AuraGo owner named by canonical or legacy
// aurago.managed labels on a service, volume or network.
func dockerComposeLabelOwner(labels map[string]string) string {
	for _, owner := range []string{acestep.Owner, dockerutil.LocalLLMOwner, dockerutil.DecisionsOwner, dockerutil.BoringGarageOwner, dockerutil.HomepageOwner, dockerutil.AppOwner, dockerutil.CYDCreatorOwner, dockerutil.OllamaOwner, dockerutil.RustFSOwner} {
		if dockerutil.ManagedBy(labels, owner) {
			return owner
		}
	}
	return ""
}

func dockerComposeContainerOwner(name string) string {
	name = strings.TrimSpace(name)
	switch {
	case dockerutil.IsRustFSContainerName(name):
		return dockerutil.RustFSOwner
	case dockerutil.IsOllamaContainerName(name):
		return dockerutil.OllamaOwner
	case name == "":
		return ""
	case acestep.IsResourceName(name):
		return acestep.Owner
	case dockerutil.IsLocalLLMContainerName(name):
		return dockerutil.LocalLLMOwner
	case dockerutil.IsDecisionsContainerName(name):
		return dockerutil.DecisionsOwner
	case dockerutil.IsBoringGarageContainerName(name):
		return dockerutil.BoringGarageOwner
	case dockerutil.IsHomepageContainerName(name):
		return dockerutil.HomepageOwner
	case dockerutil.IsCYDCreatorBuilderContainerName(name):
		return dockerutil.CYDCreatorOwner
	case strings.EqualFold(strings.TrimPrefix(name, "/"), dockerutil.AppContainerName):
		// Exact name only. dockerutil.IsAuraGoAppContainerName also matches
		// "*-aurago"/"*_aurago" replicas; the agent create path keeps using it,
		// but for Compose it would block user stacks named e.g. "dev-aurago".
		return dockerutil.AppOwner
	}
	return ""
}

func dockerComposeVolumeOwner(name string) string {
	name = strings.TrimSpace(name)
	switch {
	case dockerutil.IsRustFSVolumeName(name):
		return dockerutil.RustFSOwner
	case dockerutil.IsOllamaVolumeName(name):
		return dockerutil.OllamaOwner
	case name == "":
		return ""
	case acestep.IsResourceName(name):
		return acestep.Owner
	case dockerutil.IsLocalLLMVolumeName(name):
		return dockerutil.LocalLLMOwner
	case dockerutil.IsBoringGarageContainerName(name):
		return dockerutil.BoringGarageOwner
	case dockerutil.IsCYDCreatorVolumeName(name):
		return dockerutil.CYDCreatorOwner
	}
	return ""
}

func dockerComposeMountOwner(model DockerComposeModel, mount DockerComposeMount) string {
	switch strings.ToLower(strings.TrimSpace(mount.Type)) {
	case "volume":
		if owner := dockerComposeVolumeOwner(mount.Source); owner != "" {
			return owner
		}
		if volume, ok := model.Volumes[mount.Source]; ok {
			return dockerComposeVolumeOwner(volume.Name)
		}
	case "bind":
		if dockerPathUsesRustFSVolume(mount.Source) {
			return dockerutil.RustFSOwner
		}
		if dockerutil.IsBoringGarageDataPath(mount.Source) {
			return dockerutil.BoringGarageOwner
		}
	}
	return ""
}
