package config

import (
	"strings"

	"aurago/internal/providerutil"
)

const (
	// MirageDefaultContainerName is the managed sidecar's container name.
	MirageDefaultContainerName = "aurago_mirage"
	// MirageInternalPort is the port the sidecar listens on inside its container.
	MirageInternalPort = 8080
	MirageModeFallback = "fallback"
	MirageModeAlways   = "always"
)

// MirageMode returns "always" or "fallback"; anything else is "fallback".
func (c *Config) MirageMode() string {
	if strings.EqualFold(strings.TrimSpace(c.Vision.Mirage.Mode), MirageModeAlways) {
		return MirageModeAlways
	}
	return MirageModeFallback
}

// MirageBaseURL is the OpenAI-style base URL (ending in /v1) of the sidecar.
// On the host it is the published loopback port; inside Docker it is the
// container name on the shared network (same helper the Manifest sidecar uses).
func (c *Config) MirageBaseURL() string {
	inDocker := c.Runtime.IsDocker || probeDockerContainer()
	port := c.Vision.Mirage.ContainerPort
	if inDocker {
		port = MirageInternalPort
	}
	name := strings.TrimSpace(c.Vision.Mirage.ContainerName)
	if name == "" {
		name = MirageDefaultContainerName
	}
	base := providerutil.NormalizeBaseURL(defaultSidecarURL(inDocker, name, port))
	return strings.TrimRight(base, "/") + "/v1"
}
