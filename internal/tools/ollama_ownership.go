package tools

import (
	"aurago/internal/dockerutil"
	"encoding/json"
	"strings"
)

func DockerBindUsesOllamaVolume(bind string) bool {
	spec, ok := parseDockerBindMount(bind)
	return ok && !spec.isHostPath && dockerutil.IsOllamaVolumeName(spec.hostPath)
}

// A reserved name protects generic access, but is not authority to replace it.
// Legacy adoption requires the known image, model volume and loopback binding.
func ollamaOwnsInspect(inspect, desired []byte) bool {
	var got struct {
		Config struct {
			Image  string
			Labels map[string]string
		}
		HostConfig ollamaCreateHostConfig
	}
	if json.Unmarshal(inspect, &got) != nil {
		return false
	}
	if dockerutil.ManagedBy(got.Config.Labels, dockerutil.OllamaOwner) {
		return true
	}
	if len(got.Config.Labels) != 0 {
		return false
	}
	if !strings.HasPrefix(ollamaImageRef(got.Config.Image), "ollama/ollama:") {
		return false
	}
	var want ollamaCreateBody
	if json.Unmarshal(desired, &want) != nil || len(got.HostConfig.Binds) != 1 {
		return false
	}
	bind := ollamaBindIdentity(got.HostConfig.Binds[0])
	if bind != ollamaManagedVolumeName+":/root/.ollama" && bind != ollamaEmbeddingsVolumeName+":/root/.ollama" {
		return false
	}
	if len(want.HostConfig.Binds) > 0 && !stringSetEqual(normalizeOllamaBinds(got.HostConfig.Binds), normalizeOllamaBinds(want.HostConfig.Binds)) {
		return false
	}
	publication := false
	for _, bindings := range got.HostConfig.PortBindings {
		for _, binding := range bindings {
			if binding.HostIp != "127.0.0.1" || binding.HostPort == "" {
				return false
			}
			publication = true
		}
	}
	return publication
}
