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
// equals the current catalog, and only while spec.Image names the catalog's
// image repository (any tag or digest); managed workspace binds and every
// other bind are still validated by internal/tools.
func catalogTrustedBinds(spec ContainerSpec) []string {
	appID := normalizeAppID(spec.Labels["aurago.desktop_store.app_id"])
	if appID == "" {
		return nil
	}
	companionID := strings.TrimSpace(spec.Labels["aurago.desktop_store.companion"])
	var templates []HostBindTemplate
	var catalogImage string
	for _, entry := range DefaultCatalog() {
		if normalizeAppID(entry.ID) != appID {
			continue
		}
		if companionID == "" {
			templates, catalogImage = entry.HostBinds, entry.Image
			break
		}
		for _, companion := range entry.Companions {
			if companion.ID == companionID {
				templates, catalogImage = companion.HostBinds, companion.Image
			}
		}
		break
	}
	// A record whose image is not the catalog's repository gets no trust, so a
	// tampered Store DB row cannot receive the read-only Docker socket. Tag and
	// digest are ignored: a rollback to an older tag still recreates.
	repository := storeImageRepository(spec.Image)
	if len(templates) == 0 || repository == "" || repository != storeImageRepository(catalogImage) {
		return nil
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

// storeImageRepository returns the repository of an image reference, without
// tag or digest, lowercased, with Docker Hub's implicit registry and library/
// namespace removed: "tecnativa/docker-socket-proxy",
// "docker.io/tecnativa/docker-socket-proxy:latest" and
// "tecnativa/docker-socket-proxy@sha256:…" all name the same repository.
func storeImageRepository(ref string) string {
	ref = strings.ToLower(strings.TrimSpace(ref))
	if i := strings.IndexByte(ref, '@'); i >= 0 {
		ref = ref[:i]
	}
	if i := strings.LastIndexByte(ref, ':'); i > strings.LastIndexByte(ref, '/') {
		ref = ref[:i]
	}
	for _, registry := range []string{"docker.io/", "index.docker.io/", "registry-1.docker.io/"} {
		if rest, ok := strings.CutPrefix(ref, registry); ok {
			ref = rest
			break
		}
	}
	return strings.TrimPrefix(ref, "library/")
}

// dockerHostBindString is the HostConfig.Binds spelling of a Store host bind.
func dockerHostBindString(bind HostBinding) string {
	mode := "rw"
	if bind.ReadOnly {
		mode = "ro"
	}
	return dockerutil.FormatBindMount(bind.HostPath, bind.ContainerPath, mode)
}
