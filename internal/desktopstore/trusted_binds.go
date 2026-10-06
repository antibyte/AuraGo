package desktopstore

import (
	"strings"

	"aurago/internal/dockerutil"
)

// catalogTrustedBinds returns the HostConfig.Binds strings of spec that the
// code-pinned DefaultCatalog declares for spec's app or companion (exact host
// path, container path and read-only flag), such as the read-only Docker
// socket of the socket-proxy companions. Only these skip the generic Docker
// bind policy. A bind from a persisted record is trusted only while it still
// equals the current catalog; managed workspace binds and every other bind
// are still validated by internal/tools.
func catalogTrustedBinds(spec ContainerSpec) []string {
	appID := normalizeAppID(spec.Labels["aurago.desktop_store.app_id"])
	if appID == "" {
		return nil
	}
	companionID := strings.TrimSpace(spec.Labels["aurago.desktop_store.companion"])
	var templates []HostBindTemplate
	for _, entry := range DefaultCatalog() {
		if normalizeAppID(entry.ID) != appID {
			continue
		}
		if companionID == "" {
			templates = entry.HostBinds
			break
		}
		for _, companion := range entry.Companions {
			if companion.ID == companionID {
				templates = companion.HostBinds
			}
		}
		break
	}
	var trusted []string
	for _, bind := range spec.HostBinds {
		if bind.Managed {
			continue
		}
		for _, template := range templates {
			if strings.TrimSpace(template.HostPath) == strings.TrimSpace(bind.HostPath) &&
				strings.TrimSpace(template.ContainerPath) == strings.TrimSpace(bind.ContainerPath) &&
				template.ReadOnly == bind.ReadOnly {
				trusted = append(trusted, dockerHostBindString(bind))
				break
			}
		}
	}
	return trusted
}

// dockerHostBindString is the HostConfig.Binds spelling of a Store host bind.
func dockerHostBindString(bind HostBinding) string {
	mode := "rw"
	if bind.ReadOnly {
		mode = "ro"
	}
	return dockerutil.FormatBindMount(bind.HostPath, bind.ContainerPath, mode)
}
