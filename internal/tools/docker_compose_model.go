package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
	// EnvFile is present only in the `--no-env-resolution` (all-profiles)
	// model; the default model inlines env files into Environment.
	EnvFile  []DockerComposeEnvFileRef `json:"env_file"`
	Provider *DockerComposeProvider    `json:"provider"`
	Develop  *DockerComposeDevelop     `json:"develop"`
	// UseAPISocket mounts the Docker API socket into the container.
	UseAPISocket      bool                `json:"use_api_socket"`
	PostStart         DockerComposeHooks  `json:"post_start"`
	PreStop           DockerComposeHooks  `json:"pre_stop"`
	DeviceCgroupRules DockerComposeList   `json:"device_cgroup_rules"`
	Gpus              DockerComposeList   `json:"gpus"`
	Deploy            DockerComposeDeploy `json:"deploy"`
}

// DockerComposeEnvFileRef is one env_file entry: the {path, required} object
// of current Compose releases or a plain path string. Any other shape is kept
// as Unknown instead of failing the parse.
type DockerComposeEnvFileRef struct {
	Path    string
	Unknown bool
}

// UnmarshalJSON implements json.Unmarshaler. It never fails.
func (r *DockerComposeEnvFileRef) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		r.Path = text
		return nil
	}
	var value struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(data, &value); err != nil || strings.TrimSpace(value.Path) == "" {
		r.Unknown = true
		return nil
	}
	r.Path = value.Path
	return nil
}

// DockerComposeHook is one post_start or pre_stop hook; Compose runs it in
// the container, with full host privileges when Privileged is set.
type DockerComposeHook struct {
	Privileged bool
}

// DockerComposeHooks is a hook list. It never fails: a hook whose shape is
// unknown counts as privileged.
type DockerComposeHooks []DockerComposeHook

// UnmarshalJSON implements json.Unmarshaler.
func (h *DockerComposeHooks) UnmarshalJSON(data []byte) error {
	*h = nil
	var items []json.RawMessage
	if json.Unmarshal(data, &items) != nil {
		if trimmed := bytes.TrimSpace(data); len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null")) {
			*h = DockerComposeHooks{{Privileged: true}}
		}
		return nil
	}
	for _, item := range items {
		var hook struct {
			Privileged bool `json:"privileged"`
		}
		if json.Unmarshal(item, &hook) != nil {
			hook.Privileged = true
		}
		*h = append(*h, DockerComposeHook{Privileged: hook.Privileged})
	}
	return nil
}

// DockerComposeDeploy is the part of a deploy section the policy reads: the
// devices reserved under resources.reservations. It never fails to decode.
type DockerComposeDeploy struct {
	ReservedDevices DockerComposeList
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *DockerComposeDeploy) UnmarshalJSON(data []byte) error {
	var deploy struct {
		Resources struct {
			Reservations struct {
				Devices json.RawMessage `json:"devices"`
			} `json:"reservations"`
		} `json:"resources"`
	}
	d.ReservedDevices = nil
	if json.Unmarshal(data, &deploy) == nil && len(deploy.Resources.Reservations.Devices) > 0 {
		_ = d.ReservedDevices.UnmarshalJSON(deploy.Resources.Reservations.Devices)
	}
	return nil
}

// DockerComposeString is a resolved string attribute that never fails to
// decode; other JSON is kept as its text.
type DockerComposeString string

// UnmarshalJSON implements json.Unmarshaler.
func (s *DockerComposeString) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*s = ""
		return nil
	}
	*s = DockerComposeString(dockerComposeJSONText(data))
	return nil
}

// DockerComposeProvider is a service provider section: Compose runs Type as
// an executable on the host instead of creating a container.
type DockerComposeProvider struct {
	Type string `json:"type"`
}

// DockerComposeDevelop is a service develop section.
type DockerComposeDevelop struct {
	Watch []DockerComposeWatch `json:"watch"`
}

// DockerComposeWatch is one develop.watch rule; Path is a host path that
// `up --watch` syncs into the container or rebuilds from.
type DockerComposeWatch struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

// DockerComposeList is a resolved list attribute such as build.ssh or
// build.entitlements. It accepts a list, a map, a single value or null and
// never fails, so an unexpected Compose shape cannot break the preflight:
// strings are kept as they are, other entries as their JSON text and map
// entries as "key=value" (or "key").
type DockerComposeList []string

// UnmarshalJSON implements json.Unmarshaler.
func (l *DockerComposeList) UnmarshalJSON(data []byte) error {
	*l = nil
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	var items []json.RawMessage
	if json.Unmarshal(trimmed, &items) == nil {
		for _, item := range items {
			*l = append(*l, dockerComposeJSONText(item))
		}
		return nil
	}
	var byKey map[string]json.RawMessage
	if json.Unmarshal(trimmed, &byKey) == nil {
		for _, key := range sortedDockerComposeKeys(byKey) {
			entry := key
			if value := dockerComposeJSONText(byKey[key]); value != "" && value != "null" {
				entry += "=" + value
			}
			*l = append(*l, entry)
		}
		return nil
	}
	*l = DockerComposeList{dockerComposeJSONText(trimmed)}
	return nil
}

// DockerComposeRefs are the names a reference list names, such as the
// top-level secrets of build.secrets: entries of a list of strings or of
// {"source": …} objects, or the keys of a map. It never fails.
type DockerComposeRefs []string

// UnmarshalJSON implements json.Unmarshaler.
func (r *DockerComposeRefs) UnmarshalJSON(data []byte) error {
	*r = nil
	var byName map[string]json.RawMessage
	if json.Unmarshal(data, &byName) == nil {
		*r = append(*r, sortedDockerComposeKeys(byName)...)
		return nil
	}
	var items []json.RawMessage
	if json.Unmarshal(data, &items) != nil {
		return nil
	}
	for _, item := range items {
		var name string
		if json.Unmarshal(item, &name) == nil {
			*r = append(*r, name)
			continue
		}
		var ref struct {
			Source string `json:"source"`
		}
		if json.Unmarshal(item, &ref) == nil && ref.Source != "" {
			*r = append(*r, ref.Source)
		}
	}
	return nil
}

// dockerComposeJSONText returns a JSON string's value, or other JSON as text.
func dockerComposeJSONText(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	return string(bytes.TrimSpace(raw))
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
	// SSH forwards the host's SSH agent or key files into the build.
	SSH DockerComposeList `json:"ssh"`
	// Secrets names the top-level secrets the build reads.
	Secrets      DockerComposeRefs `json:"secrets"`
	Privileged   bool              `json:"privileged"`
	Entitlements DockerComposeList `json:"entitlements"`
	// Labels are the image labels as "key=value" entries.
	Labels DockerComposeList `json:"labels"`
	// Network is the network mode of the build's RUN steps ("host" shares
	// the host network).
	Network DockerComposeString `json:"network"`
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

// DockerComposeFileResource is a resolved top-level secret or config. Its
// source is a host File, a variable of the Compose process Environment or
// inline Content (configs).
type DockerComposeFileResource struct {
	Name        string `json:"name"`
	File        string `json:"file"`
	Environment string `json:"environment"`
	Content     string `json:"content"`
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

func sortedDockerComposeKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// SortedDockerComposeKeys returns the keys of a resolved model map, sorted, so
// the policy and the agent preflight visit services and resources in a stable
// order.
func SortedDockerComposeKeys[V any](values map[string]V) []string {
	return sortedDockerComposeKeys(values)
}

// DockerComposeRefNames reads the names a resolved service reference lists
// (depends_on, networks, secrets, configs): the keys of a map, or the entries
// of a list of strings or of {"source": …} objects, as DockerComposeRefs
// decodes them. Anything else yields no names.
func DockerComposeRefNames(raw json.RawMessage) []string {
	var refs DockerComposeRefs
	_ = refs.UnmarshalJSON(raw)
	return []string(refs)
}
