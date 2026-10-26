package tools

import (
	"aurago/internal/dockerutil"
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const (
	// ollamaImageTag is the stable ollama/ollama release recorded on 12 Oct 2026
	// (GitHub v0.40.3, 11 Oct 2026). AMD uses the versioned rocm tag published
	// beside it. pullDockerImage splits on the first colon, so this stays a tag
	// and not an @sha256 digest.
	ollamaImageTag             = "0.40.3"
	ollamaManagedVolumeName    = "aurago_ollama_managed_models"
	ollamaEmbeddingsVolumeName = "aurago_ollama_embeddings_models"
	ollamaEmbeddingsMemory     = 8 * 1024 * 1024 * 1024
	ollamaEmbeddingsPidsLimit  = 512
)

type ollamaCreateBody struct {
	Labels       map[string]string      `json:"Labels,omitempty"`
	Image        string                 `json:"Image"`
	Env          []string               `json:"Env,omitempty"`
	ExposedPorts map[string]struct{}    `json:"ExposedPorts,omitempty"`
	HostConfig   ollamaCreateHostConfig `json:"HostConfig"`
}

type ollamaCreateHostConfig struct {
	NetworkMode    string                      `json:"NetworkMode,omitempty"`
	RestartPolicy  map[string]string           `json:"RestartPolicy,omitempty"`
	PortBindings   map[string][]ollamaPortBind `json:"PortBindings"`
	Binds          []string                    `json:"Binds,omitempty"`
	Memory         int64                       `json:"Memory,omitempty"`
	PidsLimit      int64                       `json:"PidsLimit,omitempty"`
	SecurityOpt    []string                    `json:"SecurityOpt,omitempty"`
	DeviceRequests []ollamaDeviceRequest       `json:"DeviceRequests,omitempty"`
	Devices        []ollamaDevice              `json:"Devices,omitempty"`
	GroupAdd       []string                    `json:"GroupAdd,omitempty"`
}

type ollamaPortBind struct {
	HostIp   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

type ollamaDeviceRequest struct {
	Driver       string     `json:"Driver"`
	Count        int        `json:"Count"`
	Capabilities [][]string `json:"Capabilities"`
}

type ollamaDevice struct {
	PathOnHost        string `json:"PathOnHost"`
	PathInContainer   string `json:"PathInContainer"`
	CgroupPermissions string `json:"CgroupPermissions"`
}

type ollamaContainerSpec struct {
	Role        string
	Image       string
	Port        int
	Bind        string
	Memory      int64
	PidsLimit   int64
	SecurityOpt []string
	GPU         GPUInfo
	ApplyGPU    bool
}

func ollamaManagedCreatePayload(port int, volumePath, memoryLimit string, gpu GPUInfo, useGPU bool) ([]byte, error) {
	if port <= 0 {
		port = 11434
	}
	bind := ollamaManagedVolumeName + ":/root/.ollama"
	if path := strings.TrimSpace(volumePath); path != "" {
		bind = path + ":/root/.ollama"
	}
	var memory int64
	if mem, ok := parseMemoryLimit(memoryLimit); ok {
		memory = mem
	} else if strings.TrimSpace(memoryLimit) != "" {
		return nil, fmt.Errorf("invalid Ollama memory limit")
	}
	return ollamaContainerCreatePayload(ollamaContainerSpec{
		Image:       ollamaImageForGPU(gpu),
		Port:        port,
		Bind:        bind,
		Memory:      memory,
		PidsLimit:   ollamaEmbeddingsPidsLimit,
		SecurityOpt: []string{"no-new-privileges:true"},
		GPU:         gpu,
		ApplyGPU:    useGPU,
	})
}

func ollamaEmbeddingsCreatePayload(port int, gpu GPUInfo) ([]byte, error) {
	if port <= 0 {
		port = 11435
	}
	return ollamaContainerCreatePayload(ollamaContainerSpec{
		Role:        "embeddings",
		Image:       ollamaImageForGPU(gpu),
		Port:        port,
		Bind:        ollamaEmbeddingsVolumeName + ":/root/.ollama",
		Memory:      ollamaEmbeddingsMemory,
		PidsLimit:   ollamaEmbeddingsPidsLimit,
		SecurityOpt: []string{"no-new-privileges:true"},
		GPU:         gpu,
		ApplyGPU:    true,
	})
}

func ollamaContainerCreatePayload(spec ollamaContainerSpec) ([]byte, error) {
	if spec.Role == "" {
		spec.Role = "inference"
	}
	host := ollamaCreateHostConfig{
		RestartPolicy: map[string]string{"Name": "unless-stopped"},
		PortBindings: map[string][]ollamaPortBind{
			"11434/tcp": {{HostIp: "127.0.0.1", HostPort: strconv.Itoa(spec.Port)}},
		},
		Binds:       []string{spec.Bind},
		Memory:      spec.Memory,
		PidsLimit:   spec.PidsLimit,
		SecurityOpt: spec.SecurityOpt,
	}
	body := ollamaCreateBody{
		Labels: dockerutil.ManagedLabels(dockerutil.OllamaOwner, "ollama", spec.Role, ""),
		Image:  spec.Image,
		ExposedPorts: map[string]struct{}{
			"11434/tcp": {},
		},
		HostConfig: host,
	}
	if spec.ApplyGPU {
		applyGPUToCreateHost(spec.GPU, &body.HostConfig)
		if len(spec.GPU.Env) > 0 {
			body.Env = append([]string(nil), spec.GPU.Env...)
		}
	}
	return json.Marshal(body)
}

func applyGPUToCreateHost(gpu GPUInfo, host *ollamaCreateHostConfig) {
	switch gpu.Backend {
	case "nvidia":
		host.DeviceRequests = []ollamaDeviceRequest{{
			Driver:       "nvidia",
			Count:        -1,
			Capabilities: [][]string{{"gpu"}},
		}}
	case "amd", "intel", "vulkan":
		var devices []ollamaDevice
		for _, path := range gpu.Devices {
			devices = append(devices, ollamaDevice{
				PathOnHost:        path,
				PathInContainer:   path,
				CgroupPermissions: "rwm",
			})
		}
		if len(devices) > 0 {
			host.Devices = devices
		}
		if gids := hostGroupIDs([]string{"video", "render"}); len(gids) > 0 {
			host.GroupAdd = gids
		}
	}
}

// ollamaManagedSpecDrift reports whether a Docker inspect document differs from
// the create payload in image, published port, binds, memory, pid limit,
// security options, Ollama env, device requests, or devices. A digest image
// matches when RepoTags contains the desired reference. Extra inspect env
// entries are ignored.
func ollamaManagedSpecDrift(inspect, desired []byte) bool {
	var want ollamaCreateBody
	if json.Unmarshal(desired, &want) != nil {
		return true
	}
	var got struct {
		Config struct {
			Image  string            `json:"Image"`
			Env    []string          `json:"Env"`
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
		RepoTags   []string               `json:"RepoTags"`
		HostConfig ollamaCreateHostConfig `json:"HostConfig"`
	}
	if json.Unmarshal(inspect, &got) != nil {
		return true
	}
	networkMatches := got.HostConfig.NetworkMode == want.HostConfig.NetworkMode ||
		want.HostConfig.NetworkMode == "" && (got.HostConfig.NetworkMode == "default" || got.HostConfig.NetworkMode == "bridge")
	if !dockerutil.ManagedBy(got.Config.Labels, dockerutil.OllamaOwner) || !networkMatches {
		return true
	}
	if !ollamaImageMatches(got.Config.Image, got.RepoTags, want.Image) {
		return true
	}
	if !ollamaPortBindingsMatch(got.HostConfig.PortBindings, want.HostConfig.PortBindings) {
		return true
	}
	if !stringSetEqual(normalizeOllamaBinds(got.HostConfig.Binds), normalizeOllamaBinds(want.HostConfig.Binds)) {
		return true
	}
	if got.HostConfig.Memory != want.HostConfig.Memory || got.HostConfig.PidsLimit != want.HostConfig.PidsLimit {
		return true
	}
	if !stringSetEqual(got.HostConfig.SecurityOpt, want.HostConfig.SecurityOpt) {
		return true
	}
	if !stringSetEqual(got.HostConfig.GroupAdd, want.HostConfig.GroupAdd) {
		return true
	}
	if !envCovers(want.Env, got.Config.Env) {
		return true
	}
	if !jsonEqualEmpty(got.HostConfig.DeviceRequests, want.HostConfig.DeviceRequests) ||
		!jsonEqualEmpty(got.HostConfig.Devices, want.HostConfig.Devices) {
		return true
	}
	return false
}

func ollamaImageMatches(inspectImage string, repoTags []string, desired string) bool {
	desired = ollamaImageRef(desired)
	if ollamaImageRef(inspectImage) == desired {
		return true
	}
	for _, tag := range repoTags {
		if ollamaImageRef(tag) == desired {
			return true
		}
	}
	return false
}

func ollamaImageRef(image string) string {
	image = strings.TrimSpace(image)
	for _, prefix := range []string{"docker.io/", "index.docker.io/"} {
		if rest, ok := strings.CutPrefix(image, prefix); ok {
			return rest
		}
	}
	return image
}

func normalizeOllamaBinds(binds []string) []string {
	if len(binds) == 0 {
		return nil
	}
	out := make([]string, len(binds))
	for i, bind := range binds {
		out[i] = ollamaBindIdentity(bind)
	}
	return out
}

func ollamaBindIdentity(bind string) string {
	bind = strings.TrimSpace(bind)
	colon := strings.LastIndex(bind, ":")
	if colon <= 0 || !strings.Contains(bind[:colon], ":") || !ollamaBindMode(bind[colon+1:]) {
		return bind
	}
	return bind[:colon]
}

func ollamaBindMode(mode string) bool {
	if mode == "" {
		return false
	}
	for _, part := range strings.Split(mode, ",") {
		switch part {
		case "rw", "ro", "z", "Z", "consistent", "delegated", "cached":
		default:
			return false
		}
	}
	return true
}

func ensureOllamaImage(cfg DockerConfig, image string) error {
	if err := requireDockerMutationPermission(); err != nil {
		return err
	}
	if ollamaImageIsLocal(cfg, image) {
		return nil
	}
	return pullDockerImageStream(nativeHTTPContext(cfg.Context), cfg, image)
}

func ollamaImageIsLocal(cfg DockerConfig, image string) bool {
	filterURL := fmt.Sprintf("/images/json?filters=%%7B%%22reference%%22%%3A%%5B%%22%s%%22%%5D%%7D", url.QueryEscape(image))
	data, code, err := dockerRequest(cfg, "GET", filterURL, "")
	if err != nil || code != 200 {
		return false
	}
	var images []json.RawMessage
	return json.Unmarshal(data, &images) == nil && len(images) > 0
}

func ollamaPortBindingsMatch(got, want map[string][]ollamaPortBind) bool {
	if len(want) == 0 {
		return len(got) == 0
	}
	for key, wantBinds := range want {
		gotBinds := got[key]
		if len(gotBinds) != len(wantBinds) {
			return false
		}
		for i := range wantBinds {
			if gotBinds[i].HostIp != wantBinds[i].HostIp || gotBinds[i].HostPort != wantBinds[i].HostPort {
				return false
			}
		}
	}
	return true
}

func stringSetEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, item := range a {
		seen[item]++
	}
	for _, item := range b {
		seen[item]--
		if seen[item] < 0 {
			return false
		}
	}
	return true
}

func envCovers(desired, actual []string) bool {
	actualMap := make(map[string]string, len(actual))
	for _, entry := range actual {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			actualMap[key] = value
		}
	}
	for _, entry := range desired {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if actualMap[key] != value {
			return false
		}
	}
	return true
}

func jsonEqualEmpty(a, b any) bool {
	ab, aErr := json.Marshal(a)
	bb, bErr := json.Marshal(b)
	if aErr != nil || bErr != nil {
		return false
	}
	if emptyJSON(ab) && emptyJSON(bb) {
		return true
	}
	return bytes.Equal(ab, bb)
}

func emptyJSON(raw []byte) bool {
	switch strings.TrimSpace(string(raw)) {
	case "", "null", "[]", "{}":
		return true
	default:
		return false
	}
}

type ollamaEnsureLogger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}

type ollamaReconcile struct {
	Label         string
	ContainerName string
	Port          int
	Image         string
	GPUBackend    string
	Payload       []byte
	Docker        DockerConfig
	Logger        ollamaEnsureLogger
	Models        []string
	BaseURL       string
	Force         bool
	RequireGPU    bool
	InDocker      bool
	Phase         func(string)
	ModelState    func(string, string)
}

func ollamaInspectRunning(data []byte) bool {
	var info struct {
		State struct {
			Running bool `json:"Running"`
		} `json:"State"`
	}
	if json.Unmarshal(data, &info) != nil {
		return false
	}
	return info.State.Running
}
