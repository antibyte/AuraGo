package dockerutil

import (
	"strconv"
	"strings"
)

const (
	OllamaOwner = "ollama"
	// LocalLLMOwner is the canonical owner value for AuraGo-Qwen resources.
	LocalLLMOwner = "local-llm"

	// DecisionsOwner is the canonical owner of the managed Decisions runtime
	// and discovery containers (internal/decisions).
	DecisionsOwner = "decisions"
	OCROwner       = "ocr"

	LocalLLMContainerName   = "aurago-local-llm"
	LocalLLMKeySeedName     = "aurago-local-llm-key-seed"
	LocalLLMModelVolumeName = "aurago_models"
	LocalLLMKeyVolumeName   = "aurago_local_llm_runtime"

	// BoringGarageOwner is the canonical owner for managed Boring Computers Garage.
	BoringGarageOwner         = "boring-garage"
	BoringGarageContainerName = "aurago-boring-garage"
	// BoringGarageDataPathSuffix is appended to control_plane.install_dir on the target host.
	BoringGarageDataPathSuffix = "data/sidecars/garage"

	// HomepageOwner is the canonical owner for AuraGo's managed homepage resources.
	HomepageOwner            = "homepage"
	HomepageContainerName    = "aurago-homepage"
	HomepageWebContainerName = "aurago-homepage-web"
	HomepageImageRepository  = "aurago-homepage"

	// AppOwner is the AuraGo application container from docker-compose.yml.
	AppOwner         = "aurago-app"
	AppContainerName = "aurago"

	// SecurityProxyOwner is the canonical owner of the managed Caddy security
	// proxy container (internal/proxy).
	SecurityProxyOwner         = "security-proxy"
	SecurityProxyContainerName = "aurago-security-proxy"

	// CYDCreatorOwner is the canonical owner of the CYD Creator builder
	// container and its two named volumes (internal/server/cyd_creator_builder.go).
	CYDCreatorOwner                = "cyd-creator"
	CYDCreatorBuilderContainerName = "aurago-cyd-builder"
	// CYDCreatorWorkVolumeName holds the project trees (/work);
	// CYDCreatorCacheVolumeName holds the PlatformIO objects every project's
	// firmware is linked from (/cache).
	CYDCreatorWorkVolumeName  = "aurago-cyd-work"
	CYDCreatorCacheVolumeName = "aurago-cyd-cache"
	// CYDCreatorImageRepository is the repository of the published builder
	// image (config.DefaultCYDCreatorBuilderImage without its tag).
	CYDCreatorImageRepository = "ghcr.io/antibyte/aurago-cyd-builder"
)

// IsOllamaContainerName protects both current names, including unlabeled legacy containers.
func IsOllamaContainerName(name string) bool {
	name = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(name)), "/")
	return name == "aurago_ollama_managed" || name == "aurago_ollama_embeddings"
}

func IsOllamaVolumeName(name string) bool {
	return name == "aurago_ollama_managed_models" || name == "aurago_ollama_embeddings_models"
}

// ManagedBy recognizes both the canonical AuraGo label and the legacy labels
// used by early AuraGo-Qwen integration builds.
func ManagedBy(labels map[string]string, owner string) bool {
	owner = strings.ToLower(strings.TrimSpace(owner))
	if owner == "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(labels["aurago.managed"]), owner) {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(labels["com.aurago.managed"]), "true") &&
		strings.EqualFold(strings.TrimSpace(labels["com.aurago.owner"]), owner)
}

// ManagedLabels returns the canonical labels for an AuraGo-managed resource.
func ManagedLabels(owner, component, role, fingerprint string) map[string]string {
	labels := map[string]string{
		"aurago.managed":   strings.TrimSpace(owner),
		"aurago.component": strings.TrimSpace(component),
		"aurago.role":      strings.TrimSpace(role),
	}
	if fingerprint = strings.TrimSpace(fingerprint); fingerprint != "" {
		labels["aurago.fingerprint"] = fingerprint
	}
	return labels
}

// IsDecisionsContainerName recognizes the managed Decisions runtime and
// discovery containers. Only the prefixes with a trailing hyphen match, so
// aurago-decisions-runtime itself is not reserved.
func IsDecisionsContainerName(name string) bool {
	name = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(name)), "/")
	return strings.HasPrefix(name, "aurago-decisions-runtime-") ||
		strings.HasPrefix(name, "aurago-decisions-discover-")
}

// IsLocalLLMContainerName recognizes every reserved AuraGo-Qwen container.
func IsLocalLLMContainerName(name string) bool {
	name = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(name)), "/")
	if name == LocalLLMContainerName || name == LocalLLMKeySeedName {
		return true
	}
	return strings.Contains(name, "-"+LocalLLMContainerName+"-") ||
		strings.Contains(name, "_"+LocalLLMContainerName+"_") ||
		strings.Contains(name, "-"+LocalLLMKeySeedName+"-") ||
		strings.Contains(name, "_"+LocalLLMKeySeedName+"_")
}

// IsLocalLLMVolumeName recognizes every reserved AuraGo-Qwen volume.
func IsLocalLLMVolumeName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return name == LocalLLMModelVolumeName ||
		name == LocalLLMKeyVolumeName ||
		strings.HasSuffix(name, "_"+LocalLLMModelVolumeName) ||
		strings.HasSuffix(name, "_"+LocalLLMKeyVolumeName)
}

// IsBoringGarageContainerName recognizes the reserved managed Garage container.
func IsBoringGarageContainerName(name string) bool {
	name = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(name)), "/")
	if name == BoringGarageContainerName {
		return true
	}
	return strings.Contains(name, "-"+BoringGarageContainerName+"-") ||
		strings.Contains(name, "_"+BoringGarageContainerName+"_")
}

// IsHomepageContainerName recognizes AuraGo's reserved homepage containers.
func IsHomepageContainerName(name string) bool {
	name = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(name)), "/")
	return name == HomepageContainerName || name == HomepageWebContainerName
}

// IsSecurityProxyContainerName recognizes the reserved security proxy
// container. Older AuraGo versions created it without labels, so the name alone
// identifies it.
func IsSecurityProxyContainerName(name string) bool {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(name)), "/") == SecurityProxyContainerName
}

// IsCYDCreatorBuilderContainerName recognizes the reserved CYD Creator builder
// container. AuraGo creates it only under this exact name, so compose-style
// variants stay free.
func IsCYDCreatorBuilderContainerName(name string) bool {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(name)), "/") == CYDCreatorBuilderContainerName
}

// IsCYDCreatorVolumeName recognizes the builder's work and cache volumes.
// Exact names only: the test volumes (aurago-cyd-work-it, aurago-cyd-cache-it,
// cyd-ci-*) stay usable.
func IsCYDCreatorVolumeName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return name == CYDCreatorWorkVolumeName || name == CYDCreatorCacheVolumeName
}

// ImageRepository returns the repository of an image reference: lowercase,
// without tag and digest, and without Docker Hub's implicit "docker.io/" (or
// "index.docker.io/") and "library/" prefixes, so "nginx", "library/nginx:1"
// and "docker.io/library/nginx@sha256:…" share the repository "nginx". It
// returns "" for an empty reference.
func ImageRepository(reference string) string {
	reference = strings.ToLower(strings.TrimSpace(reference))
	if digest := strings.IndexByte(reference, '@'); digest >= 0 {
		reference = reference[:digest]
	}
	if tag := strings.LastIndexByte(reference, ':'); tag > strings.LastIndexByte(reference, '/') {
		reference = reference[:tag]
	}
	for _, prefix := range []string{"docker.io/", "index.docker.io/"} {
		if rest, ok := strings.CutPrefix(reference, prefix); ok {
			reference = rest
			break
		}
	}
	if rest, ok := strings.CutPrefix(reference, "library/"); ok && !strings.Contains(rest, "/") {
		reference = rest
	}
	return reference
}

// ImageReferenceDigest returns the "sha256:…" digest of a digest-pinned
// reference (lowercase), or "".
func ImageReferenceDigest(reference string) string {
	_, digest, ok := strings.Cut(strings.ToLower(strings.TrimSpace(reference)), "@")
	if !ok {
		return ""
	}
	return digest
}

// IsCYDCreatorImageReference reports whether reference names the published
// builder repository or, when configured is not empty, the repository of the
// configured builder image (cyd_creator.builder_image), with any tag or digest.
func IsCYDCreatorImageReference(reference, configured string) bool {
	repository := ImageRepository(reference)
	if repository == "" {
		return false
	}
	if repository == CYDCreatorImageRepository {
		return true
	}
	configuredRepository := ImageRepository(configured)
	return configuredRepository != "" && repository == configuredRepository
}

// IsAuraGoAppContainerName recognizes the compose AuraGo application container
// (`aurago`) and compose-project prefixed replicas (`project_aurago`, `project-aurago-1`).
// Sidecars such as aurago-local-llm, aurago_gotenberg, and aurago-homepage are excluded.
func IsAuraGoAppContainerName(name string) bool {
	name = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(name)), "/")
	if name == "" {
		return false
	}
	if name == AppContainerName {
		return true
	}
	trimmed := name
	i := len(trimmed) - 1
	for i >= 0 && trimmed[i] >= '0' && trimmed[i] <= '9' {
		i--
	}
	if i >= 0 && i < len(trimmed)-1 && (trimmed[i] == '-' || trimmed[i] == '_') {
		trimmed = trimmed[:i]
	}
	if trimmed == AppContainerName {
		return true
	}
	return strings.HasSuffix(trimmed, "_"+AppContainerName) || strings.HasSuffix(trimmed, "-"+AppContainerName)
}

// IsHomepageImageReference recognizes the reserved homepage image repository,
// including tagged, digested, and registry-qualified references.
func IsHomepageImageReference(reference string) bool {
	reference = strings.ToLower(strings.TrimSpace(reference))
	if reference == "" {
		return false
	}
	if digest := strings.IndexByte(reference, '@'); digest >= 0 {
		reference = reference[:digest]
	}
	lastSlash := strings.LastIndexByte(reference, '/')
	if tag := strings.LastIndexByte(reference, ':'); tag > lastSlash {
		reference = reference[:tag]
	}
	if slash := strings.LastIndexByte(reference, '/'); slash >= 0 {
		reference = reference[slash+1:]
	}
	return reference == HomepageImageRepository
}

// IsBoringGarageDataPath reports whether path is under a managed Garage data root.
func IsBoringGarageDataPath(path string) bool {
	path = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(path), "\\", "/"))
	if path == "" {
		return false
	}
	marker := "/" + strings.ToLower(BoringGarageDataPathSuffix)
	return strings.Contains(path, marker) || strings.HasSuffix(path, strings.TrimPrefix(marker, "/"))
}

// ParseNumericGroupIDs validates, deduplicates, and bounds host GPU group IDs.
// Group zero is intentionally rejected and no more than 16 supplemental groups
// are accepted from an installer- or administrator-provided environment value.
func ParseNumericGroupIDs(raw string) []string {
	const maxGroups = 16
	result := make([]string, 0, maxGroups)
	seen := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		value := strings.TrimSpace(part)
		number, err := strconv.ParseUint(value, 10, 31)
		if err != nil || number == 0 {
			continue
		}
		normalized := strconv.FormatUint(number, 10)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
		if len(result) == maxGroups {
			break
		}
	}
	return result
}
