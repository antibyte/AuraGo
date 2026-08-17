package dockerutil

import (
	"strconv"
	"strings"
)

const (
	// LocalLLMOwner is the canonical owner value for AuraGo-Qwen resources.
	LocalLLMOwner = "local-llm"

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
)

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
