package tools

import (
	"bytes"
	"context"
	"encoding/json"
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

// DockerComposeModel is the part of `docker compose config --format json` that
// the agent Compose policy reads. Compose has already resolved includes,
// extends, YAML anchors, .env and process-environment interpolation and
// relative paths, so every check sees what Compose will actually run.
type DockerComposeModel struct {
	Name     string                               `json:"name"`
	Services map[string]DockerComposeService      `json:"services"`
	Volumes  map[string]DockerComposeNamedVolume  `json:"volumes"`
	Networks map[string]DockerComposeNetwork      `json:"networks"`
	Secrets  map[string]DockerComposeFileResource `json:"secrets"`
	Configs  map[string]DockerComposeFileResource `json:"configs"`
}

// DockerComposeService is one resolved service.
type DockerComposeService struct {
	ContainerName string                `json:"container_name"`
	Image         string                `json:"image"`
	Labels        map[string]string     `json:"labels"`
	Environment   map[string]*string    `json:"environment"`
	Volumes       []DockerComposeMount  `json:"volumes"`
	VolumesFrom   []string              `json:"volumes_from"`
	Privileged    bool                  `json:"privileged"`
	NetworkMode   string                `json:"network_mode"`
	Pid           string                `json:"pid"`
	Ipc           string                `json:"ipc"`
	UsernsMode    string                `json:"userns_mode"`
	Uts           string                `json:"uts"`
	Cgroup        string                `json:"cgroup"`
	CapAdd        []string              `json:"cap_add"`
	SecurityOpt   []string              `json:"security_opt"`
	Devices       []DockerComposeDevice `json:"devices"`
	Build         *DockerComposeBuild   `json:"build"`
}

// DockerComposeMount is a resolved service volume in long syntax.
type DockerComposeMount struct {
	Type     string `json:"type"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read_only"`
}

// DockerComposeDevice accepts the object form of current Compose releases and
// the "host:container[:permissions]" string form of older ones.
type DockerComposeDevice struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *DockerComposeDevice) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		parts := strings.SplitN(text, ":", 3)
		d.Source = parts[0]
		if len(parts) > 1 {
			d.Target = parts[1]
		}
		return nil
	}
	var value struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	d.Source, d.Target = value.Source, value.Target
	return nil
}

// DockerComposeBuild is a resolved build section.
type DockerComposeBuild struct {
	Context            string             `json:"context"`
	Dockerfile         string             `json:"dockerfile"`
	AdditionalContexts map[string]string  `json:"additional_contexts"`
	Args               map[string]*string `json:"args"`
}

// DockerComposeNamedVolume is a resolved top-level volume.
type DockerComposeNamedVolume struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	DriverOpts map[string]string `json:"driver_opts"`
	Labels     map[string]string `json:"labels"`
}

// DockerComposeNetwork is a resolved top-level network.
type DockerComposeNetwork struct {
	Name   string            `json:"name"`
	Driver string            `json:"driver"`
	Labels map[string]string `json:"labels"`
}

// DockerComposeFileResource is a resolved top-level secret or config.
type DockerComposeFileResource struct {
	Name string `json:"name"`
	File string `json:"file"`
}

// ParseDockerComposeModel decodes the stdout of `docker compose config
// --format json`. An empty or non-JSON model is an error so callers fail closed.
func ParseDockerComposeModel(resolved string) (DockerComposeModel, error) {
	var model DockerComposeModel
	trimmed := strings.TrimSpace(resolved)
	if trimmed == "" {
		return model, fmt.Errorf("docker compose config returned no model")
	}
	if err := json.Unmarshal([]byte(trimmed), &model); err != nil {
		return model, fmt.Errorf("parse docker compose config JSON: %w", err)
	}
	return model, nil
}

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
	return dockerCLIArgs(cfg, args...)
}

// runDockerComposeConfig runs one `docker compose ... config` invocation and
// keeps stdout and stderr apart. Tests replace it.
var runDockerComposeConfig = func(ctx context.Context, args []string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
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
// BoringGarageOwner, HomepageOwner and AppOwner. The app container matches only
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
		for _, mount := range service.Volumes {
			if owner := dockerComposeMountOwner(model, mount); owner != "" {
				return owner
			}
		}
		for _, ref := range service.VolumesFrom {
			target := strings.TrimSpace(ref)
			isContainer := strings.HasPrefix(target, "container:")
			target = strings.TrimPrefix(target, "container:")
			target = strings.TrimSuffix(strings.TrimSuffix(target, ":ro"), ":rw")
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

// dockerComposeLabelOwner reports the AuraGo owner named by canonical or legacy
// aurago.managed labels on a service, volume or network.
func dockerComposeLabelOwner(labels map[string]string) string {
	for _, owner := range []string{acestep.Owner, dockerutil.LocalLLMOwner, dockerutil.BoringGarageOwner, dockerutil.HomepageOwner, dockerutil.AppOwner} {
		if dockerutil.ManagedBy(labels, owner) {
			return owner
		}
	}
	return ""
}

func dockerComposeContainerOwner(name string) string {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return ""
	case acestep.IsResourceName(name):
		return acestep.Owner
	case dockerutil.IsLocalLLMContainerName(name):
		return dockerutil.LocalLLMOwner
	case dockerutil.IsBoringGarageContainerName(name):
		return dockerutil.BoringGarageOwner
	case dockerutil.IsHomepageContainerName(name):
		return dockerutil.HomepageOwner
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
	case name == "":
		return ""
	case acestep.IsResourceName(name):
		return acestep.Owner
	case dockerutil.IsLocalLLMVolumeName(name):
		return dockerutil.LocalLLMOwner
	case dockerutil.IsBoringGarageContainerName(name):
		return dockerutil.BoringGarageOwner
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
		if dockerutil.IsBoringGarageDataPath(mount.Source) {
			return dockerutil.BoringGarageOwner
		}
	}
	return ""
}

func sortedDockerComposeKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
