package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"

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

// ResolveDockerComposeFile applies the workspace jail DockerCompose uses and
// returns the absolute file Compose will read.
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
// warnings such as "the attribute `version` is obsolete" on stderr; they go
// into the error text when the command fails and are dropped otherwise.
func dockerComposeConfigResult(stdout, stderr []byte, err error) (string, error) {
	if err != nil {
		detail := strings.TrimSpace(string(stderr))
		if detail == "" {
			return "", fmt.Errorf("resolve Compose config: %w", err)
		}
		return "", fmt.Errorf("resolve Compose config: %w: %s", err, detail)
	}
	return string(stdout), nil
}

// DockerComposeModelOwner reports the first AuraGo owner whose resources the
// resolved model names, or "". It checks container_name, aurago.managed labels
// (canonical and legacy), the homepage image repository, named and bind volume
// sources, local bind volume devices, volumes_from and container: namespace
// references. Owners: acestep.Owner, dockerutil.LocalLLMOwner,
// BoringGarageOwner, HomepageOwner and AppOwner. The app container matches only
// its exact name "aurago" or the aurago-app owner label: this tier also binds
// grandfathered configs, and user stacks name containers like "dev-aurago".
func DockerComposeModelOwner(model DockerComposeModel) string {
	for _, name := range sortedDockerComposeKeys(model.Services) {
		service := model.Services[name]
		if owner := dockerComposeContainerOwner(service.ContainerName); owner != "" {
			return owner
		}
		for _, owner := range []string{acestep.Owner, dockerutil.LocalLLMOwner, dockerutil.BoringGarageOwner, dockerutil.HomepageOwner, dockerutil.AppOwner} {
			if dockerutil.ManagedBy(service.Labels, owner) {
				return owner
			}
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
